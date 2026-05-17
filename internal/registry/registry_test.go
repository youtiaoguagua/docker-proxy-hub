package registry

import "testing"

func TestParsePath(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		registryPrefix string
		imagePath      string
		action         string
		reference      string
	}{
		{
			name:           "docker hub short image",
			path:           "/v2/nginx/manifests/latest",
			registryPrefix: DockerHubPrefix,
			imagePath:      "library/nginx",
			action:         "manifests",
			reference:      "latest",
		},
		{
			name:           "docker hub library image",
			path:           "/v2/library/nginx/blobs/sha256:abc",
			registryPrefix: DockerHubPrefix,
			imagePath:      "library/nginx",
			action:         "blobs",
			reference:      "sha256:abc",
		},
		{
			name:           "ghcr image",
			path:           "/v2/ghcr.io/owner/image/manifests/latest",
			registryPrefix: "ghcr.io",
			imagePath:      "owner/image",
			action:         "manifests",
			reference:      "latest",
		},
		{
			name:           "gcr image",
			path:           "/v2/gcr.io/project/image/manifests/v1",
			registryPrefix: "gcr.io",
			imagePath:      "project/image",
			action:         "manifests",
			reference:      "v1",
		},
		{
			name:           "quay image",
			path:           "/v2/quay.io/org/image/tags/list",
			registryPrefix: "quay.io",
			imagePath:      "org/image",
			action:         "tags",
			reference:      "list",
		},
		{
			name:           "k8s image",
			path:           "/v2/registry.k8s.io/pause/manifests/3.9",
			registryPrefix: "registry.k8s.io",
			imagePath:      "pause",
			action:         "manifests",
			reference:      "3.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := ParsePath(tt.path)
			if err != nil {
				t.Fatalf("ParsePath() error = %v", err)
			}
			if target.RegistryPrefix != tt.registryPrefix {
				t.Fatalf("RegistryPrefix = %q, want %q", target.RegistryPrefix, tt.registryPrefix)
			}
			if target.ImagePath != tt.imagePath {
				t.Fatalf("ImagePath = %q, want %q", target.ImagePath, tt.imagePath)
			}
			if target.Action != tt.action {
				t.Fatalf("Action = %q, want %q", target.Action, tt.action)
			}
			if target.Reference != tt.reference {
				t.Fatalf("Reference = %q, want %q", target.Reference, tt.reference)
			}
		})
	}
}

func TestParsePathRejectsInvalidPaths(t *testing.T) {
	for _, path := range []string{"", "/", "/v2/", "/v1/nginx/manifests/latest", "/v2/nginx", "/v2/nginx/unknown/latest"} {
		t.Run(path, func(t *testing.T) {
			if _, err := ParsePath(path); err == nil {
				t.Fatalf("ParsePath(%q) expected error", path)
			}
		})
	}
}

func TestParseReference(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		registryPrefix string
		imagePath      string
		action         string
		reference      string
	}{
		{
			name:           "docker hub short image with tag",
			path:           "/library/alpine:latest",
			registryPrefix: DockerHubPrefix,
			imagePath:      "library/alpine",
			action:         "manifests",
			reference:      "latest",
		},
		{
			name:           "docker hub short image without tag",
			path:           "/nginx",
			registryPrefix: DockerHubPrefix,
			imagePath:      "library/nginx",
			action:         "manifests",
			reference:      "latest",
		},
		{
			name:           "docker hub library image",
			path:           "/library/alpine",
			registryPrefix: DockerHubPrefix,
			imagePath:      "library/alpine",
			action:         "manifests",
			reference:      "latest",
		},
		{
			name:           "docker hub with digest",
			path:           "/library/alpine@sha256:abc123",
			registryPrefix: DockerHubPrefix,
			imagePath:      "library/alpine",
			action:         "manifests",
			reference:      "sha256:abc123",
		},
		{
			name:           "ghcr image with tag",
			path:           "/ghcr.io/owner/image:v1",
			registryPrefix: "ghcr.io",
			imagePath:      "owner/image",
			action:         "manifests",
			reference:      "v1",
		},
		{
			name:           "ghcr image without tag",
			path:           "/ghcr.io/owner/image",
			registryPrefix: "ghcr.io",
			imagePath:      "owner/image",
			action:         "manifests",
			reference:      "latest",
		},
		{
			name:           "gcr image",
			path:           "/gcr.io/project/image:3.9",
			registryPrefix: "gcr.io",
			imagePath:      "project/image",
			action:         "manifests",
			reference:      "3.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := ParseReference(tt.path)
			if err != nil {
				t.Fatalf("ParseReference() error = %v", err)
			}
			if target.RegistryPrefix != tt.registryPrefix {
				t.Fatalf("RegistryPrefix = %q, want %q", target.RegistryPrefix, tt.registryPrefix)
			}
			if target.ImagePath != tt.imagePath {
				t.Fatalf("ImagePath = %q, want %q", target.ImagePath, tt.imagePath)
			}
			if target.Action != tt.action {
				t.Fatalf("Action = %q, want %q", target.Action, tt.action)
			}
			if target.Reference != tt.reference {
				t.Fatalf("Reference = %q, want %q", target.Reference, tt.reference)
			}
		})
	}
}

func TestParseReferenceRejectsInvalid(t *testing.T) {
	for _, path := range []string{"", "/"} {
		t.Run(path, func(t *testing.T) {
			if _, err := ParseReference(path); err == nil {
				t.Fatalf("ParseReference(%q) expected error", path)
			}
		})
	}
}
