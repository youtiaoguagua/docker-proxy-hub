package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"docker-proxy-hub/internal/auth"
	"docker-proxy-hub/internal/db"
	"docker-proxy-hub/internal/proxy"

	"github.com/gin-gonic/gin"
)

type mockChecker struct{}

func (m *mockChecker) GetAllHealth() []proxy.Upstream                  { return nil }
func (m *mockChecker) ForceCheckAll(_ context.Context)                  {}
func (m *mockChecker) ForceCheckOne(_ context.Context, _ int64)      {}
func (m *mockChecker) SpeedTestOne(_ context.Context, _ int64) (proxy.Upstream, error) {
	return proxy.Upstream{}, fmt.Errorf("not implemented")
}

func setupTestServer(t *testing.T) (*httptest.Server, *db.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	authSvc, err := auth.NewService(context.Background(), store, "test-secret-key-long-enough", 0)
	if err != nil {
		t.Fatalf("auth service: %v", err)
	}

	handler := NewRouter(Dependencies{Auth: authSvc, Store: store, Checker: &mockChecker{}, CookieName: "token"})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server, store
}

func doRequest(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func parseJSON(t *testing.T, resp *http.Response, target any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return data
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func TestHealthEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %s", result["status"])
	}
}

func TestSetupStatusRequired(t *testing.T) {
	server, _ := setupTestServer(t)
	resp, err := http.Get(server.URL + "/api/setup/status")
	if err != nil {
		t.Fatalf("GET /api/setup/status: %v", err)
	}
	defer resp.Body.Close()
	var result map[string]bool
	json.NewDecoder(resp.Body).Decode(&result)
	if !result["setupRequired"] {
		t.Error("expected setupRequired=true")
	}
}

func TestSetupFlow(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()

	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var setupResult map[string]any
	parseJSON(t, resp, &setupResult)
	admin := setupResult["admin"].(map[string]any)
	if admin["username"] != "admin" {
		t.Errorf("expected username admin, got %v", admin["username"])
	}

	resp = doRequest(t, client, http.MethodGet, server.URL+"/api/auth/me", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /me after setup, got %d", resp.StatusCode)
	}

	var status map[string]bool
	resp = doRequest(t, client, http.MethodGet, server.URL+"/api/setup/status", nil)
	parseJSON(t, resp, &status)
	if status["setupRequired"] {
		t.Error("expected setupRequired=false after setup")
	}
}

func TestSetupFailsTwice(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})

	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "second",
		"password": "password123",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 for second setup, got %d", resp.StatusCode)
	}
}

func TestLoginFlow(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})

	loginClient := newClient()
	resp := doRequest(t, loginClient, http.MethodPost, server.URL+"/api/auth/login", map[string]string{
		"username": "admin",
		"password": "password123",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	resp = doRequest(t, loginClient, http.MethodGet, server.URL+"/api/auth/me", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /me after login, got %d", resp.StatusCode)
	}
}

func TestWrongPassword(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})
	otherClient := newClient()
	resp := doRequest(t, otherClient, http.MethodPost, server.URL+"/api/auth/login", map[string]string{
		"username": "admin",
		"password": "wrongpassword",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", resp.StatusCode)
	}
}

func TestUnauthenticatedAccess(t *testing.T) {
	server, _ := setupTestServer(t)
	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodGet, "/api/dashboard/summary"},
		{http.MethodGet, "/api/upstreams"},
	}
	for _, ep := range endpoints {
		resp, err := http.Get(server.URL + ep.path)
		if err != nil {
			t.Fatalf("GET %s: %v", ep.path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401 for %s, got %d", ep.path, resp.StatusCode)
		}
	}
}

func TestChangeCredentials(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})

	resp := doRequest(t, client, http.MethodPut, server.URL+"/api/auth/admin", map[string]string{
		"username":        "newadmin",
		"password":        "newpassword123",
		"currentPassword": "password123",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for credential change, got %d", resp.StatusCode)
	}

	resp = doRequest(t, client, http.MethodGet, server.URL+"/api/auth/me", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for old token after credential change, got %d", resp.StatusCode)
	}
}

func TestDashboardSummary(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})

	resp := doRequest(t, client, http.MethodGet, server.URL+"/api/dashboard/summary", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var summary map[string]any
	parseJSON(t, resp, &summary)
	if summary["upstreamsTotal"] != float64(0) {
		t.Errorf("expected 0 upstreams, got %v", summary["upstreamsTotal"])
	}
}

func TestUpstreamCRUD(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})

	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/upstreams", map[string]any{
		"registryPrefix": "docker.io",
		"name":           "Test Mirror",
		"baseUrl":        "https://mirror.example.com",
		"priority":       50,
		"enabled":        true,
		"healthEnabled":  true,
		"healthPath":     "/v2/",
	})
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}
	var createResult map[string]any
	parseJSON(t, resp, &createResult)
	upstream := createResult["upstream"].(map[string]any)
	upstreamID := int64(upstream["id"].(float64))
	if upstream["name"] != "Test Mirror" {
		t.Errorf("expected name 'Test Mirror', got %v", upstream["name"])
	}

	resp = doRequest(t, client, http.MethodGet, server.URL+"/api/upstreams", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for list, got %d", resp.StatusCode)
	}
	var listResult map[string]any
	parseJSON(t, resp, &listResult)
	if len(listResult["upstreams"].([]any)) != 1 {
		t.Errorf("expected 1 upstream, got %v", listResult["upstreams"])
	}

	resp = doRequest(t, client, http.MethodPut, server.URL+"/api/upstreams/"+formatID(upstreamID), map[string]any{
		"registryPrefix": "docker.io",
		"name":           "Updated Mirror",
		"baseUrl":        "https://mirror2.example.com",
		"priority":       30,
		"enabled":        true,
		"healthEnabled":  true,
		"healthPath":     "/v2/",
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for update, got %d", resp.StatusCode)
	}

	resp = doRequest(t, client, http.MethodDelete, server.URL+"/api/upstreams/"+formatID(upstreamID), nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for delete, got %d", resp.StatusCode)
	}

	resp = doRequest(t, client, http.MethodGet, server.URL+"/api/upstreams", nil)
	var emptyList map[string]any
	parseJSON(t, resp, &emptyList)
	upstreams, ok := emptyList["upstreams"].([]any)
	if !ok || len(upstreams) != 0 {
		t.Error("expected 0 upstreams after delete")
	}
}

func TestDeleteNonexistentUpstream(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})
	resp := doRequest(t, client, http.MethodDelete, server.URL+"/api/upstreams/999", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent upstream, got %d", resp.StatusCode)
	}
}

func TestUpstreamValidation(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})
	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/upstreams", map[string]any{
		"registryPrefix": "",
		"name":           "",
		"baseUrl":        "not-a-url",
		"priority":       50,
		"enabled":        true,
		"healthEnabled":  true,
		"healthPath":     "/v2/",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid upstream, got %d", resp.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})

	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/auth/logout", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for logout, got %d", resp.StatusCode)
	}

	resp = doRequest(t, client, http.MethodGet, server.URL+"/api/auth/me", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 after logout, got %d", resp.StatusCode)
	}
}

func TestMonitoringHealth(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})

	resp := doRequest(t, client, http.MethodGet, server.URL+"/api/monitoring/health", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for monitoring health, got %d", resp.StatusCode)
	}
	var result map[string]any
	parseJSON(t, resp, &result)
	if result["upstreams"] == nil {
		t.Error("expected upstreams in monitoring response")
	}
	if result["summary"] == nil {
		t.Error("expected summary in monitoring response")
	}
}

func TestCheckHealth(t *testing.T) {
	server, _ := setupTestServer(t)
	client := newClient()
	doRequest(t, client, http.MethodPost, server.URL+"/api/setup", map[string]string{
		"username": "admin",
		"password": "password123",
	})

	resp := doRequest(t, client, http.MethodPost, server.URL+"/api/upstreams/check-health", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for check-health, got %d", resp.StatusCode)
	}
}

func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}