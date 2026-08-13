#!/bin/bash
# Build the Mooncake Transfer Engine shared library and NIXL 1.3.0 with the
# Mooncake backend enabled, on top of the eRDMA-patched UCX.
#
# The pip package mooncake-transfer-engine ships only a Python module and a
# benchmark binary - the NIXL plugin links against libtransfer_engine.so, so
# Mooncake has to be built from source.
#
# Prerequisites in /tmp (download outside the cluster, github is not reliably
# reachable from inside):
#   mooncake-src.tgz  https://codeload.github.com/kvcache-ai/Mooncake/tar.gz/refs/tags/v0.3.12.post1
#   ylt.tgz           https://codeload.github.com/alibaba/yalantinglibs/tar.gz/refs/tags/0.5.5
#   pybind11.tgz      https://codeload.github.com/pybind/pybind11/tar.gz/refs/tags/v2.13.6
#   nixl.tgz          https://codeload.github.com/ai-dynamo/nixl/tar.gz/refs/tags/v1.3.0
# plus a UCX built by build_ucx_erdma.sh (default prefix below).
set -euo pipefail

UCX_PREFIX="${UCX_PREFIX:-/opt/ucx121-erdma}"
NIXL_PREFIX="${NIXL_PREFIX:-/opt/nixl}"

sudo apt-get update -qq
sudo apt-get install -y --no-install-recommends \
  cmake build-essential libgflags-dev libgoogle-glog-dev libjsoncpp-dev \
  libnuma-dev libcurl4-openssl-dev libyaml-cpp-dev libssl-dev uuid-dev \
  libunwind-dev pkg-config
pip install -q meson ninja meson-python pybind11 patchelf

# --- yalantinglibs (a Mooncake submodule, absent from the release tarball) ---
cd /tmp && rm -rf ylt && mkdir ylt && tar -xzf ylt.tgz -C ylt --strip-components=1
cd ylt && mkdir -p build && cd build
cmake .. -DBUILD_EXAMPLES=OFF -DBUILD_BENCHMARK=OFF -DBUILD_UNIT_TESTS=OFF \
  -DCMAKE_BUILD_TYPE=Release
sudo make install

# --- Mooncake transfer engine ---
cd /tmp && rm -rf mooncake-src && mkdir mooncake-src
tar -xzf mooncake-src.tgz -C mooncake-src --strip-components=1
cd mooncake-src
# pybind11 is another missing submodule that the top-level CMakeLists
# add_subdirectory()s.
rm -rf extern/pybind11 && mkdir -p extern/pybind11
tar -xzf /tmp/pybind11.tgz -C extern/pybind11 --strip-components=1

mkdir -p build && cd build
cmake .. \
  -Dpybind11_DIR="$(python -c 'import pybind11; print(pybind11.get_cmake_dir())')" \
  -DBUILD_SHARED_LIBS=ON \
  -DWITH_TE=ON -DWITH_STORE=OFF -DWITH_STORE_RUST=OFF -DWITH_P2P_STORE=OFF \
  -DWITH_RUST_EXAMPLE=OFF -DUSE_ETCD=OFF -DUSE_REDIS=OFF -DUSE_HTTP=ON \
  -DUSE_CUDA=ON -DBUILD_UNIT_TESTS=OFF -DBUILD_EXAMPLES=OFF -DBUILD_BENCHMARK=OFF \
  -DCMAKE_BUILD_TYPE=Release
make -j"$(nproc)"
sudo make install
sudo ldconfig
ls -l /usr/local/lib/libtransfer_engine.so

# --- NIXL with UCX + Mooncake ---
cd /tmp && rm -rf nixl-src && mkdir nixl-src
tar -xzf nixl.tgz -C nixl-src --strip-components=1
cd nixl-src
export PKG_CONFIG_PATH="$UCX_PREFIX/lib/pkgconfig:${PKG_CONFIG_PATH:-}"
export LD_LIBRARY_PATH="$UCX_PREFIX/lib:/usr/local/lib:${LD_LIBRARY_PATH:-}"
meson setup build \
  -Ducx_path="$UCX_PREFIX" \
  -Ddisable_mooncake_backend=false \
  -Dbuild_docs=false \
  --prefix="$NIXL_PREFIX"
ninja -C build
sudo -E env "PATH=$PATH" ninja -C build install

ls "$NIXL_PREFIX"/lib/*/plugins/
echo "NIXL with UCX + Mooncake installed to $NIXL_PREFIX"
