# 教程验证 Agent 的执行镜像。
#
# 基于官方 Agentic Pipeline 执行镜像（Debian 13，自带 Agent CLI、jq、PyYAML），
# 只叠加指南验证缺失的命令行工具：kubectl / helm。
#
# 设计约束：
#   1. Agentic Pipeline 的 image 只接受集团内部仓（hub.docker.alibaba-inc.com），
#      且自定义镜像必须预装 Agent CLI —— 因此必须 FROM 官方执行镜像，不能自建基础镜像。
#   2. build-image 的构建机不保证有公网出口，所以二进制统一由 runner 侧的
#      prepare-context.sh 预先下载到 bin/ 目录，构建过程只做 COPY + 安装。
#   3. 基础镜像 tag 升级时同步更新此处与 .aoneci/agent-image.yaml 的 base_image 默认值。
#   4. 用非加速（_multi_arch）变体而非 _accelerated（DADI）变体：build-image 组件
#      对该 DADI 基础镜像的校验自相矛盾（设/不设 accelerate-type 都报错），无法派生。
FROM hub.docker.alibaba-inc.com/aone/agentic-pipeline:20260730100501502765_multi_arch

COPY bin/ /tmp/agent-image-bin/
COPY install.sh /tmp/agent-image-install.sh

# 注：RUN 以基础镜像非 root 用户执行，无法删除 COPY 进来的 root 属主文件，
# 故不在此清理 /tmp（残留二进制相对 GB 级基础镜像可忽略）。
RUN bash /tmp/agent-image-install.sh
