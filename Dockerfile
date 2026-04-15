FROM mirror.gcr.io/oven/bun:1 AS builder
ARG NPM_REGISTRY=https://registry.npmmirror.com
ARG WEB_DIST_STRATEGY=prebuilt
ARG WEB_BUILD_NODE_OPTIONS=--max-old-space-size=4096
ARG APP_VERSION=
ENV NPM_CONFIG_REGISTRY=${NPM_REGISTRY}

WORKDIR /build
COPY . .
RUN set -eux; \
    resolve_version() { \
      version="$(tr -d '\r\n' < /build/VERSION 2>/dev/null || true)"; \
      if [ -z "$version" ]; then \
        version="${APP_VERSION:-dev}"; \
      fi; \
      printf '%s' "$version"; \
    }; \
    build_web_dist() { \
      version="$(resolve_version)"; \
      cd web; \
      bun install --registry "${NPM_REGISTRY}"; \
      DISABLE_ESLINT_PLUGIN='true' NODE_OPTIONS="${WEB_BUILD_NODE_OPTIONS}" VITE_REACT_APP_VERSION="$version" bun run build; \
      mkdir -p /build/dist; \
      cp -R dist/. /build/dist/; \
    }; \
    copy_prebuilt_dist() { \
      if [ ! -d web/dist ] || [ -z "$(ls -A web/dist 2>/dev/null)" ]; then \
        echo "web/dist is empty; use WEB_DIST_STRATEGY=build or provide a prebuilt dist" >&2; \
        exit 1; \
      fi; \
      mkdir -p /build/dist; \
      cp -R web/dist/. /build/dist/; \
    }; \
    case "$WEB_DIST_STRATEGY" in \
      build) \
        build_web_dist; \
        ;; \
      prebuilt) \
        copy_prebuilt_dist; \
        ;; \
      *) \
        echo "Unsupported WEB_DIST_STRATEGY: $WEB_DIST_STRATEGY" >&2; \
        exit 1; \
        ;; \
    esac

FROM mirror.gcr.io/library/golang:1.26.1-alpine AS builder2
ENV GO111MODULE=on CGO_ENABLED=0
ARG GOPROXY_URL=https://goproxy.cn,direct
ARG GOSUMDB_URL=sum.golang.google.cn
ARG APP_VERSION=
ENV GOPROXY=${GOPROXY_URL} GOSUMDB=${GOSUMDB_URL}

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

ADD go.mod go.sum ./
RUN set -eux; \
    n=0; \
    until [ "$n" -ge 5 ]; do \
      go mod download && break; \
      n=$((n+1)); \
      echo "go mod download failed, retry $n/5"; \
      sleep 5; \
    done; \
    [ "$n" -lt 5 ]

COPY . .
COPY --from=builder /build/dist ./web/dist
RUN set -eux; \
    version="$(tr -d '\r\n' < VERSION 2>/dev/null || true)"; \
    if [ -z "$version" ]; then \
      version="${APP_VERSION:-dev}"; \
    fi; \
    go build -tags embed -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=${version}'" -o new-api

FROM mirror.gcr.io/library/debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata libasan8 wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder2 /build/new-api /
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]
