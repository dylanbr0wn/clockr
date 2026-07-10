FROM golang:1.26.2-bookworm AS build

ARG BUF_VERSION=1.71.0

WORKDIR /src

# gen/ is gitignored; install pinned Buf so the image regenerates Connect sources.
RUN set -eux; \
  arch="$(uname -m)"; \
  case "$arch" in \
    x86_64) arch=x86_64 ;; \
    aarch64|arm64) arch=aarch64 ;; \
    *) echo "unsupported arch: $arch" >&2; exit 1 ;; \
  esac; \
  curl -fsSL "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-Linux-${arch}" \
    -o /usr/local/bin/buf; \
  chmod +x /usr/local/bin/buf; \
  buf --version

COPY go.mod go.sum ./
RUN go mod download

COPY buf.yaml buf.gen.yaml ./
COPY proto ./proto
COPY cmd ./cmd
COPY internal ./internal

# frontend/ is omitted via .railwayignore; create the ES plugin out dir anyway.
RUN mkdir -p frontend/src/gen \
 && buf generate

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/oauth-broker ./cmd/oauth-broker

FROM gcr.io/distroless/static-debian12

COPY --from=build /out/oauth-broker /oauth-broker

ENTRYPOINT ["/oauth-broker"]
