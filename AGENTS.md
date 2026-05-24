# Repository Guidelines

## Project Structure & Module Organization

This repository contains a small Go service that checks a ZTE router WAN port speed and pushes the result to Uptime Kuma.

- `cmd/bandwidth-check/` contains the executable entry point and main loop.
- `internal/config/` loads and validates environment variables.
- `internal/zte/` handles router login, XML parsing, and WAN port selection.
- `internal/kuma/` pushes monitor results to Uptime Kuma.
- `compose.yml` provides a minimal runtime service definition with visible environment keys.
- `.github/workflows/docker.yml` builds and publishes the Docker image.

## Build, Test, and Development Commands

Use Docker when Go is not installed locally:

- `docker build -t bandwidth-check .` builds the production image.
- `docker run --rm -e ROUTER_URL=... -e ROUTER_PASSWORD=... -e KUMA_PUSH_URL=... bandwidth-check` runs the probe.
- `docker compose up -d` runs the published image after editing placeholder values in `compose.yml`.
- `docker run --rm -v "$PWD":/src -w /src golang:1.24-alpine go test ./...` runs tests.
- `docker run --rm -v "$PWD":/src -w /src golang:1.24-alpine gofmt -w .` formats Go files.

With local Go installed, use `go test ./...` and `go run ./cmd/bandwidth-check`.

## Coding Style & Naming Conventions

Use standard Go formatting with `gofmt`. Keep packages small and focused. Prefer descriptive names such as `WANPortStatus`, `KumaPushURL`, and `ParseEthernetPorts`. Environment variables use uppercase snake case, for example `ROUTER_PASSWORD` and `MIN_SPEED_MBPS`.

## Testing Guidelines

Unit tests live beside the package they cover, using `*_test.go`. Tests should avoid real router or Kuma dependencies; use samples and local test servers instead. Parser and status decision logic should be covered before changing router response handling.

## Commit & Pull Request Guidelines

Current history only establishes `Initial commit`, so use concise imperative commit messages such as `Add Kuma push probe` or `Document Docker usage`. Pull requests should include a summary, configuration changes, and the commands used for testing. Link issues when relevant.

## Security & Configuration Tips

Never commit router passwords, Uptime Kuma push URLs, Telegram tokens, real `.env` files, or logs containing secrets. Keep runtime secrets in environment variables and keep example values generic.
