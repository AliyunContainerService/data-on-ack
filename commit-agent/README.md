# Commit-Agent
The usage of commit-agent can refer to the [documentation](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/user-guide/create-and-use-a-jupyter-notebook?spm=a2c4g.11186623.0.0.434e4497kN54rC#acdef32034shm).

Run to build
```shell
go mod tidy && do mod vendor
make build && make build-client 
```

## generate grpc code

```shell
cd v1beta1
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    service.proto
```