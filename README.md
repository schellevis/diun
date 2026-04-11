<p align="center"><a href="https://crazymax.dev/diun/" target="_blank"><img height="128" src="https://raw.githubusercontent.com/crazy-max/diun/master/.res/diun.png"></a></p>

<p align="center">
  <a href="https://crazymax.dev/diun/"><img src="https://img.shields.io/badge/doc-mkdocs-02a6f2?style=flat-square&logo=read-the-docs" alt="Documentation"></a>
  <a href="https://github.com/crazy-max/diun/releases/latest"><img src="https://img.shields.io/github/release/crazy-max/diun.svg?style=flat-square" alt="GitHub release"></a>
  <a href="https://github.com/crazy-max/diun/releases/latest"><img src="https://img.shields.io/github/downloads/crazy-max/diun/total.svg?style=flat-square" alt="Total downloads"></a>
  <a href="https://github.com/crazy-max/diun/actions?workflow=build"><img src="https://img.shields.io/github/actions/workflow/status/crazy-max/diun/build.yml?branch=master&label=build&logo=github&style=flat-square" alt="Build Status"></a>
  <a href="https://hub.docker.com/r/crazymax/diun/"><img src="https://img.shields.io/docker/stars/crazymax/diun.svg?style=flat-square&logo=docker" alt="Docker Stars"></a>
  <a href="https://hub.docker.com/r/crazymax/diun/"><img src="https://img.shields.io/docker/pulls/crazymax/diun.svg?style=flat-square&logo=docker" alt="Docker Pulls"></a>
  <br /><a href="https://goreportcard.com/report/github.com/crazy-max/diun"><img src="https://goreportcard.com/badge/github.com/crazy-max/diun?style=flat-square" alt="Go Report"></a>
  <a href="https://codecov.io/gh/crazy-max/diun"><img src="https://img.shields.io/codecov/c/github/crazy-max/diun?logo=codecov&style=flat-square" alt="Codecov"></a>
  <a href="https://github.com/sponsors/crazy-max"><img src="https://img.shields.io/badge/sponsor-crazy--max-181717.svg?logo=github&style=flat-square" alt="Become a sponsor"></a>
  <a href="https://www.paypal.me/crazyws"><img src="https://img.shields.io/badge/donate-paypal-00457c.svg?logo=paypal&style=flat-square" alt="Donate Paypal"></a>
</p>

## About

**Diun** is a CLI application written in [Go](https://golang.org/)  and delivered as a
[single executable](https://github.com/crazy-max/diun/releases/latest) (and a
[Docker image](https://hub.docker.com/r/crazymax/diun/)) to receive notifications when a Docker image is updated on
a Docker registry.

![](.res/screenshot.png)

## Web Portal

Diun ships with an optional **web portal** that gives you a real-time overview of all watched
container images and their update status.

### Enabling the portal

Pass the `--web-addr` flag (or set the `WEB_ADDR` environment variable) when starting Diun:

```bash
diun serve --web-addr :8080
```

With Docker:

```yaml
services:
  diun:
    image: ghcr.io/schellevis/diun:latest
    environment:
      WEB_ADDR: ":8080"
    ports:
      - "8080:8080"
```

The portal is then available at **http://localhost:8080**.

### Features

| Feature | Description |
|---|---|
| **Image status table** | Shows every watched image with its current status: ⬆ Update, ✦ New, ✔ Unchanged, — Skipped, ✖ Error |
| **Sort by any column** | Click a column header to sort ascending/descending |
| **Filter by status** | Dropdown to show only images with updates, or only unchanged images, etc. |
| **Text search** | Live-filter by image name or tag |
| **One-click rebuild** | For Docker containers with an available update, click **Rebuild** to pull the new image and recreate the container automatically |

### Container Rebuild

When Diun is running on the same host as the Docker daemon (or with access to the Docker socket),
the **Rebuild** button in the portal will:

1. Pull the latest image from the registry.
2. Stop the running container.
3. Remove the old container.
4. Recreate it with the same configuration (volumes, environment variables, ports, networks).
5. Start the new container.

> **Note:** Rebuild works best for standalone `docker run` containers. For containers managed by
> Docker Compose or an orchestrator, use the native update mechanism of that tool instead.

You can also trigger a rebuild via the REST API:

```bash
curl -X POST http://localhost:8080/api/containers/<container_id>/rebuild
```

## Docker Image (GHCR)

Images are automatically published to the GitHub Container Registry on every push to `master`
and on each version tag:

```bash
docker pull ghcr.io/schellevis/diun:latest
docker pull ghcr.io/schellevis/diun:edge    # latest master build
docker pull ghcr.io/schellevis/diun:v4.x.y  # specific release
```

## Documentation

Documentation can be found on https://crazymax.dev/diun/

## Contributing

Want to contribute? Awesome! The most basic way to show your support is to star the project, or to raise issues. If
you want to open a pull request, please read the [contributing guidelines](.github/CONTRIBUTING.md).

You can also support this project by [**becoming a sponsor on GitHub**](https://github.com/sponsors/crazy-max) or by
making a [Paypal donation](https://www.paypal.me/crazyws) to ensure this journey continues indefinitely!

Thanks again for your support, it is much appreciated! :pray:

## License

MIT. See `LICENSE` for more details.
