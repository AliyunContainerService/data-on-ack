# Commit Agent

Commit Agent is a gRPC-based sidecar agent that runs inside Jupyter Notebook pods. It enables code synchronization between the notebook container and a remote Git repository. For usage details, refer to the [official documentation](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/user-guide/create-and-use-a-jupyter-notebook?spm=a2c4g.11186623.0.0.434e4497kN54rC#acdef32034shm).

## Build

```shell
go mod tidy && go mod vendor
make build && make build-client
```

## Generate gRPC Code

```shell
cd v1beta1
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    service.proto
```
