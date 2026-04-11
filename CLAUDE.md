# CLAUDE.md

This file provides guidance for AI coding assistants working on the Diun codebase.

## Project Overview

**Diun** (Docker Image Update Notifier) is a Go CLI application that watches Docker images for
updates and sends notifications when a newer version becomes available. It supports multiple
container providers (Docker, Swarm, Kubernetes, Nomad, File, Dockerfile) and many notification
back-ends (Slack, Telegram, Discord, Email, Webhook, etc.).

It also exposes an optional **web portal** that shows the current update status of all watched
images and allows rebuilding containers via the Docker API.

## Repository Layout

```
cmd/                    CLI entry point (main, serve, image, notif sub-commands)
internal/
  app/                  Core application logic (diun.go, job.go, hc.go)
  config/               Configuration loading (YAML + env vars via gonfig)
  db/                   BoltDB wrapper – stores image manifests
  grpc/                 gRPC server exposing ImageService and NotifService
  model/                Shared data types (Image, Job, NotifEntry, Watch, …)
  notif/                Notification clients (one file per back-end)
  provider/             Provider clients: docker, swarm, kubernetes, nomad, file, dockerfile
  web/                  HTTP web portal (server.go, handler.go)
pb/                     Generated protobuf + gRPC stubs
pkg/
  docker/               Low-level Docker API client helpers (container, image, rebuild)
  registry/             Docker registry client (auth, manifest, tags, image parsing)
  utl/                  Utility helpers
docs/                   MkDocs documentation source
hack/                   Build / code-generation scripts
vendor/                 Vendored Go dependencies (GOFLAGS=-mod=vendor)
```

## Build & Test

```bash
# Build binary (vendor mode required)
GOFLAGS="-mod=vendor" go build ./cmd

# Or using the Makefile / Bake targets
make build        # uses docker buildx bake

# Run all tests
GOFLAGS="-mod=vendor" go test ./...

# Run with race detector (as CI does)
GOFLAGS="-mod=vendor" go test -race ./...

# Lint (golangci-lint is run via docker bake)
docker buildx bake validate
```

## Development Conventions

* **Go modules in vendor mode** – all dependencies are in `vendor/`. When adding or updating
  dependencies run `go mod tidy && go mod vendor`.
* **Error wrapping** – use `github.com/pkg/errors` (`errors.Wrap`, `errors.Wrapf`,
  `errors.Errorf`). Do **not** use `fmt.Errorf` with `%w`.
* **Logging** – use `github.com/rs/zerolog`. Access the logger via `log.With().Str("key","val").Logger()` for structured contextual logging.
* **Configuration** – model types live in `internal/model/`. Each type implements `GetDefaults()` and `SetDefaults()`. Configuration is loaded from YAML file and `DIUN_*` environment variables.
* **Nil-safe receivers** – helpers that may be called on a nil pointer (e.g. `web.Server`) must guard with `if s == nil { return }`.
* **Context** – Docker client methods use `context.Background()` stored in `Client.ctx`. New HTTP
  handlers should use `r.Context()` for request-scoped cancellation.
* **No external frontend deps** – the web portal UI is a self-contained inline HTML page embedded
  as a Go string constant. Keep it dependency-free.

## Key Data Flow

1. `diun serve` → `app.New()` → creates DB, gRPC server, notif client, optional web server.
2. On schedule (or startup) → `Diun.Run()` → each provider returns `[]model.Job`.
3. Each job is dispatched to an `ants` worker pool → `runJob()`.
4. `runJob()` fetches the remote manifest, compares to DB, sets `entry.Status`
   (new / update / unchange / skip / error).
5. Manifest is persisted to BoltDB; `web.Server.UpdateStatus(entry)` refreshes the portal.
6. If status warrants notification, `notif.Client.Send(entry)` is called.

## Web Portal

Start with `--web-addr :8080` (or `WEB_ADDR=:8080` env var). The portal is then available at
`http://localhost:8080`.

* `GET /api/images` – JSON list of all watched images with their last known status.
* `POST /api/containers/{id}/rebuild` – pulls the new image and recreates the named container.
* `GET /` – HTML single-page portal with sortable/filterable table.

## Common Pitfalls

* `registry.Image.Name()` is a **method** (not a field). Use `img.Name()`.
* `registry.Image.String()` returns `name:tag` and is used as the DB key.
* The BoltDB file is locked exclusively – only one process can open it at a time.
* Providers are only active when the corresponding config section is present; always nil-check `cfg.Providers.*`.
