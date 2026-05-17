package proxy

import (
	"testing"

	"docker-proxy-hub/internal/health"
)

func TestSelectUpstreams(t *testing.T) {
	upstreams := []Upstream{
		{ID: 1, RegistryPrefix: "docker.io", Name: "disabled", Priority: 1, Enabled: false, HealthStatus: health.Healthy},
		{ID: 2, RegistryPrefix: "docker.io", Name: "unhealthy", Priority: 1, Enabled: true, HealthStatus: health.Unhealthy},
		{ID: 3, RegistryPrefix: "docker.io", Name: "healthy low priority", Priority: 20, Enabled: true, HealthStatus: health.Healthy},
		{ID: 4, RegistryPrefix: "docker.io", Name: "healthy high priority", Priority: 10, Enabled: true, HealthStatus: health.Healthy},
		{ID: 5, RegistryPrefix: "ghcr.io", Name: "wrong registry", Priority: 1, Enabled: true, HealthStatus: health.Healthy},
	}

	selected := SelectUpstreams(upstreams, "docker.io")

	if len(selected) != 2 {
		t.Fatalf("len(selected) = %d, want 2", len(selected))
	}
	if selected[0].ID != 4 || selected[1].ID != 3 {
		t.Fatalf("selected IDs = [%d %d], want [4 3]", selected[0].ID, selected[1].ID)
	}
}

func TestSelectUpstreamsFallsBackWhenOnlyUnhealthyExists(t *testing.T) {
	upstreams := []Upstream{
		{ID: 1, RegistryPrefix: "docker.io", Name: "second", Priority: 20, Enabled: true, HealthStatus: health.Unhealthy},
		{ID: 2, RegistryPrefix: "docker.io", Name: "first", Priority: 10, Enabled: true, HealthStatus: health.Unhealthy},
	}

	selected := SelectUpstreams(upstreams, "docker.io")

	if len(selected) != 2 {
		t.Fatalf("len(selected) = %d, want 2", len(selected))
	}
	if selected[0].ID != 2 || selected[1].ID != 1 {
		t.Fatalf("selected IDs = [%d %d], want [2 1]", selected[0].ID, selected[1].ID)
	}
}

func TestValidateUpstreamInput(t *testing.T) {
	valid := UpstreamInput{
		RegistryPrefix: "docker.io",
		Name:           "Docker Hub",
		BaseURL:        "https://registry-1.docker.io",
		Priority:       10,
		Enabled:        true,
		HealthEnabled:  true,
		HealthPath:     "/v2/",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := valid
	invalid.BaseURL = "not a url"
	if err := invalid.Validate(); err == nil {
		t.Fatalf("Validate() expected invalid URL error")
	}
}
