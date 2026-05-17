package db

import (
	"context"
	"path/filepath"
	"testing"

	"docker-proxy-hub/internal/proxy"
)

func openTestDB(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestOpenCreatesTables(t *testing.T) {
	store := openTestDB(t)
	tables := []string{"schema_migrations", "app_settings", "admins", "upstreams", "upstream_health", "request_metrics"}
	for _, table := range tables {
		var name string
		err := store.db.QueryRowContext(context.Background(), "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestHasAdminFalseInitially(t *testing.T) {
	store := openTestDB(t)
	has, err := store.HasAdmin(context.Background())
	if err != nil {
		t.Fatalf("HasAdmin: %v", err)
	}
	if has {
		t.Error("expected no admin initially")
	}
}

func TestCreateAndGetAdmin(t *testing.T) {
	store := openTestDB(t)
	admin, err := store.CreateAdmin(context.Background(), "testuser", "hashedpassword")
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	if admin.ID != 1 {
		t.Errorf("expected ID 1, got %d", admin.ID)
	}
	if admin.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", admin.Username)
	}
	if admin.TokenVersion != 1 {
		t.Errorf("expected token_version 1, got %d", admin.TokenVersion)
	}

	has, _ := store.HasAdmin(context.Background())
	if !has {
		t.Error("expected admin to exist after creation")
	}

	got, err := store.GetAdminByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetAdminByID: %v", err)
	}
	if got.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", got.Username)
	}

	byName, err := store.GetAdminByUsername(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("GetAdminByUsername: %v", err)
	}
	if byName.ID != 1 {
		t.Errorf("expected ID 1, got %d", byName.ID)
	}
}

func TestSecondAdminFails(t *testing.T) {
	store := openTestDB(t)
	_, err := store.CreateAdmin(context.Background(), "first", "hash1")
	if err != nil {
		t.Fatalf("first CreateAdmin: %v", err)
	}
	_, err = store.CreateAdmin(context.Background(), "second", "hash2")
	if err == nil {
		t.Error("expected error creating second admin")
	}
}

func TestGetAdminNotFound(t *testing.T) {
	store := openTestDB(t)
	_, err := store.GetAdminByID(context.Background(), 999)
	if !IsNotFound(err) {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestUpdateAdmin(t *testing.T) {
	store := openTestDB(t)
	_, err := store.CreateAdmin(context.Background(), "olduser", "oldhash")
	if err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}
	updated, err := store.UpdateAdmin(context.Background(), 1, "newuser", "newhash")
	if err != nil {
		t.Fatalf("UpdateAdmin: %v", err)
	}
	if updated.Username != "newuser" {
		t.Errorf("expected newuser, got %s", updated.Username)
	}
	if updated.TokenVersion != 2 {
		t.Errorf("expected token_version 2, got %d", updated.TokenVersion)
	}
}

func TestGetOrCreateSetting(t *testing.T) {
	store := openTestDB(t)
	val, err := store.GetOrCreateSetting(context.Background(), "jwt_secret", func() (string, error) {
		return "generated-secret", nil
	})
	if err != nil {
		t.Fatalf("GetOrCreateSetting: %v", err)
	}
	if val != "generated-secret" {
		t.Errorf("expected generated-secret, got %s", val)
	}
	val2, err := store.GetOrCreateSetting(context.Background(), "jwt_secret", func() (string, error) {
		return "should-not-be-used", nil
	})
	if err != nil {
		t.Fatalf("GetOrCreateSetting second call: %v", err)
	}
	if val2 != "generated-secret" {
		t.Errorf("expected same value on second call, got %s", val2)
	}
}

func TestUpstreamCRUD(t *testing.T) {
	store := openTestDB(t)
	ctx := context.Background()

	input := proxy.UpstreamInput{
		RegistryPrefix: "docker.io",
		Name:           "Alibaba Cloud",
		BaseURL:        "https://mirror.example.com",
		Priority:       50,
		Enabled:        true,
		HealthEnabled:  true,
		HealthPath:     "/v2/",
	}

	created, err := store.CreateUpstream(ctx, input)
	if err != nil {
		t.Fatalf("CreateUpstream: %v", err)
	}
	if created.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if created.RegistryPrefix != "docker.io" {
		t.Errorf("expected docker.io, got %s", created.RegistryPrefix)
	}
	if created.Name != "Alibaba Cloud" {
		t.Errorf("expected Alibaba Cloud, got %s", created.Name)
	}
	if !created.Enabled {
		t.Error("expected Enabled=true")
	}

	got, err := store.GetUpstream(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUpstream: %v", err)
	}
	if got.Name != "Alibaba Cloud" {
		t.Errorf("expected Alibaba Cloud, got %s", got.Name)
	}

	list, err := store.ListUpstreams(ctx)
	if err != nil {
		t.Fatalf("ListUpstreams: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 upstream, got %d", len(list))
	}

	updateInput := proxy.UpstreamInput{
		RegistryPrefix: "docker.io",
		Name:           "Aliyun Mirror",
		BaseURL:        "https://mirror.aliyun.com",
		Priority:       30,
		Enabled:        true,
		HealthEnabled:  true,
		HealthPath:     "/v2/",
	}
	updated, err := store.UpdateUpstream(ctx, created.ID, updateInput)
	if err != nil {
		t.Fatalf("UpdateUpstream: %v", err)
	}
	if updated.Name != "Aliyun Mirror" {
		t.Errorf("expected Aliyun Mirror, got %s", updated.Name)
	}
	if updated.Priority != 30 {
		t.Errorf("expected priority 30, got %d", updated.Priority)
	}

	err = store.DeleteUpstream(ctx, created.ID)
	if err != nil {
		t.Fatalf("DeleteUpstream: %v", err)
	}
	list, _ = store.ListUpstreams(ctx)
	if len(list) != 0 {
		t.Error("expected 0 upstreams after delete")
	}
}

func TestDeleteNonexistentUpstream(t *testing.T) {
	store := openTestDB(t)
	err := store.DeleteUpstream(context.Background(), 999)
	if !IsNotFound(err) {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestDashboardSummaryEmpty(t *testing.T) {
	store := openTestDB(t)
	summary, err := store.DashboardSummary(context.Background())
	if err != nil {
		t.Fatalf("DashboardSummary: %v", err)
	}
	if summary.UpstreamsTotal != 0 {
		t.Errorf("expected 0 upstreams, got %d", summary.UpstreamsTotal)
	}
	if summary.RequestsToday != 0 {
		t.Errorf("expected 0 requests today, got %d", summary.RequestsToday)
	}
}