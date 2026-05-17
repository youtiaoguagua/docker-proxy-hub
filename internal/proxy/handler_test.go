package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"docker-proxy-hub/internal/health"
)

type mockStore struct {
	upstreams []Upstream
	metrics   []RequestMetricCall
}

type RequestMetricCall struct {
	RegistryPrefix string
	UpstreamID     int64
	Method         string
	Path           string
	StatusCode     int
	DurationMs     int64
	Error          string
	Failover       bool
}

func (m *mockStore) ListUpstreams(_ context.Context) ([]Upstream, error) {
	return m.upstreams, nil
}

func (m *mockStore) RecordMetric(_ context.Context, registryPrefix string, upstreamID int64, method, path string, statusCode int, durationMs int64, errMsg string, failover bool) error {
	m.metrics = append(m.metrics, RequestMetricCall{
		RegistryPrefix: registryPrefix,
		UpstreamID:     upstreamID,
		Method:         method,
		Path:           path,
		StatusCode:     statusCode,
		DurationMs:     durationMs,
		Error:          errMsg,
		Failover:       failover,
	})
	return nil
}

func TestV2Ping(t *testing.T) {
	handler := NewHandler(&mockStore{})
	req := httptest.NewRequest(http.MethodGet, "/v2/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /v2/, got %d", rec.Code)
	}
	if rec.Header().Get("Docker-Distribution-API-Version") != "registry/2.0" {
		t.Error("missing Docker-Distribution-API-Version header")
	}
}

func TestInvalidPath(t *testing.T) {
	handler := NewHandler(&mockStore{})
	req := httptest.NewRequest(http.MethodGet, "/v2/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /v2/ ping, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/nginx", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid path, got %d", rec.Code)
	}
}

func TestNoUpstreamAvailable(t *testing.T) {
	handler := NewHandler(&mockStore{})
	req := httptest.NewRequest(http.MethodGet, "/v2/nginx/manifests/latest", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when no upstream, got %d", rec.Code)
	}
}

func TestProxyForwardWithUpstream(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backend.Close()

	store := &mockStore{
		upstreams: []Upstream{
			{ID: 1, RegistryPrefix: "docker.io", Name: "test", BaseURL: backend.URL, Priority: 10, Enabled: true, HealthStatus: health.Healthy},
		},
	}
	handler := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v2/nginx/manifests/latest", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if len(store.metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(store.metrics))
	}
	if store.metrics[0].RegistryPrefix != "docker.io" {
		t.Errorf("expected docker.io, got %s", store.metrics[0].RegistryPrefix)
	}
	if store.metrics[0].Failover {
		t.Error("expected no failover on first attempt")
	}
}

func TestProxyFailover(t *testing.T) {
	store := &mockStore{
		upstreams: []Upstream{
			{ID: 1, RegistryPrefix: "docker.io", Name: "bad", BaseURL: "http://127.0.0.1:0", Priority: 10, Enabled: true, HealthStatus: health.Healthy},
			{ID: 2, RegistryPrefix: "docker.io", Name: "good", BaseURL: "http://127.0.0.1:0", Priority: 20, Enabled: true, HealthStatus: health.Healthy},
		},
	}
	_ = store
	// Both upstreams will fail since port 0 is invalid - testing that failover is attempted
	handler := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v2/nginx/manifests/latest", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502 when all upstreams fail, got %d", rec.Code)
	}
	if len(store.metrics) != 2 {
		t.Errorf("expected 2 metrics (one per attempt), got %d", len(store.metrics))
	}
	if !store.metrics[1].Failover {
		t.Error("expected second attempt to be marked as failover")
	}
}