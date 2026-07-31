FROM --platform=$BUILDPLATFORM golang:1.21 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_SHORT_COMMIT=none

WORKDIR /opt
COPY . .

# Build the agent. -trimpath strips local paths so binaries are reproducible
# across build hosts.
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build -trimpath \
    -ldflags "-s -w -X main.Version=${VERSION} -X main.GitCommit=${GIT_SHORT_COMMIT}" \
    -o /usr/bin/commit-agent ./cmd/commit-agent

# Bake the CLI into the agent image so the DaemonSet can use it for liveness
# probes (`ack-commit-ctl version`).
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}-${GIT_SHORT_COMMIT}" \
    -o /usr/bin/ack-commit-ctl ./cmd/commit-ctl

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && update-ca-certificates

ENV DOCKER_HOST=unix:///host/run/docker.sock

COPY --from=builder /usr/bin/commit-agent  /usr/bin/commit-agent
COPY --from=builder /usr/bin/ack-commit-ctl /usr/bin/ack-commit-ctl

RUN chmod +x /usr/bin/commit-agent /usr/bin/ack-commit-ctl

ENTRYPOINT ["/usr/bin/commit-agent"]
