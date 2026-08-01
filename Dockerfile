# syntax=docker/dockerfile:1

# Builds the single deployable (ARCH-01 ADR-0001) for the Docker Compose
# reference environment (ARCH-01 §7). Multi-stage: the builder stage
# compiles a static, CGO-free binary; the runtime stage carries nothing
# but that binary and a minimal Alpine base -- no source, no build
# tooling, no Git metadata.

# Alpine variant tag without a distinct alpine-release suffix -- Docker
# Hub does not publish a golang:1.26.5-alpine3.22 tag; 1.26.5-alpine is
# the exact patch match for this repository's go.mod (go 1.26.5).
FROM golang:1.26.5-alpine AS builder

WORKDIR /src

# Dependency layer cached separately from source so a source-only change
# doesn't force a full module re-download.
COPY go.mod go.sum ./
RUN go mod download

# Only the packages the binary actually needs -- migrations and
# definitions are compiled in via go:embed (migrations/embed.go,
# definitions/embed.go); nothing under docs/, spikes/, or test/ is part
# of the build.
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
COPY definitions ./definitions

# CGO_ENABLED=0: every production dependency (pgx's pure-Go driver,
# golang-migrate's pgx/v5 backend, the k8s.io audit-event types, the YAML
# parser) is pure Go, so the static build succeeds with cgo disabled.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/platform ./cmd/platform

# Alpine, not scratch/distroless: the app healthcheck below and the
# Compose healthcheck both need a real HTTP client, and Alpine's busybox
# provides `wget` out of the box -- no custom healthcheck binary, no
# change to the Go application, no extra packages installed.
FROM alpine:3.22

# Dedicated non-root, non-login system user/group (NFR-014: least
# privilege) -- no shell, no home directory.
RUN addgroup -S cnsdp && adduser -S -H -D -G cnsdp cnsdp

COPY --from=builder /out/platform /usr/local/bin/platform

USER cnsdp:cnsdp

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/platform"]
