package registry

import (
	"errors"
	"strings"
)

const DockerHubPrefix = "docker.io"

var knownRegistryPrefixes = map[string]struct{}{
	"ghcr.io":         {},
	"gcr.io":          {},
	"quay.io":         {},
	"registry.k8s.io": {},
}

type Target struct {
	RegistryPrefix string
	ImagePath      string
	Action         string
	Reference      string
}

func ParsePath(path string) (Target, error) {
	path = strings.Trim(path, "/")
	if !strings.HasPrefix(path, "v2/") {
		return Target{}, errors.New("registry path must start with /v2/")
	}

	parts := strings.Split(strings.TrimPrefix(path, "v2/"), "/")
	if len(parts) < 3 {
		return Target{}, errors.New("registry path is incomplete")
	}

	actionIndex := -1
	for i, part := range parts {
		if part == "manifests" || part == "blobs" || part == "tags" {
			actionIndex = i
			break
		}
	}
	if actionIndex <= 0 || actionIndex+1 >= len(parts) {
		return Target{}, errors.New("registry path missing action or reference")
	}

	imageParts := parts[:actionIndex]
	registryPrefix := DockerHubPrefix
	if _, ok := knownRegistryPrefixes[imageParts[0]]; ok {
		registryPrefix = imageParts[0]
		imageParts = imageParts[1:]
	}
	if len(imageParts) == 0 {
		return Target{}, errors.New("registry path missing image")
	}
	if registryPrefix == DockerHubPrefix && len(imageParts) == 1 {
		imageParts = []string{"library", imageParts[0]}
	}

	return Target{
		RegistryPrefix: registryPrefix,
		ImagePath:      strings.Join(imageParts, "/"),
		Action:         parts[actionIndex],
		Reference:      strings.Join(parts[actionIndex+1:], "/"),
	}, nil
}

// ParseReference parses a bare Docker image reference path (without /v2/ prefix)
// such as "/library/alpine:latest" or "/nginx" into a Target with action "manifests".
func ParseReference(path string) (Target, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return Target{}, errors.New("empty image reference")
	}

	// Split off tag or digest
	imagePart := path
	reference := "latest"

	if atIdx := strings.Index(imagePart, "@"); atIdx >= 0 {
		reference = imagePart[atIdx+1:]
		imagePart = imagePart[:atIdx]
	} else if colonIdx := strings.LastIndex(imagePart, ":"); colonIdx >= 0 {
		// A colon after the last slash is a tag (e.g., "library/alpine:latest").
		// A colon before a slash is part of a registry domain (e.g., "ghcr.io/owner/img").
		lastSlash := strings.LastIndex(imagePart, "/")
		if colonIdx > lastSlash {
			reference = imagePart[colonIdx+1:]
			imagePart = imagePart[:colonIdx]
		}
	}

	if imagePart == "" {
		return Target{}, errors.New("empty image name")
	}

	parts := strings.Split(imagePart, "/")
	registryPrefix := DockerHubPrefix
	if _, ok := knownRegistryPrefixes[parts[0]]; ok {
		registryPrefix = parts[0]
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return Target{}, errors.New("missing image name")
	}
	if registryPrefix == DockerHubPrefix && len(parts) == 1 {
		parts = []string{"library", parts[0]}
	}

	return Target{
		RegistryPrefix: registryPrefix,
		ImagePath:      strings.Join(parts, "/"),
		Action:         "manifests",
		Reference:      reference,
	}, nil
}
