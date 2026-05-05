ARG ALPINE_VERSION=3.23
ARG TYPST_VERSION=v0.14.2

FROM golang:1.25-bookworm AS builder
ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w -X 'main.Version=${VERSION}'" -o /app/typos-server ./cmd/typos-server
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X 'main.Version=${VERSION}'" -o /app/typos ./cmd/typos

FROM alpine:${ALPINE_VERSION} AS typst-dl
ARG TYPST_VERSION
RUN apk add --no-cache curl tar xz && \
    curl -fsSL \
    "https://github.com/typst/typst/releases/download/${TYPST_VERSION}/typst-x86_64-unknown-linux-musl.tar.xz" \
    | tar -xJ --strip-components=1 -C /tmp

FROM alpine:${ALPINE_VERSION}

RUN apk add --no-cache ca-certificates curl

COPY --from=typst-dl /tmp/typst /usr/local/bin/typst

WORKDIR /app
RUN mkdir -p /app/templates

RUN adduser -D -s /bin/sh typos && addgroup typos dialout

COPY --from=builder /app/typos-server /usr/local/bin/typos-server
COPY --from=builder /app/typos /usr/local/bin/typos
COPY docker/templates.toml /app/templates/templates.toml

RUN chown -R typos:typos /app

ENV TYPOS_ADDR="0.0.0.0:8888"
ENV TYPOS_TEMPLATES="/app/templates/templates.toml"
ENV TYPOS_FONT_PATH="/app/fonts"

USER typos
EXPOSE 8888

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8888/health || exit 1

ENTRYPOINT ["typos-server", "serve"]
