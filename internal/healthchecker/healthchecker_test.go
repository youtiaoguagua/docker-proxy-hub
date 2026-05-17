package healthchecker

import "testing"

func TestParseSpeedTestImage(t *testing.T) {
	tests := []struct {
		name           string
		registryPrefix string
		image          string
		wantRepo       string
		wantRef        string
	}{
		{
			name:           "docker short image defaults latest",
			registryPrefix: "docker.io",
			image:          "alpine",
			wantRepo:       "library/alpine",
			wantRef:        "latest",
		},
		{
			name:           "ghcr image with tag",
			registryPrefix: "ghcr.io",
			image:          "github/super-linter:latest",
			wantRepo:       "github/super-linter",
			wantRef:        "latest",
		},
		{
			name:           "ghcr image with explicit prefix",
			registryPrefix: "ghcr.io",
			image:          "ghcr.io/stefanprodan/podinfo:latest",
			wantRepo:       "stefanprodan/podinfo",
			wantRef:        "latest",
		},
		{
			name:           "ghcr image with digest",
			registryPrefix: "ghcr.io",
			image:          "github/super-linter@sha256:abc123",
			wantRepo:       "github/super-linter",
			wantRef:        "sha256:abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, ref, err := parseSpeedTestImage(tt.registryPrefix, tt.image)
			if err != nil {
				t.Fatalf("parseSpeedTestImage() error = %v", err)
			}
			if repo != tt.wantRepo {
				t.Fatalf("repo = %q, want %q", repo, tt.wantRepo)
			}
			if ref != tt.wantRef {
				t.Fatalf("ref = %q, want %q", ref, tt.wantRef)
			}
		})
	}
}
