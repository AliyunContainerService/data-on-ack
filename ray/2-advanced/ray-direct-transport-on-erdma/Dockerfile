# Ray image with eRDMA userspace libraries installed.
#
# The base image is Ray 2.57.0 (the first release with Ray Direct Transport).
# In regions in China, use the relayed copy on ACR instead of docker.io:
#   rayproject/ray-llm:2.57.0-py312-cu130
ARG BASE_IMAGE=rayproject/ray-llm:2.57.0-py312-cu130
FROM ${BASE_IMAGE}

USER root

# Install the eRDMA userspace stack (libibverbs with the erdma provider)
# from the Alibaba Cloud eRDMA APT repository. The repository host
# mirrors.cloud.aliyuncs.com is only reachable from inside a VPC, which is
# why this image is built with an in-cluster BuildKit daemon.
RUN apt-get update -qq && apt-get install -y -qq wget >/dev/null 2>&1 && \
    wget -qO /tmp/erdma.gpg.key http://mirrors.cloud.aliyuncs.com/erdma/GPGKEY && \
    gpg --batch --yes --dearmour -o /etc/apt/trusted.gpg.d/erdma.gpg /tmp/erdma.gpg.key && \
    echo "deb [ ] http://mirrors.cloud.aliyuncs.com/erdma/apt/ubuntu jammy/erdma main" \
      > /etc/apt/sources.list.d/erdma.list && \
    apt-get update -qq && \
    apt-get install -y --no-install-recommends \
      libibverbs1 ibverbs-providers ibverbs-utils librdmacm1 && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/erdma.gpg.key

USER ray

# Ray Direct Transport moves GPU tensors with NCCL. The NCCL that ships with
# torch 2.11 (2.28.9) segfaults inside ncclNetInit() as soon as it probes an
# eRDMA device, and so does 2.28.7; 2.28.3, 2.29.7 and 2.30.7 are fine. Pin a
# known-good version, installed from the in-VPC PyPI mirror.
RUN pip install --no-cache-dir \
      -i http://mirrors.cloud.aliyuncs.com/pypi/simple \
      --trusted-host mirrors.cloud.aliyuncs.com \
      nvidia-nccl-cu13==2.30.7
