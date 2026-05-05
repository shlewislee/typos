# typos-server API

Base URL: `http://<host>:<port>` (default `127.0.0.1:8888`)

| Method | Path                 | Description                                                   |
| ------ | -------------------- | ------------------------------------------------------------- |
| `GET`  | `/health`            | Returns `Ok` (200) or `printer offline` (503)                 |
| `GET`  | `/fonts`             | Lists fonts available to the Typst compiler                   |
| `GET`  | `/printer/status`    | Printer status: `"online"` or `"na"`                           |
| `POST` | `/printer/reconnect` | Reopens serial port                                           |
| `POST` | `/print/template`    | Render a named template (JSON or multipart)                   |
| `POST` | `/print/file`        | Upload and render a `.typ` file (multipart)                   |
| `POST` | `/print/image`       | Print an image directly (multipart)                           |
| `GET`  | `/print/jobs/:id`    | Poll job status: `pending` → `processing` → `done` / `failed` |

## Print endpoints

All three accept multipart/form-data. Common optional fields:

| Field           | Description                                              |
| --------------- | -------------------------------------------------------- |
| `rotate_image`  | `"true"` or `"1"` to rotate                              |
| `dither_method` | `0` (Atkinson), `1` (FloydSteinberg), `2` (StevenPigeon) |
| `gamma`         | Float (e.g. `"4.5"`)                                     |

```bash
# Print a named template with file uploads
curl -X POST http://localhost:8888/print/template \
  -F "name=receipt" \
  -F 'inputs={"title":"Hello"}' \
  -F "logo=@logo.png"

# Upload and print a .typ file
curl -X POST http://localhost:8888/print/file \
  -F "file=@hello.typ" \
  -F 'inputs={"name":"shlewislee"}' \
  -F "logo=@logo.png"

# Print an image directly
curl -X POST http://localhost:8888/print/image \
  -F "file=@photo.png"
```

## Responses

**Success:** `202 {"id": "a1b2c"}`
**Errors:** `400`, `404`, `503` — all return `{"message": "..."}`.
**Queue full:** `503 {"message": "job queue is full"}`.

### Job object

`GET /print/jobs/:id` returns the full job:

| Field    | Type     | Description                                       |
| -------- | -------- | ------------------------------------------------- |
| `id`     | string   | Job identifier                                    |
| `status` | string   | `"pending"` → `"processing"` → `"done"` / `"failed"` |
| `type`   | string   | `"template"`, `"file"`, or `"image"`              |
| `error`  | string   | Error message (only present when `status` is `"failed"`) |

## Notes

- This implementation has no authorization. Multipart endpoints also accept arbitrary file uploads with no filtering. Keep server access internal-only.
- `/print/template` accepts JSON or multipart. Use JSON when no file uploads are needed.
- Files uploaded alongside a template are saved to a temp dir and their basename is injected into `inputs[key]` automatically — no need to declare them in the `inputs` field.
