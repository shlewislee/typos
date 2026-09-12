# Changelog

## [0.2.0] - 2026-09-12

### Breaking

- All print endpoints now require `multipart/form-data`; JSON requests to `POST /print/template` are rejected with `415`
- Image options (`gamma`, `dither_method`, `rotate_image`) are now handled as multipart form fields only; unset fields fall back to server defaults (`--gamma`, `--dither-method`)

### Changed

- README revised
