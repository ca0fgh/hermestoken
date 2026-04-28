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
    validate_dist_integrity() { \
      dist_dir="$1"; \
      index_path="$dist_dir/index.html"; \
      if [ ! -f "$index_path" ]; then \
        echo "missing index.html in frontend dist: $index_path" >&2; \
        exit 1; \
      fi; \
      missing_assets_file="$(mktemp)"; \
      grep -Eo '/assets/[^"'"'"'?# ]+' "$index_path" | sort -u > "$missing_assets_file.refs"; \
      while IFS= read -r asset; do \
        [ -n "$asset" ] || continue; \
        if [ ! -f "$dist_dir/${asset#/}" ]; then \
          echo "missing asset referenced by index.html: $asset" >> "$missing_assets_file"; \
        fi; \
      done < "$missing_assets_file.refs"; \
      if [ -s "$missing_assets_file" ]; then \
        cat "$missing_assets_file" >&2; \
        rm -f "$missing_assets_file" "$missing_assets_file.refs"; \
        exit 1; \
      fi; \
      rm -f "$missing_assets_file" "$missing_assets_file.refs"; \
    }; \
    build_web_dist() { \
      app_dir="$1"; \
      out_dir="$2"; \
      version="$(resolve_version)"; \
      cd "/build/$app_dir"; \
      bun install --registry "${NPM_REGISTRY}"; \
      DISABLE_ESLINT_PLUGIN='true' NODE_OPTIONS="${WEB_BUILD_NODE_OPTIONS}" VITE_REACT_APP_VERSION="$version" bun run build; \
      mkdir -p "$out_dir"; \
      cp -R dist/. "$out_dir/"; \
      validate_dist_integrity "$out_dir"; \
    }; \
    copy_prebuilt_dist() { \
      app_dir="$1"; \
      out_dir="$2"; \
      if [ ! -d "$app_dir/dist" ] || [ -z "$(ls -A "$app_dir/dist" 2>/dev/null)" ]; then \
        echo "$app_dir/dist is empty; use WEB_DIST_STRATEGY=build or provide a prebuilt dist" >&2; \
        exit 1; \
      fi; \
      mkdir -p "$out_dir"; \
      cp -R "$app_dir/dist/." "$out_dir/"; \
      validate_dist_integrity "$out_dir"; \
    }; \
    case "$WEB_DIST_STRATEGY" in \
      build) \
        build_web_dist web/default /build/default-dist; \
        build_web_dist web/classic /build/classic-dist; \
        ;; \
      prebuilt) \
        copy_prebuilt_dist web/default /build/default-dist; \
        copy_prebuilt_dist web/classic /build/classic-dist; \
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
COPY --from=builder /build/default-dist ./web/default/dist
COPY --from=builder /build/classic-dist ./web/classic/dist
RUN set -eux; \
    version="$(tr -d '\r\n' < VERSION 2>/dev/null || true)"; \
    if [ -z "$version" ]; then \
      version="${APP_VERSION:-dev}"; \
    fi; \
    go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=${version}'" -o new-api

FROM mirror.gcr.io/library/debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata libasan8 wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder2 /build/new-api /
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]
