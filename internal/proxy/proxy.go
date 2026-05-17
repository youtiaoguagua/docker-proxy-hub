package proxy

import (
	"errors"
	"net/url"
	"sort"
	"strings"

	"docker-proxy-hub/internal/health"
)

type Upstream struct {
	ID               int64               `json:"id"`
	RegistryPrefix   string              `json:"registryPrefix"`
	Name             string              `json:"name"`
	BaseURL          string              `json:"baseUrl"`
	Priority         int                 `json:"priority"`
	Enabled          bool                `json:"enabled"`
	HealthEnabled    bool                `json:"healthEnabled"`
	HealthPath       string              `json:"healthPath"`
	HttpProxy        string              `json:"-"`
	SpeedTestImage   string              `json:"speedTestImage"`
	HealthStatus     health.HealthStatus `json:"healthStatus"`
	LatencyMs        int64               `json:"latencyMs"`
	StatusCode       int                 `json:"statusCode"`
	DownloadSpeedKbps float64            `json:"downloadSpeedKbps"`
	ManifestTimeMs   int64               `json:"manifestTimeMs"`
}

type UpstreamInput struct {
	RegistryPrefix string `json:"registryPrefix"`
	Name           string `json:"name"`
	BaseURL        string `json:"baseUrl"`
	Priority       int    `json:"priority"`
	Enabled        bool   `json:"enabled"`
	HealthEnabled  bool   `json:"healthEnabled"`
	HealthPath     string `json:"healthPath"`
	SpeedTestImage string `json:"speedTestImage"`
}

type SpeedTestResult struct {
	ManifestTimeMs   int64
	DownloadBytes    int64
	DownloadTimeMs   int64
	DownloadSpeedKbps float64
}

func (input UpstreamInput) Normalize() UpstreamInput {
	input.RegistryPrefix = strings.TrimSpace(input.RegistryPrefix)
	input.Name = strings.TrimSpace(input.Name)
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if input.BaseURL != "" && !strings.HasPrefix(input.BaseURL, "http://") && !strings.HasPrefix(input.BaseURL, "https://") {
		input.BaseURL = "https://" + input.BaseURL
	}
	input.HealthPath = strings.TrimSpace(input.HealthPath)
	input.SpeedTestImage = strings.TrimSpace(input.SpeedTestImage)
	if input.SpeedTestImage == "" {
		input.SpeedTestImage = defaultSpeedTestImage(input.RegistryPrefix)
	}
	if input.HealthPath == "" {
		input.HealthPath = "/v2/"
	}
	if !strings.HasPrefix(input.HealthPath, "/") {
		input.HealthPath = "/" + input.HealthPath
	}
	return input
}

func (input UpstreamInput) Validate() error {
	input = input.Normalize()
	if input.RegistryPrefix == "" {
		return errors.New("registry prefix is required")
	}
	if input.Name == "" {
		return errors.New("name is required")
	}
	parsed, err := url.Parse(input.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("base URL must be a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("base URL must use http or https")
	}
	return nil
}

func defaultSpeedTestImage(prefix string) string {
	switch prefix {
	case "docker.io":
		return "library/alpine"
	case "ghcr.io":
		return "stefanprodan/podinfo"
	case "gcr.io":
		return "google-containers/pause"
	case "quay.io":
		return "prometheus/prometheus"
	case "registry.k8s.io":
		return "pause"
	default:
		return ""
	}
}

func SelectUpstreams(upstreams []Upstream, registryPrefix string) []Upstream {
	eligible := make([]Upstream, 0, len(upstreams))
	for _, upstream := range upstreams {
		if upstream.RegistryPrefix == registryPrefix && upstream.Enabled {
			eligible = append(eligible, upstream)
		}
	}
	if len(eligible) == 0 {
		return nil
	}

	preferred := make([]Upstream, 0, len(eligible))
	for _, upstream := range eligible {
		if upstream.HealthStatus != health.Unhealthy {
			preferred = append(preferred, upstream)
		}
	}
	if len(preferred) > 0 {
		eligible = preferred
	}

	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].Priority == eligible[j].Priority {
			return eligible[i].ID < eligible[j].ID
		}
		return eligible[i].Priority < eligible[j].Priority
	})
	return eligible
}
