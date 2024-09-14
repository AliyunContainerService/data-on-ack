#!/usr/bin/env bash

go mod vendor
retVal=$?
if [ $retVal -ne 0 ]; then
    exit $retVal
fi

set -e
TMP_DIR=$(mktemp -d)
mkdir -p "${TMP_DIR}"/src/github.com/AliyunContainerService/data-on-ack/ai-dev-console
cp -r ./{apis,hack,vendor} "${TMP_DIR}"/src/github.com/AliyunContainerService/data-on-ack/ai-dev-console/
echo "start to generate client..."

(cd "${TMP_DIR}"/src/github.com/AliyunContainerService/data-on-ack/ai-dev-console; \
    GOPATH=${TMP_DIR} GO111MODULE=off /bin/bash vendor/k8s.io/code-generator/generate-groups.sh all \
    github.com/AliyunContainerService/data-on-ack/ai-dev-console/client github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis "training:v1alpha1 apps:v1alpha1" -h ./hack/boilerplate.go.txt)

rm -rf ./client/{clientset,informers,listers}
mv "${TMP_DIR}"/src/github.com/AliyunContainerService/data-on-ack/ai-dev-console/client/* ./client
