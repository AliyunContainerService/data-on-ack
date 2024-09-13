ARG GOLANG_VERSION=1.15
FROM golang:${GOLANG_VERSION} as builder

WORKDIR /workspace

COPY . /workspace/notebook-controller

RUN cd /workspace/notebook-controller && go mod download

WORKDIR /workspace/notebook-controller

RUN if [ "$(uname -m)" = "aarch64" ]; then \
        CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GO111MODULE=on go build -a -o manager main.go; \
    else \
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go; \
    fi

FROM ubuntu:22.04
WORKDIR /
COPY --from=builder /workspace/notebook-controller/manager .
COPY --from=builder /workspace/notebook-controller/third_party/license.txt third_party/license.txt
COPY --from=builder /go/pkg/mod/github.com/hashicorp third_party/hashicorp
RUN apt-get update && apt-get upgrade -y && rm -rf /var/cache/apt/
ENTRYPOINT ["/manager"]
