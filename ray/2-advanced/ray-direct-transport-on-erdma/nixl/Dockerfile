# Ray image with eRDMA userspace libraries, a UCX rebuilt with Alibaba Cloud's
# eRDMA patch, the Mooncake Transfer Engine, and NIXL built against both.
#
# Base is the image produced by ../Dockerfile (eRDMA userspace + pinned NCCL).
#
# The five source tarballs must sit next to this Dockerfile in the build
# context; github is not reachable from inside the cluster, so download them
# outside and copy them in:
#   ucx-1.21.0.tar.gz  https://codeload.github.com/openucx/ucx/tar.gz/refs/tags/v1.21.0
#   ylt.tgz            https://codeload.github.com/alibaba/yalantinglibs/tar.gz/refs/tags/0.5.5
#   pybind11.tgz       https://codeload.github.com/pybind/pybind11/tar.gz/refs/tags/v2.13.6
#   mooncake-src.tgz   https://codeload.github.com/kvcache-ai/Mooncake/tar.gz/refs/tags/v0.3.12.post1
#   nixl.tgz           https://codeload.github.com/ai-dynamo/nixl/tar.gz/refs/tags/v1.3.0
#
# NIXL's meson build additionally pulls six subprojects from the internet, so
# those have to be pre-staged in the context as well:
#   nixl-packagecache.tar.gz      a tarball of a directory containing the
#                                 source_url/patch_url of every .wrap under
#                                 nixl-1.3.0/subprojects/ except
#                                 prometheus-cpp, i.e.
#                                   abseil-cpp-20250814.1.tar.gz
#                                   abseil-cpp_20250814.1-1_patch.zip
#                                   asio-1.30.2.tar.gz
#                                   asio_1.30.2-2_patch.zip
#                                   liburing-2.14.tar.gz
#                                   taskflow-3.10.0.tar.gz
#                                   tomlplusplus-3.4.0.tar.gz
#   prometheus-cpp-v1.3.0.tar.gz  prometheus-cpp.wrap is a wrap-git, so it
#                                 needs a clone rather than a tarball:
#                                   git clone --depth 1 --branch v1.3.0 \
#                                     --recurse-submodules \
#                                     https://github.com/jupp0r/prometheus-cpp.git p
#                                   tar -czf prometheus-cpp-v1.3.0.tar.gz -C p .
ARG BASE_IMAGE=registry.cn-beijing.aliyuncs.com/image-yueming/ray-llm:2.57.0-py312-cu130-erdma
FROM ${BASE_IMAGE}

USER root

RUN apt-get update -qq && apt-get install -y --no-install-recommends \
      build-essential autoconf automake libtool cmake pkg-config \
      libibverbs-dev librdmacm-dev libnuma-dev \
      libgflags-dev libgoogle-glog-dev libjsoncpp-dev libcurl4-openssl-dev \
      libyaml-cpp-dev libssl-dev uuid-dev libunwind-dev \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
      -i http://mirrors.cloud.aliyuncs.com/pypi/simple \
      --trusted-host mirrors.cloud.aliyuncs.com \
      meson ninja meson-python pybind11 patchelf

COPY ucx-1.21.0.tar.gz ylt.tgz pybind11.tgz mooncake-src.tgz nixl.tgz \
     nixl-packagecache.tar.gz prometheus-cpp-v1.3.0.tar.gz /src/

# --- UCX with the eRDMA patch -------------------------------------------
# Stock UCX cannot use eRDMA: rc_verbs always creates a shared receive queue
# and eRDMA has no SRQ. The patch replaces the SRQ with per-QP receive queues.
# It is published against 1.19.0 but applies cleanly to 1.21.0, which is the
# version NIXL 1.3 expects (1.19 lacks ucp_device_mem_list_release).
# --enable-mt is mandatory, NIXL refuses a single-threaded UCX.
RUN mkdir -p /src/ucx && tar -xf /src/ucx-1.21.0.tar.gz -C /src/ucx --strip-components=1 \
 && cd /src/ucx \
 && curl -sS -o ucx-for-erdma.patch \
      http://mirrors.cloud.aliyuncs.com/erdma/patch/ucx-1.19.0-for-erdma.patch \
 && patch -p1 --fuzz=5 < ucx-for-erdma.patch \
 && { [ -x ./configure ] || ./autogen.sh; } \
 && mv src/uct/ib/mlx5/gdaki/gdaki.cuh src/uct/ib/mlx5/gdaki/gdaki.cuh.disabled \
 && ./configure --prefix=/opt/ucx-erdma \
      --enable-optimizations --enable-mt \
      --with-verbs --with-rdmacm --with-cuda=/usr/local/cuda \
      --without-gda --without-go --without-java --disable-numa \
 && make -j"$(nproc)" && make install \
 && cd / && rm -rf /src/ucx

# --- yalantinglibs (a Mooncake submodule, absent from the tarball) -------
RUN mkdir -p /src/ylt && tar -xzf /src/ylt.tgz -C /src/ylt --strip-components=1 \
 && cd /src/ylt && mkdir build && cd build \
 && cmake .. -DBUILD_EXAMPLES=OFF -DBUILD_BENCHMARK=OFF -DBUILD_UNIT_TESTS=OFF \
      -DCMAKE_BUILD_TYPE=Release \
 && make install \
 && cd / && rm -rf /src/ylt

# --- Mooncake Transfer Engine -------------------------------------------
# The PyPI package ships no libtransfer_engine.so, which is what the NIXL
# Mooncake plugin links against, so build it from source. pybind11 is another
# missing submodule that the top-level CMakeLists add_subdirectory()s.
RUN mkdir -p /src/mooncake && tar -xzf /src/mooncake-src.tgz -C /src/mooncake --strip-components=1 \
 && cd /src/mooncake \
 && mkdir -p extern/pybind11 \
 && tar -xzf /src/pybind11.tgz -C extern/pybind11 --strip-components=1 \
 && mkdir build && cd build \
 && cmake .. \
      -Dpybind11_DIR="$(python -c 'import pybind11; print(pybind11.get_cmake_dir())')" \
      -DBUILD_SHARED_LIBS=ON \
      -DWITH_TE=ON -DWITH_STORE=OFF -DWITH_STORE_RUST=OFF -DWITH_P2P_STORE=OFF \
      -DWITH_RUST_EXAMPLE=OFF -DUSE_ETCD=OFF -DUSE_REDIS=OFF -DUSE_HTTP=ON \
      -DUSE_CUDA=ON -DBUILD_UNIT_TESTS=OFF -DBUILD_EXAMPLES=OFF -DBUILD_BENCHMARK=OFF \
      -DCMAKE_BUILD_TYPE=Release \
 && make -j"$(nproc)" && make install && ldconfig \
 && cd / && rm -rf /src/mooncake

# --- NIXL with the UCX and Mooncake backends -----------------------------
# meson resolves its subprojects from subprojects/packagecache when the file
# is already there, and skips the wrap-git clone when the subproject directory
# exists - so pre-stage both and forbid downloads outright, otherwise a
# missing file turns into minutes of retrying against an unreachable github.
RUN mkdir -p /src/nixl && tar -xzf /src/nixl.tgz -C /src/nixl --strip-components=1 \
 && cd /src/nixl \
 && mkdir -p subprojects/packagecache subprojects/prometheus-cpp \
 && tar -xzf /src/nixl-packagecache.tar.gz -C subprojects/packagecache \
 && tar -xzf /src/prometheus-cpp-v1.3.0.tar.gz -C subprojects/prometheus-cpp \
 && PKG_CONFIG_PATH=/opt/ucx-erdma/lib/pkgconfig \
    LD_LIBRARY_PATH=/opt/ucx-erdma/lib:/usr/local/lib \
    meson setup build \
      --wrap-mode=nodownload \
      -Ducx_path=/opt/ucx-erdma \
      -Ddisable_mooncake_backend=false \
      -Dbuild_docs=false \
      --prefix=/opt/nixl \
 && ninja -C build && ninja -C build install \
 && cd / && rm -rf /src/nixl /src/*.tgz /src/*.tar.gz

# The prebuilt python bindings in the image pick the plugins up from here.
ENV NIXL_PLUGIN_DIR=/opt/nixl/lib/x86_64-linux-gnu/plugins
ENV LD_LIBRARY_PATH=/opt/nixl/lib/x86_64-linux-gnu:/opt/ucx-erdma/lib:/usr/local/lib:${LD_LIBRARY_PATH}
ENV PATH=/opt/ucx-erdma/bin:${PATH}

# UCX settings required by the eRDMA patch; harmless on other fabrics.
ENV UCX_RC_VERBS_USE_SRQ=n
ENV UCX_RC_VERBS_RX_CQ_LEN=131072
ENV UCX_UD_VERBS_TX_MIN_INLINE=128

USER ray
