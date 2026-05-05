# typos

[![License](https://img.shields.io/badge/license-MPL--2.0-blue)](LICENSE.md)
[![Release](https://img.shields.io/github/v/release/shlewislee/typos)](https://github.com/shlewislee/typos/releases)

Typst-to-ESC/POS bridge for thermal printers. CLI and REST server. Linux only.

## CLI

Pre-built binaries are available on the [releases page](https://github.com/shlewislee/typos/releases).

```bash
./typos print receipt.typ --device /dev/ttyUSB0 --width 72 --input "name=Lewis" --input "order=1234"
./typos image photo.jpg --rotate --gamma 1.95 --dither-method 1
./typos status --device /dev/ttyACM0
```

## Server

```bash
./typos-server serve --device /dev/ttyACM0 --width 72 --templates ./templates.toml --addr 127.0.0.1:8888
```

or you can also use docker compose:

```yaml
services:
  typos:
    image: ghcr.io/shlewislee/typos:latest
    restart: unless-stopped
    devices:
      - /dev/ttyACM0:/dev/ttyACM0
    ports:
      - "8888:8888"
    environment:
      TYPOS_WIDTH: 72
      TYPOS_TEMPLATES: /app/templates/templates.toml
      TYPOS_FONT_PATH: /app/fonts
    volumes:
      - ./templates:/app/templates:ro
      - ./fonts:/app/fonts:ro
```

```bash
docker compose up -d
```

### API

See [`docs/api.md`](docs/api.md). Job responses return `202 {"id": "..."}`, poll `GET /print/jobs/:id`.

```bash
curl http://localhost:8888/health
curl -X POST http://localhost:8888/print/template -F "name=receipt" -F 'inputs={"title":"Hello"}'
curl -X POST http://localhost:8888/print/image -F "file=@photo.png"
```

## Configuration

See [`docker/.env.example`](docker/.env.example) for a reference file.

| Flag              | Env                   | Default          |
| ----------------- | --------------------- | ---------------- |
| `--device`        | `TYPOS_DEVICE`        | `/dev/ttyACM0`   |
| `--baudrate`      | `TYPOS_BAUDRATE`      | `9600`           |
| `--width`         | `TYPOS_WIDTH`         | `72`             |
| `--dpi`           | `TYPOS_DPI`           | `203`            |
| `--gamma`         | `TYPOS_GAMMA`         | `4.5`            |
| `--dither-method` | `TYPOS_DITHER_METHOD` | `0` (Atkinson)   |
| `--templates`     | `TYPOS_TEMPLATES`     | —                |
| `--font-path`     | `TYPOS_FONT_PATH`     | —                |
| `--addr`          | `TYPOS_ADDR`          | `127.0.0.1:8888` |
| `--max-jobs`      | `TYPOS_MAX_JOBS`      | `1000`           |
| `--verbose`       | `TYPOS_VERBOSE`       | `false`          |

Dither methods: `0`=Atkinson, `1`=FloydSteinberg, `2`=StevenPigeon.

Note that if you're using docker, you'd want to change `addr` to not use `127.0.0.1`

## License

MPL-2.0. See [`LICENSE.md`](LICENSE.md).
