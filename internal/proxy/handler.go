package proxy

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"docker-proxy-hub/internal/registry"
)

type Store interface {
	ListUpstreams(ctx context.Context) ([]Upstream, error)
	RecordMetric(ctx context.Context, registryPrefix string, upstreamID int64, method, path string, statusCode int, durationMs int64, errMsg string, failover bool) error
}

type Handler struct {
	store Store
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Docker Registry v2 API version check
	if path == "/v2/" || path == "/v2" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
		return
	}

	var target registry.Target
	var err error
	if strings.HasPrefix(path, "/v2/") {
		target, err = registry.ParsePath(path)
	} else {
		// Handle bare image references like /library/alpine:latest
		target, err = registry.ParseReference(path)
	}
	if err != nil {
		slog.Warn("proxy: invalid path", "path", path, "error", err)
		http.Error(w, "invalid registry path", http.StatusBadRequest)
		return
	}

	slog.Info("proxy request",
		"method", r.Method,
		"path", path,
		"registry", target.RegistryPrefix,
		"image", target.ImagePath,
		"action", target.Action,
		"reference", target.Reference,
	)

	upstreams, err := h.store.ListUpstreams(r.Context())
	if err != nil {
		slog.Error("proxy: failed to list upstreams", "registry", target.RegistryPrefix, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	selected := SelectUpstreams(upstreams, target.RegistryPrefix)
	if len(selected) == 0 {
		slog.Warn("proxy: no upstream available", "registry", target.RegistryPrefix)
		http.Error(w, "no upstream available for this registry", http.StatusNotFound)
		return
	}

	var lastErr error
	for i, up := range selected {
		start := time.Now()
		targetURL := buildTargetURL(up.BaseURL, target)
		proxyErr := h.forwardRequest(w, r, targetURL, up.HttpProxy)
		duration := time.Since(start).Milliseconds()

		statusCode := http.StatusBadGateway
		if proxyErr == nil {
			statusCode = http.StatusOK
		} else if resp, ok := proxyErr.(proxyResponse); ok {
			statusCode = resp.code
		}

		failover := i > 0
		errMsg := ""
		if proxyErr != nil {
			errMsg = proxyErr.Error()
			lastErr = proxyErr
			slog.Warn("proxy: upstream failed",
				"upstream", up.Name,
				"registry", target.RegistryPrefix,
				"attempt", i+1,
				"status", statusCode,
				"error", errMsg,
			)
		} else {
			slog.Info("proxy response",
				"method", r.Method,
				"path", path,
				"registry", target.RegistryPrefix,
				"upstream", up.Name,
				"status", statusCode,
				"latency_ms", duration,
				"failover", failover,
			)
		}
		_ = h.store.RecordMetric(r.Context(), target.RegistryPrefix, up.ID, r.Method, path, statusCode, duration, errMsg, failover)
		if proxyErr == nil {
			return
		}
	}

	slog.Error("proxy: all upstreams failed", "registry", target.RegistryPrefix, "path", path, "error", lastErr)
	http.Error(w, "all upstreams failed", http.StatusBadGateway)
}

type proxyResponse struct {
	code int
}

func (p proxyResponse) Error() string { return "upstream error" }

func (h *Handler) forwardRequest(w http.ResponseWriter, r *http.Request, targetURL string, httpProxy string) error {
	target, err := url.Parse(targetURL)
	if err != nil {
		return err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if httpProxy != "" {
		proxyURL, err := url.Parse(httpProxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false}
		}
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = target
			req.Host = target.Host
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "Docker-Proxy-Hub/1.0")
			}
		},
		Transport: transport,
	}

	rw := &responseWriter{ResponseWriter: w}
	proxy.ServeHTTP(rw, r)
	if rw.code >= 400 {
		return proxyResponse{code: rw.code}
	}
	return nil
}

type responseWriter struct {
	http.ResponseWriter
	code int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}

func buildTargetURL(baseURL string, target registry.Target) string {
	path := "/v2/" + target.ImagePath + "/" + target.Action + "/" + target.Reference
	if target.RegistryPrefix != registry.DockerHubPrefix {
		path = "/v2/" + target.RegistryPrefix + "/" + target.ImagePath + "/" + target.Action + "/" + target.Reference
	}
	return baseURL + path
}