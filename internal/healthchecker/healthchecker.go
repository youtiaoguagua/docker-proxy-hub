package healthchecker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"docker-proxy-hub/internal/db"
	"docker-proxy-hub/internal/health"
	"docker-proxy-hub/internal/proxy"
)

const (
	speedTestTokenTimeout       = 10 * time.Second
	speedTestManifestTimeout    = 10 * time.Second
	speedTestTimeout            = time.Minute
	speedTestDownloadLimitBytes = int64(4 * 1024 * 1024)
)

type Checker struct {
	store    *db.Store
	client   *http.Client
	interval time.Duration
	mu       sync.RWMutex
	results  map[int64]proxy.Upstream
	done     chan struct{}
}

type dockerManifest struct {
	Layers []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

type dockerManifestList struct {
	Manifests []struct {
		Digest   string `json:"digest"`
		Platform struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"platform"`
	} `json:"manifests"`
}

func NewChecker(store *db.Store, interval time.Duration) *Checker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Checker{
		store:    store,
		interval: interval,
		results:  make(map[int64]proxy.Upstream),
		done:     make(chan struct{}),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Checker) Start(ctx context.Context) {
	slog.Info("health checker started", "interval", c.interval)
	go c.run(ctx)
}

func (c *Checker) Stop() {
	close(c.done)
}

func (c *Checker) run(ctx context.Context) {
	c.checkAll(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			c.checkAll(ctx)
		}
	}
}

func (c *Checker) checkAll(ctx context.Context) {
	upstreams, err := c.store.ListUpstreams(ctx)
	if err != nil {
		slog.Error("health check: failed to list upstreams", "error", err)
		return
	}

	var wg sync.WaitGroup
	for i := range upstreams {
		up := upstreams[i]
		if !up.Enabled || !up.HealthEnabled {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.checkOne(ctx, up)
		}()
	}
	wg.Wait()
}

func (c *Checker) checkOne(ctx context.Context, up proxy.Upstream) {
	checkURL := up.BaseURL + up.HealthPath
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		c.record(ctx, up, health.Unhealthy, 0, 0, err.Error())
		return
	}

	resp, err := c.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		c.record(ctx, up, health.Unhealthy, latency, 0, err.Error())
		return
	}
	defer resp.Body.Close()

	// The health check determines if the upstream server is reachable and
	// responding, not whether a specific endpoint returns 200.
	// - Any HTTP response means the server is up (healthy).
	// - Connection refused / timeout / DNS failure means unhealthy.
	// We still record the status code so admins can check it in monitoring.
	c.record(ctx, up, health.Healthy, latency, resp.StatusCode, "")
}

func (c *Checker) record(ctx context.Context, up proxy.Upstream, status health.HealthStatus, latencyMs int64, statusCode int, checkErr string) {
	up.HealthStatus = status
	if err := c.store.UpdateUpstreamHealth(ctx, up.ID, db.HealthRecord{
		Status:     string(status),
		LatencyMs:  latencyMs,
		StatusCode: statusCode,
		Error:      checkErr,
	}); err != nil {
		slog.Error("health check: failed to record", "upstream_id", up.ID, "error", err)
	}

	c.mu.Lock()
	up.HealthStatus = status
	c.results[up.ID] = up
	c.mu.Unlock()

	prevStatus := health.Unknown
	if prev, ok := c.results[up.ID]; ok {
		prevStatus = prev.HealthStatus
	}
	if status != prevStatus {
		slog.Warn("health check: status changed",
			"upstream", up.Name,
			"id", up.ID,
			"prev_status", prevStatus,
			"status", status,
			"latency_ms", latencyMs,
			"status_code", statusCode,
		)
	} else {
		slog.Debug("health check",
			"upstream", up.Name,
			"id", up.ID,
			"status", status,
			"latency_ms", latencyMs,
			"status_code", statusCode,
		)
	}
}

func (c *Checker) GetUpstreamHealth(id int64) (proxy.Upstream, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	up, ok := c.results[id]
	return up, ok
}

func (c *Checker) GetAllHealth() []proxy.Upstream {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]proxy.Upstream, 0, len(c.results))
	for _, up := range c.results {
		result = append(result, up)
	}
	return result
}

// ForceCheckAll triggers an immediate health check on all eligible upstreams.
func (c *Checker) ForceCheckAll(ctx context.Context) {
	c.checkAll(ctx)
}

// ForceCheckOne triggers an immediate health check for a single upstream.
func (c *Checker) ForceCheckOne(ctx context.Context, upstreamID int64) {
	upstreams, err := c.store.ListUpstreams(ctx)
	if err != nil {
		return
	}
	for _, up := range upstreams {
		if up.ID == upstreamID && up.Enabled && up.HealthEnabled {
			c.checkOne(ctx, up)
			return
		}
	}
}

// SpeedTestOne performs a speed test on a single upstream by fetching its
// manifest and downloading a partial blob range.
func (c *Checker) SpeedTestOne(ctx context.Context, upstreamID int64) (proxy.Upstream, error) {
	up, err := c.store.GetUpstream(ctx, upstreamID)
	if err != nil {
		return proxy.Upstream{}, fmt.Errorf("upstream not found")
	}
	if up.SpeedTestImage == "" {
		return proxy.Upstream{}, fmt.Errorf("no speed test image configured")
	}

	result, testErr := c.speedTest(ctx, up)
	if testErr != nil {
		return proxy.Upstream{}, testErr
	}
	if err := c.store.UpdateSpeedTest(ctx, up.ID, result); err != nil {
		slog.Error("speed test: failed to record result", "upstream_id", up.ID, "upstream", up.Name, "error", err)
	}
	return c.store.GetUpstream(ctx, upstreamID)
}

func (c *Checker) speedTest(ctx context.Context, up proxy.Upstream) (proxy.SpeedTestResult, error) {
	var result proxy.SpeedTestResult

	client := newSpeedTestHTTPClient(up)
	image := normalizeSpeedTestImage(up.RegistryPrefix, up.SpeedTestImage)
	repository, reference, err := splitImageReference(image)
	if err != nil {
		return result, fmt.Errorf("invalid image reference %q: %w", image, err)
	}

	tokenCtx, cancel := speedTestStageContext(ctx, speedTestTokenTimeout)
	token, err := getRegistryToken(tokenCtx, client, up.BaseURL, repository)
	cancel()
	if err != nil {
		slog.Debug("speed test: could not get auth token, proceeding without",
			"upstream", up.Name,
			"repository", repository,
			"error", err,
		)
	}

	manifestURL := fmt.Sprintf("%s/v2/%s/manifests/%s", strings.TrimRight(up.BaseURL, "/"), repository, reference)
	slog.Info("speed test: fetching manifest",
		"upstream", up.Name,
		"image", image,
		"manifest_url", manifestURL,
	)
	start := time.Now()
	manifestCtx, cancel := speedTestStageContext(ctx, speedTestManifestTimeout)
	resp, body, err := fetchManifestDebug(manifestCtx, client, manifestURL, token)
	cancel()
	result.ManifestTimeMs = time.Since(start).Milliseconds()
	if err != nil {
		c.logSpeedTestDiagnostics(ctx, up, image, err)
		return result, fmt.Errorf("manifest request failed: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		c.logSpeedTestDiagnostics(ctx, up, image, fmt.Errorf("manifest returned status %d", resp.StatusCode))
		return result, fmt.Errorf("manifest returned status %d body=%q", resp.StatusCode, formatLogBody(body))
	}

	resolveCtx, cancel := speedTestStageContext(ctx, speedTestManifestTimeout)
	layerDigest, err := resolveLayerDigest(resolveCtx, client, up.BaseURL, repository, body, token)
	cancel()
	if err != nil {
		c.logSpeedTestDiagnostics(ctx, up, image, err)
		return result, err
	}

	blobURL := fmt.Sprintf("%s/v2/%s/blobs/%s", strings.TrimRight(up.BaseURL, "/"), repository, layerDigest)
	slog.Info("speed test: downloading blob range",
		"upstream", up.Name,
		"image", image,
		"blob_url", blobURL,
		"bytes", speedTestDownloadLimitBytes,
	)
	blobCtx, cancel := speedTestStageContext(ctx, speedTestTimeout)
	downloaded, elapsedMs, err := downloadBlobRange(blobCtx, client, blobURL, token, speedTestDownloadLimitBytes)
	cancel()
	if err != nil {
		return result, fmt.Errorf("blob request failed: %w", err)
	}

	result.DownloadBytes = downloaded
	result.DownloadTimeMs = elapsedMs
	if result.DownloadTimeMs > 0 {
		result.DownloadSpeedKbps = float64(downloaded) / float64(result.DownloadTimeMs) * 1000 / 1024
	}

	slog.Info("speed test: download complete",
		"upstream", up.Name,
		"downloaded_mb", float64(downloaded)/1024/1024,
		"elapsed_ms", result.DownloadTimeMs,
		"speed_kbps", result.DownloadSpeedKbps,
	)

	return result, nil
}

func (c *Checker) logSpeedTestDiagnostics(ctx context.Context, up proxy.Upstream, image string, copyErr error) {
	repository, reference, err := splitImageReference(image)
	if err != nil {
		slog.Warn("speed test: failed to parse image for diagnostics",
			"upstream", up.Name,
			"image", image,
			"copy_error", copyErr,
			"error", err,
		)
		return
	}

	manifestURL := fmt.Sprintf("%s/v2/%s/manifests/%s", strings.TrimRight(up.BaseURL, "/"), repository, reference)
	client := newSpeedTestHTTPClient(up)

	slog.Warn("speed test: collecting manifest diagnostics",
		"upstream", up.Name,
		"image", image,
		"repository", repository,
		"reference", reference,
		"manifest_url", manifestURL,
		"copy_error", copyErr,
	)

	resp, body, err := fetchManifestDebug(ctx, client, manifestURL, "")
	if err != nil {
		slog.Warn("speed test: anonymous manifest request failed",
			"upstream", up.Name,
			"manifest_url", manifestURL,
			"error", err,
		)
		return
	}
	logManifestDebug("anonymous", resp, body)

	if resp.StatusCode != http.StatusUnauthorized {
		return
	}

	token, err := getRegistryToken(ctx, client, up.BaseURL, repository)
	if err != nil {
		slog.Warn("speed test: token request failed",
			"upstream", up.Name,
			"repository", repository,
			"error", err,
		)
		return
	}

	resp, body, err = fetchManifestDebug(ctx, client, manifestURL, token)
	if err != nil {
		slog.Warn("speed test: authorized manifest request failed",
			"upstream", up.Name,
			"manifest_url", manifestURL,
			"error", err,
		)
		return
	}
	logManifestDebug("authorized", resp, body)
}

func normalizeSpeedTestImage(registryPrefix, image string) string {
	image = strings.TrimSpace(strings.TrimPrefix(image, "/"))
	if registryPrefix != "" {
		image = strings.TrimPrefix(image, registryPrefix+"/")
	}

	name := image
	suffix := ""
	if idx := strings.Index(image, "@"); idx >= 0 {
		name = image[:idx]
		suffix = image[idx:]
	} else {
		slash := strings.LastIndex(image, "/")
		colon := strings.LastIndex(image, ":")
		if colon > slash {
			name = image[:colon]
			suffix = image[colon:]
		} else {
			suffix = ":latest"
		}
	}

	if registryPrefix == "docker.io" && !strings.Contains(name, "/") {
		name = "library/" + name
	}

	return name + suffix
}

func splitImageReference(image string) (string, string, error) {
	if image == "" {
		return "", "", fmt.Errorf("empty image")
	}
	if idx := strings.Index(image, "@"); idx >= 0 {
		if idx == 0 || idx == len(image)-1 {
			return "", "", fmt.Errorf("invalid digest reference %q", image)
		}
		return image[:idx], image[idx+1:], nil
	}

	slash := strings.LastIndex(image, "/")
	colon := strings.LastIndex(image, ":")
	if colon > slash {
		return image[:colon], image[colon+1:], nil
	}
	return image, "latest", nil
}

func resolveLayerDigest(ctx context.Context, client *http.Client, baseURL, repository string, body []byte, token string) (string, error) {
	var manifest dockerManifest
	if err := json.Unmarshal(body, &manifest); err == nil && len(manifest.Layers) > 0 {
		return manifest.Layers[0].Digest, nil
	}

	var list dockerManifestList
	if err := json.Unmarshal(body, &list); err != nil || len(list.Manifests) == 0 {
		return "", fmt.Errorf("manifest has no layers and is not a manifest list")
	}

	manifestDigest := list.Manifests[0].Digest
	for _, item := range list.Manifests {
		if item.Platform.OS == "linux" && item.Platform.Architecture == "amd64" {
			manifestDigest = item.Digest
			break
		}
	}

	manifestURL := fmt.Sprintf("%s/v2/%s/manifests/%s", strings.TrimRight(baseURL, "/"), repository, manifestDigest)
	resp, manifestBody, err := fetchManifestDebug(ctx, client, manifestURL, token)
	if err != nil {
		return "", fmt.Errorf("arch manifest request failed: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("arch manifest returned status %d body=%q", resp.StatusCode, formatLogBody(manifestBody))
	}
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		return "", fmt.Errorf("failed to parse arch manifest: %w", err)
	}
	if len(manifest.Layers) == 0 {
		return "", fmt.Errorf("arch manifest has no layers")
	}
	return manifest.Layers[0].Digest, nil
}

func downloadBlobRange(ctx context.Context, client *http.Client, blobURL, token string, limitBytes int64) (int64, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", limitBytes-1))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, time.Since(start).Milliseconds(), err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return 0, time.Since(start).Milliseconds(), fmt.Errorf("blob returned status %d body=%q", resp.StatusCode, formatLogBody(body))
	}

	n, err := io.Copy(io.Discard, io.LimitReader(resp.Body, limitBytes))
	return n, time.Since(start).Milliseconds(), err
}

func speedTestStageContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

func getRegistryToken(ctx context.Context, client *http.Client, baseURL, image string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v2/", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		return "", nil
	}

	realm, service, scope := parseAuthChallenge(resp.Header.Get("Www-Authenticate"))
	if realm == "" {
		return "", fmt.Errorf("no realm in Www-Authenticate header")
	}

	tokenURL, err := url.Parse(realm)
	if err != nil {
		return "", err
	}
	q := tokenURL.Query()
	if service != "" {
		q.Set("service", service)
	}
	if scope == "" {
		scope = "repository:" + image + ":pull"
	}
	q.Set("scope", scope)
	tokenURL.RawQuery = q.Encode()

	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err = client.Do(tokenReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token endpoint returned %d body=%q", resp.StatusCode, formatLogBody(body))
	}

	var tokenResult struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResult); err != nil {
		return "", err
	}
	token := tokenResult.Token
	if token == "" {
		token = tokenResult.AccessToken
	}
	if token == "" {
		return "", fmt.Errorf("empty token in auth response")
	}
	return token, nil
}

func parseAuthChallenge(header string) (realm, service, scope string) {
	header = strings.TrimPrefix(header, "Bearer ")
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		val = strings.Trim(val, `"`)
		switch strings.TrimSpace(key) {
		case "realm":
			realm = val
		case "service":
			service = val
		case "scope":
			scope = val
		}
	}
	return
}

func newSpeedTestHTTPClient(up proxy.Upstream) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if up.HttpProxy != "" {
		if proxyURL, err := url.Parse(up.HttpProxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &http.Client{
		Timeout:   speedTestTimeout,
		Transport: transport,
	}
}

func fetchManifestDebug(ctx context.Context, client *http.Client, manifestURL, token string) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.oci.image.index.v1+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return resp, nil, readErr
	}
	return resp, body, nil
}

func logManifestDebug(mode string, resp *http.Response, body []byte) {
	slog.Warn("speed test: manifest response",
		"mode", mode,
		"url", resp.Request.URL.String(),
		"proto", resp.Proto,
		"status", resp.StatusCode,
		"status_text", resp.Status,
		"headers", resp.Header,
		"transfer_encoding", resp.TransferEncoding,
		"content_length_num", resp.ContentLength,
		"body_len", len(body),
		"body", formatLogBody(body),
	)
}

func formatLogBody(body []byte) string {
	if len(body) == 0 {
		return "<empty>"
	}
	return strings.ToValidUTF8(string(body), "�")
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
