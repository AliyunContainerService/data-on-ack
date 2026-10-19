FROM pytorch/pytorch:2.7.1-cuda12.8-cudnn9-devel

LABEL description="VLA Pipeline pre-built image with Data-Juicer,  MoGe-2, HaWoR, MegaSaM, PyTorch3D support"

ENV DEBIAN_FRONTEND=noninteractive
ENV TORCH_CUDA_ARCH_LIST="7.0;7.5;8.0;8.6;8.9;9.0;12.0"
ENV FORCE_CUDA=1

RUN apt-get update && apt-get install -y --no-install-recommends \
        git \
        wget \
        vim \
        curl \
        ffmpeg \
        libgl1-mesa-glx \
        libglib2.0-0 \
        libsm6 \
        libxext6 \
        libxrender1 \
        build-essential \
        ninja-build \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir "py-data-juicer==1.6.0"
RUN pip install --no-cache-dir "ray[data,default]==2.56.0"
RUN pip install --no-cache-dir --force-reinstall "setuptools==69.5.1"

# Verify at build time that the PyTorch binary includes Blackwell (sm_120) support.
RUN python -c 'import torch; archs = torch._C._cuda_getArchFlags().split(); assert torch.version.cuda == "12.8", torch.version.cuda; assert "sm_120" in archs, archs'

RUN git clone --depth 1 https://github.com/ThunderVVV/HaWoR.git /root/.cache/data_juicer/assets/HaWoR

# modify requirements.txt:
#   - remove chumpy (need --no-build-isolation, install separately)
#   - remove aitviewer / moderngl-window / pyrender (OpenGL rendering, no head environment needed)
#   - relax torch-scatter version locking (install the PyTorch 2.7 / CUDA 12.8 wheel separately)
#   - relax numpy version locking (to avoid dependency resolution conflicts)
RUN cd /root/.cache/data_juicer/assets/HaWoR \
    && sed -i '/^chumpy/d' requirements.txt \
    && sed -i '/^aitviewer/d' requirements.txt \
    && sed -i '/^moderngl-window/d' requirements.txt \
    && sed -i '/^pyrender/d' requirements.txt \
    && sed -i 's/^torch-scatter==.*/# &/' requirements.txt \
    && sed -i 's/^numpy==.*/numpy/' requirements.txt

RUN pip install --no-cache-dir "torch-scatter==2.1.2" -f https://data.pyg.org/whl/torch-2.7.0+cu128.html

# MoGe-2 camera calibration
RUN pip install --no-cache-dir "moge @ git+https://github.com/microsoft/MoGe.git@925b8ed835a7a9cdb7578ba15c658a0afc969030"

# HaWoR dependencies (install from requirements.txt)
RUN pip install --no-cache-dir -r /root/.cache/data_juicer/assets/HaWoR/requirements.txt
# chumpy needs --no-build-isolation, install separately
RUN pip install --no-cache-dir --no-build-isolation "chumpy @ git+https://github.com/mattloper/chumpy"

RUN pip install --no-cache-dir scipy pyarrow av Pillow openai
RUN pip install --no-cache-dir \
    "numpy==1.26.4" \
    "pyarrow>=19,<26" \
    "plyfile==1.1.3" \
    "opencv-python==4.10.0.84" \
    "opencv-contrib-python==4.10.0.84" \
    "pydantic==2.10.6" \
    "click>=7.0"

# Clone MegaSAM and patch source code:
#   - replace .type() with .scalar_type() for newer PyTorch
#   - add L20 (8.9), H20 (9.0), and Blackwell (12.0) CUDA architecture support
RUN git clone --recursive https://github.com/mega-sam/mega-sam.git /root/.cache/data_juicer/assets/mega-sam \
    && cd /root/.cache/data_juicer/assets/mega-sam \
    && sed -i 's/\.type()/\.scalar_type()/g' \
        base/src/altcorr_kernel.cu \
        base/src/correlation_kernels.cu \
        base/src/droid_kernels.cu \
        base/thirdparty/lietorch/lietorch/src/lietorch_gpu.cu \
        base/thirdparty/lietorch/lietorch/src/lietorch_cpu.cpp \
    && sed -i "/compute_86,code=sm_86/a\                    '-gencode=arch=compute_89,code=sm_89',\n                    '-gencode=arch=compute_90,code=sm_90',\n                    '-gencode=arch=compute_120,code=sm_120'," base/setup.py

# Build droid_backends and lietorch in the shared Python environment.
RUN cd /root/.cache/data_juicer/assets/mega-sam/base && python setup.py install

# Fail the image build on incompatible dependencies or unavailable runtime modules.
RUN pip check \
    && python -c "import torch; import cv2, data_juicer, droid_backends, lietorch, numpy, plyfile, pyarrow, pydantic, ray, ray.data; \
assert data_juicer.__version__ == '1.6.0'; assert ray.__version__ == '2.56.0'; \
assert numpy.__version__ == '1.26.4'; assert int(pyarrow.__version__.split('.')[0]) < 26, pyarrow.__version__; \
assert cv2.__version__ == '4.10.0'; assert pydantic.__version__ == '2.10.6'; \
assert torch.version.cuda == '12.8', torch.version.cuda"

RUN pip cache purge 2>/dev/null || true

WORKDIR /workspace

ENTRYPOINT ["python"]
