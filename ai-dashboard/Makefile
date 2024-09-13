# build params
GIT_USER:=$(shell git config --get user.name)
PREFIX?=
VERSION?=1.0.16
GIT_COMMIT:=$(shell git rev-parse --short HEAD)
ARENA_BIN=arena-installer-0.9.11-14fa45c-linux-amd64.tar.gz

# Image URL to use all building/pushing image targets
IMG ?= ${PREFIX}ai-dashboard:$(VERSION)-$(GIT_COMMIT)-aliyun

BACKEND_ROOT=backend/
FRONTEND_ROOT=frontend/

all: docker-build docker-push

get-arena:
	if [ ! -d bin ]; then mkdir bin; fi \
	&& if [ ! -f bin/${ARENA_BIN} ]; then wget http://aliacs-k8s-cn-hongkong.oss-cn-hongkong.aliyuncs.com/arena/${ARENA_BIN} -O bin/${ARENA_BIN}; fi

pkg:
	cd ${BACKEND_ROOT} \
	&& mvn clean \
	&& mvn package -DskipTests \
	&& cd -

build-frontend:
	if [ ! -d ${BACKEND_ROOT}/src/main/resources/static/ ]; then mkdir ${BACKEND_ROOT}/src/main/resources/static/; fi  \
 	&& cd ${FRONTEND_ROOT}  \
 	&& npm install  \
 	&& npm run build:prod  \
 	&& cd - \
 	&& rm -rf ${BACKEND_ROOT}/src/main/resources/static/*  \
 	&& cp -r ${FRONTEND_ROOT}/dist/* ${BACKEND_ROOT}/src/main/resources/static/ \
 	&& ls ${BACKEND_ROOT}/src/main/resources/static/

# Build the docker image
docker-build: get-arena build-frontend pkg
	echo ${GIT_COMMIT} > .git-commit
	docker buildx build --platform linux/amd64 . --no-cache --push -f Dockerfile -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}
