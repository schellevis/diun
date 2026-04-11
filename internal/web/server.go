package web

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/crazy-max/diun/v4/internal/db"
	"github.com/crazy-max/diun/v4/internal/model"
	pkgdocker "github.com/crazy-max/diun/v4/pkg/docker"
	"github.com/crazy-max/diun/v4/pkg/registry"
	"github.com/rs/zerolog/log"
)

// StatusEntry holds the last known status for a watched image
type StatusEntry struct {
	Status         model.ImageStatus `json:"status"`
	Provider       string            `json:"provider"`
	ContainerID    string            `json:"container_id,omitempty"`
	ContainerNames string            `json:"container_names,omitempty"`
	Image          registry.Image    `json:"image"`
	Manifest       registry.Manifest `json:"manifest"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// Server is the web portal HTTP server
type Server struct {
	addr      string
	db        *db.Client
	dockerCfg *model.PrdDocker

	mu       sync.RWMutex
	statuses map[string]StatusEntry

	httpServer *http.Server
}

// New creates a new web Server
func New(addr string, dbClient *db.Client, dockerCfg *model.PrdDocker) *Server {
	return &Server{
		addr:      addr,
		db:        dbClient,
		dockerCfg: dockerCfg,
		statuses:  make(map[string]StatusEntry),
	}
}

// UpdateStatus stores the latest run result for an image
func (s *Server) UpdateStatus(entry model.NotifEntry) {
	if s == nil {
		return
	}
	key := entry.Image.String()
	s.mu.Lock()
	s.statuses[key] = StatusEntry{
		Status:         entry.Status,
		Provider:       entry.Provider,
		ContainerID:    entry.Metadata["ctn_id"],
		ContainerNames: entry.Metadata["ctn_names"],
		Image:          entry.Image,
		Manifest:       entry.Manifest,
		UpdatedAt:      time.Now(),
	}
	s.mu.Unlock()
}

// Start starts the HTTP server in a background goroutine
func (s *Server) Start() {
	if s == nil || s.addr == "" {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/images", s.handleAPIImages)
	mux.HandleFunc("/api/containers/", s.handleAPIContainers)
	mux.HandleFunc("/", s.handleUI)

	s.httpServer = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Msgf("Web portal listening on %s", s.addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Web portal server error")
		}
	}()
}

// Stop gracefully shuts down the HTTP server
func (s *Server) Stop() {
	if s == nil || s.httpServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Warn().Err(err).Msg("Web portal shutdown error")
	}
}

// newDockerClient creates a Docker client from the provider config, or a default one
func (s *Server) newDockerClient() (*pkgdocker.Client, error) {
	opts := pkgdocker.Options{}
	if s.dockerCfg != nil {
		opts.Endpoint = s.dockerCfg.Endpoint
		opts.APIVersion = s.dockerCfg.APIVersion
		opts.TLSCertPath = s.dockerCfg.TLSCertsPath
		opts.TLSVerify = *s.dockerCfg.TLSVerify
	}
	return pkgdocker.New(opts)
}
