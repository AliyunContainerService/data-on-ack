FROM openjdk:8-jre
MAINTAINER "huozhixin.hzx@alibaba-inc.com"

ARG arenaVersion="0.9.11-14fa45c"
COPY bin/arena-installer-${arenaVersion}-linux-amd64.tar.gz /opt
COPY .git-commit /.git-commit

RUN cd /opt && \
    tar -xzvf arena-installer-${arenaVersion}-linux-amd64.tar.gz && \
    cp ./arena-installer/bin/arena /usr/bin/arena && \
    cp ./arena-installer/bin/helm /usr/bin/arena-helm && \
    cp ./arena-installer/bin/kubectl /usr/bin/arena-kubectl && \
    cp -r ./arena-installer/charts /charts && \
    rm -rf ./arena-installer && \
    rm -f arena-installer-${arenaVersion}-linux-amd64.tar.gz

WORKDIR /workspace

COPY backend/target/ai-dashboard.jar /workspace/

ENTRYPOINT [ "java", "-jar", "ai-dashboard.jar" ]
