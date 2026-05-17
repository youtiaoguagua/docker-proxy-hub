package healthchecker

import (
	"context"
	"crypto/tls"
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

type Checker struct {
	store       *db.Store
	client      *http.Client
	speedClient *http.Client
	interval     time.Duration
	mu          sync.RWMutex
	results     map[int64]proxy.Upstream
	done        chan struct{}
}

func NewChecker(store *db.Store, interval time.Duration) *Checker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	}
	return &Checker{
		store:    store,
		interval: interval,
		results:  make(map[int64]proxy.Upstream),
		done:     make(chan struct{}),
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		},
		speedClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
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

// dockerManifest matches the Docker Registry v2 manifest v2 schema.
type dockerManifest struct {
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

// dockerManifestList matches the Docker Registry v2 manifest list (multi-arch).
type dockerManifestList struct {
	Manifests []struct {
		Digest   string `json:"digest"`
		Platform struct {
			Architecture string `json:"architecture"`
			OS           string `json:"os"`
		} `json:"platform"`
	} `json:"manifests"`
}

// SpeedTestOne performs a speed test on a single upstream by downloading its
// manifest and then the first 1MB of a layer blob.
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
	return c.store.GetUpstream(ctx, up.ID)
}

// getRegistryToken attempts to obtain an anonymous auth token from the
// Docker Registry v2 Www-Authenticate challenge. Many registries require
// this token even for public pulls.
func (c *Checker) getRegistryToken(ctx context.Context, baseURL, image string) (string, error) {
	// Hit /v2/ to get the auth challenge.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v2/", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.speedClient.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		// No auth required.
		return "", nil
	}

	// Parse Www-Authenticate: Bearer realm="...",service="...",scope="..."
	authHeader := resp.Header.Get("Www-Authenticate")
	realm, service, scope := parseAuthChallenge(authHeader)
	if realm == "" {
		return "", fmt.Errorf("no realm in Www-Authenticate header")
	}

	// Build token URL with scope for the specific repository.
	tokenURL, _ := url.Parse(realm)
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
	tokenResp, err := c.speedClient.Do(tokenReq)
	if err != nil {
		return "", err
	}
	defer tokenResp.Body.Close()

	var tokenResult struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenResult); err != nil {
		return "", err
	}
	return tokenResult.Token, nil
}

// parseAuthChallenge extracts realm, service, and scope from a
// Www-Authenticate: Bearer realm="...",service="...",scope="..." header.
func parseAuthChallenge(header string) (realm, service, scope string) {
	// Strip "Bearer " prefix.
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

func (c *Checker) speedTest(ctx context.Context, up proxy.Upstream) (proxy.SpeedTestResult, error) {
	var result proxy.SpeedTestResult

	// Try to get an auth token if the registry requires authentication.
	token, err := c.getRegistryToken(ctx, up.BaseURL, up.SpeedTestImage)
	if err != nil {
		// Don't fail here — the registry might not require auth, we'll try anyway.
		slog.Debug("speed test: could not get auth token, proceeding without", "error", err)
	}

	// Step 1: Download manifest to get a layer digest and measure manifest time.
	manifestURL := fmt.Sprintf("%s/v2/%s/manifests/latest", up.BaseURL, up.SpeedTestImage)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create manifest request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()
	resp, err := c.speedClient.Do(req)
	manifestTime := time.Since(start).Milliseconds()
	result.ManifestTimeMs = manifestTime
	if err != nil {
		return result, fmt.Errorf("manifest request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("manifest returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Try parsing as a v2 manifest first (has "layers").
	var manifest dockerManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return result, fmt.Errorf("failed to parse manifest: %w", err)
	}

	var digest string
	if len(manifest.Layers) > 0 {
		digest = manifest.Layers[0].Digest
	} else {
		// Manifest list — resolve to a platform-specific manifest.
		var list dockerManifestList
		if err := json.Unmarshal(body, &list); err != nil || len(list.Manifests) == 0 {
			return result, fmt.Errorf("manifest has no layers and is not a manifest list")
		}
		digest = list.Manifests[0].Digest
		for _, m := range list.Manifests {
			if m.Platform.Architecture == "amd64" && m.Platform.OS == "linux" {
				digest = m.Digest
				break
			}
		}
		archURL := fmt.Sprintf("%s/v2/%s/manifests/%s", up.BaseURL, up.SpeedTestImage, digest)
		archReq, err := http.NewRequestWithContext(ctx, http.MethodGet, archURL, nil)
		if err != nil {
			return result, fmt.Errorf("failed to create arch manifest request: %w", err)
		}
		archReq.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
		if token != "" {
			archReq.Header.Set("Authorization", "Bearer "+token)
		}
		archResp, err := c.speedClient.Do(archReq)
		if err != nil {
			return result, fmt.Errorf("arch manifest request failed: %w", err)
		}
		archBody, err := io.ReadAll(archResp.Body)
		archResp.Body.Close()
		if err != nil {
			return result, fmt.Errorf("failed to read arch manifest: %w", err)
		}
		if archResp.StatusCode >= 400 {
			return result, fmt.Errorf("arch manifest returned status %d", archResp.StatusCode)
		}
		var archManifest dockerManifest
		if err := json.Unmarshal(archBody, &archManifest); err != nil {
			return result, fmt.Errorf("failed to parse arch manifest: %w", err)
		}
		if len(archManifest.Layers) == 0 {
			return result, fmt.Errorf("arch manifest has no layers")
		}
		manifest = archManifest
		digest = manifest.Layers[0].Digest
	}

	// Step 2: Download first 1MB of the first layer blob.
	blobURL := fmt.Sprintf("%s/v2/%s/blobs/%s", up.BaseURL, up.SpeedTestImage, digest)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create blob request: %w", err)
	}
	req.Header.Set("Range", "bytes=0-1048575")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	start = time.Now()
	resp, err = c.speedClient.Do(req)
	if err != nil {
		return result, fmt.Errorf("blob request failed: %w", err)
	}
	defer resp.Body.Close()

	n, err := io.Copy(io.Discard, resp.Body)
	downloadTime := time.Since(start).Milliseconds()
	result.DownloadTimeMs = downloadTime
	result.DownloadBytes = n
	if downloadTime > 0 {
		result.DownloadSpeedKbps = float64(n) / float64(downloadTime) * 1000 / 1024
	}

	return result, nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}