# syntax=docker/dockerfile:1

# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS build

RUN apk add --no-cache git

# Install buf for proto generation
COPY --from=bufbuild/buf:latest /usr/local/bin/buf /usr/local/bin/buf

WORKDIR /src

# Cache module downloads: copy dependency descriptors first
COPY go.work go.mod go.sum ./
COPY plugins/timer/go.mod plugins/timer/go.sum ./plugins/timer/
COPY plugins/webhook/go.mod plugins/webhook/go.sum ./plugins/webhook/
COPY plugins/mqtt/go.mod plugins/mqtt/go.sum ./plugins/mqtt/
COPY plugins/calendar/go.mod plugins/calendar/go.sum ./plugins/calendar/
COPY plugins/devicemonitor/go.mod plugins/devicemonitor/go.sum ./plugins/devicemonitor/
RUN go mod download

# Copy full source
COPY . .

# Generate protobuf code
RUN cd api && buf generate

# Build args
ARG VERSION=dev
ARG RECUR_PLUGINS="timer webhook"

# Build core binaries
RUN CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/directedbits/recur/src/app/cli.Version=${VERSION}" \
      -o /out/recur ./src/app/recur
RUN CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/directedbits/recur/src/app/cli.Version=${VERSION}" \
      -o /out/recurd ./src/app/recurd

# Build selected plugins
RUN mkdir -p /out/plugins && \
    for plugin in ${RECUR_PLUGINS}; do \
      echo "Building plugin: ${plugin}" && \
      CGO_ENABLED=0 go build -ldflags "-s -w" \
        -o /out/plugins/${plugin}/${plugin} ./plugins/${plugin}/ && \
      cp ./plugins/${plugin}/manifest.yaml /out/plugins/${plugin}/; \
    done

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tini

# Create non-root user
RUN addgroup -S recur && adduser -S -G recur recur

# Copy binaries
COPY --from=build /out/recur /usr/local/bin/recur
COPY --from=build /out/recurd /usr/local/bin/recurd
COPY --from=build /out/plugins/ /home/recur/.recur/plugins/

# Copy entrypoint script
COPY packaging/docker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Create state and config directories
RUN mkdir -p /home/recur/.recur/state && \
    chown -R recur:recur /home/recur/.recur

USER recur
WORKDIR /home/recur

# Environment
ENV RECUR_CONFIG_PATH=""
ENV RECUR_SOCKET=""

EXPOSE 8080

VOLUME ["/home/recur/.recur/state"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["recur", "status"]

ENTRYPOINT ["tini", "--"]
CMD ["docker-entrypoint.sh"]
