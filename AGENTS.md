# typos-rc — agent instructions

**Unless explicitly instructed otherwise**, only care about server-side code (`cmd/typos-server/`, `internal/server/`, and shared flag/logger helpers in `cmd/utils/`). Never touch `internal/printer/` or the CLI (`cmd/typos/`).

## Build & run

```sh
go build -o bin/typos ./cmd/typos          # CLI
go build -o bin/typos-server ./cmd/typos-server  # REST server
just build        # → cmd/typos (default TYPE="cli")
just build server # → cmd/typos-server
```

No linter, no typechecker, no `_test.go` files. Only manual shell tests in `tests/` (curl against a running server).

## Project structure

| Path                | Role                                                               |
| ------------------- | ------------------------------------------------------------------ |
| `cmd/typos/`        | CLI entrypoint — subcommands `print`, `image`, `status`            |
| `cmd/typos-server/` | Server entrypoint — single `serve` subcommand                      |
| `cmd/utils/`        | Shared flag/logger helpers                                         |
| `internal/printer/` | ESC/POS protocol, image processing (dither, gamma, ESC/POS raster) |
| `internal/typst/`   | Typst compiler wrapper (calls external `typst` CLI binary)         |
| `internal/server/`  | Echo v5 REST API, async job queue                                  |

## Key quirks

- **Go 1.25** (`go 1.25.0`, toolchain `go1.25.7`).
- **`tests/` is gitignored** — don't expect committed test data beyond what's there.
- **Env-driven config** — every CLI flag has a `TYPOS_*` env var source (device, baudrate, width, dpi, gamma, dither-method, templates, font-path, addr, max-jobs, verbose).
- **CLI logger** uses `tint` (colorized text on stderr). **Server logger** uses JSON on stderr.
- **Version** baked at build time: `-ldflags="-s -w -X 'main.Version=$tag'"`. Defaults to `"dev"`.
- **Docker** multi-stage build embeds the `typst` binary from GitHub releases. Accepts `--build-arg VERSION=...`.
- **Dither methods**: `0`=Atkinson (default), `1`=FloydSteinberg, `2`=StevenPigeon.
- **CI** triggers on tag push (`v*`):
  - GitHub Actions (`.github/workflows/release.yaml`) — builds linux/amd64 binaries, creates GitHub release, and pushes Docker image to GHCR (`ghcr.io/shlewislee/typos`).

## Async server architecture

Jobs are queued (buffer=100, history=1000) and processed by a single goroutine. Serial printer access is mutex-guarded. Job failures close the serial port (reconnect via `POST /printer/reconnect`).

**Server exits immediately on startup** if the serial device is unavailable; it will not start in a degraded state.

## Template config (TOML)

```toml
[[templates]]
name = "example"
filename = "/app/templates/example.typ"
required_fields = ["name"]
```

## File uploads

Multipart uploads on `/print/template` and `/print/file` map form field keys to `sys.inputs` in Typst. Only the first file per key is processed. Uploaded files land in a temporary job directory and are cleaned up after the job finishes.
