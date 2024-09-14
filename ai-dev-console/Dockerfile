FROM golang:1.18 as builder

WORKDIR /workspace

COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod=vendor -a -o manager main.go

FROM registry-cn-hangzhou.ack.aliyuncs.com/dev/ubuntu:22.04-update
RUN apt-get update && apt-get upgrade -y && rm -rf /var/cache/apt/
WORKDIR /
COPY ./manager .

ENTRYPOINT ["/manager"]
