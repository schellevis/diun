package model

import (
	"github.com/crazy-max/diun/v4/pkg/registry"
)

// Job holds job configuration
type Job struct {
	Provider        string
	Image           Image
	RegImage        registry.Image
	Registry        *registry.Client
	FirstCheck      bool
	HubLinkOverride string
	LocalDigest     string // registry digest of the image the container is currently running (Docker provider only)
}
