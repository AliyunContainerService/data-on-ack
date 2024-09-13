FROM golang:1.19 as builder

WORKDIR /opt

COPY . .

RUN GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o /usr/bin/commit-agent /opt/cmd/commit-agent/main.go

FROM alpine:3.16

ENV DOCKER_HOST=unix:///host/run/docker.sock
COPY --from=builder /usr/bin/commit-agent /usr/bin/commit-agent
RUN chmod +x /usr/bin/commit-agent

ENTRYPOINT ["/usr/bin/commit-agent"]
