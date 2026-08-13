#!/bin/bash
# Build UCX with the Alibaba Cloud eRDMA patch.
#
# Why: stock UCX cannot use eRDMA. Its rc_verbs transport always creates a
# shared receive queue, and eRDMA does not implement SRQ
# (ibv_create_srq() -> "Operation not supported"). The patch published on
# mirrors.cloud.aliyuncs.com replaces the SRQ with per-QP receive queues and
# adds UCX_RC_VERBS_USE_SRQ / UCX_RC_VERBS_RX_CQ_LEN.
#
# The patch is published against UCX 1.19.0 but applies cleanly to 1.21.0,
# which is the version the prebuilt NIXL wheels are compiled against - build
# 1.21.0 if you want to plug it under NIXL, 1.19.0 if you only need UCX.
#
# Run this inside a pod that has the eRDMA userspace libraries (see the
# Dockerfile one level up), with the source tarball already in /tmp:
#   curl -LO https://github.com/openucx/ucx/releases/download/v1.19.0/ucx-1.19.0.tar.gz
#   # or, for 1.21.0:
#   curl -L -o ucx-1.21.0.tar.gz https://codeload.github.com/openucx/ucx/tar.gz/refs/tags/v1.21.0
#
# Usage: build_ucx_erdma.sh <version> [prefix]
set -euo pipefail

VERSION="${1:-1.21.0}"
PREFIX="${2:-/opt/ucx-erdma}"
SRC="/tmp/ucx-${VERSION}.tar.gz"

sudo apt-get update -qq
sudo apt-get install -y --no-install-recommends \
  build-essential autoconf automake libtool \
  libibverbs-dev librdmacm-dev libnuma-dev

cd /tmp
rm -rf ucx-build && mkdir ucx-build
tar -xf "$SRC" -C ucx-build --strip-components=1
cd ucx-build

curl -sS -o ucx-for-erdma.patch \
  http://mirrors.cloud.aliyuncs.com/erdma/patch/ucx-1.19.0-for-erdma.patch
patch -p1 --fuzz=5 < ucx-for-erdma.patch

# Release tarballs ship a configure script; git tarballs do not.
[ -x ./configure ] || ./autogen.sh

# GDA-KI needs NVIDIA DOCA GPUNetIO headers that are not installed here; the
# __has_include() guard in uct_device_impl.h picks the header up from the
# source tree and then fails, so hide it.
[ -f src/uct/ib/mlx5/gdaki/gdaki.cuh ] && \
  mv src/uct/ib/mlx5/gdaki/gdaki.cuh src/uct/ib/mlx5/gdaki/gdaki.cuh.disabled

# --enable-mt is required by NIXL ("UCX library does not support
# multi-threading" otherwise).
./configure --prefix="$PREFIX" \
  --enable-optimizations \
  --enable-mt \
  --with-verbs \
  --with-rdmacm \
  --with-cuda=/usr/local/cuda \
  --without-gda \
  --without-go \
  --without-java \
  --disable-numa

make -j"$(nproc)"
sudo make install

"$PREFIX/bin/ucx_info" -v
echo "UCX with eRDMA support installed to $PREFIX"
