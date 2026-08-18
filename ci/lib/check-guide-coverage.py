#!/usr/bin/env python3
"""校验主流水线指南 Job 清单覆盖 ray/ 下全部指南。

用法: check-guide-coverage.py --pipeline-yaml <ci-templates/tutorials-validate.yaml>

指南 = ray/ 下每个包含 README.md 的目录。validate 阶段的 use-pipeline
指南 Job 清单 + SKIP_DIRS 必须恰好覆盖全部指南：缺失（新指南没加 Job）或
多余（指南已删除但 Job 未清理）都会以非零退出，提示维护
ci-templates/tutorials-validate.yaml。

重试链约定：每个指南恰好 3 个 use-pipeline Job（基建抖动重试最多 2 次），
Job id 为 <base>-t1 / <base>-t2 / <base>-t3（同一 base），且 params 中的
guide_dir 一致。不满足（缺尝试 Job、后缀不规范、base 不一致）同样判失败。

# 使用说明（新版分支布局）
本脚本原设计里，流水线 YAML 与本脚本、以及 ray/ 都在同一次 checkout 后
可用。新方案里内容分支只放 ray/，脚本与流水线 YAML 都在 CI 分支上，
`coverage-check` Job 显式 `git fetch <ci_branch>` 并 `git checkout -- ci/
ci-templates/` 后本脚本才可执行；ray/ 由主 checkout 提供（触发提交在
内容分支）。
"""
import argparse
import json
import os
import re
import sys
from collections import defaultdict

import yaml

GUIDE_ROOT = "ray"
DEFAULT_PIPELINE_YAML = "ci-templates/tutorials-validate.yaml"

# 明确排除、不进清单的指南目录（写明原因）：
SKIP_DIRS = {
    # 测试集群无法安装云原生 AI 套件（ack-ai-installer），cGPU 指南暂不验证。
    "ray/2-advanced/gpu-sharing-with-cgpu",
}

ATTEMPT_SUFFIXES = ("t1", "t2", "t3")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--pipeline-yaml",
        default=DEFAULT_PIPELINE_YAML,
        help=f"流水线 YAML 路径（默认 {DEFAULT_PIPELINE_YAML}）",
    )
    # 兼容旧签名：`check-guide-coverage.py <yaml>`。
    ap.add_argument("legacy_yaml", nargs="?", help=argparse.SUPPRESS)
    args = ap.parse_args()
    yaml_path = args.legacy_yaml or args.pipeline_yaml

    with open(yaml_path, encoding="utf-8") as f:
        pipeline = yaml.safe_load(f)

    jobs = pipeline["stages"]["validate"]["jobs"]
    # guide_dir -> {suffix: [job_id, ...]}
    groups: dict[str, dict[str, list]] = defaultdict(lambda: defaultdict(list))
    ok = True
    for job_id, job in jobs.items():
        if not isinstance(job, dict) or job.get("uses") != "use-pipeline":
            continue
        params = json.loads(job["inputs"]["params"])
        guide = params.get("guide_dir")
        if not guide:
            print(f"Job {job_id} 的 inputs.params 缺少 guide_dir", file=sys.stderr)
            return 2
        m = re.fullmatch(r"(.+)-(t[123])", job_id)
        if not m:
            ok = False
            print(f"Job {job_id} 的 id 不符合 <base>-t1/-t2/-t3 重试链命名约定")
            continue
        groups[guide][m.group(2)].append((m.group(1), job_id))

    for guide, by_suffix in sorted(groups.items()):
        bases = set()
        for suffix in ATTEMPT_SUFFIXES:
            entries = by_suffix.get(suffix, [])
            if len(entries) != 1:
                ok = False
                ids = [job_id for _, job_id in entries]
                print(f"指南 {guide} 的 {suffix} 尝试 Job 应恰好 1 个，"
                      f"实际 {len(entries)} 个：{ids}")
            bases.update(base for base, _ in entries)
        if len(bases) > 1:
            ok = False
            print(f"指南 {guide} 的尝试 Job 前缀不一致：{sorted(bases)}")

    listed = set(groups)
    actual = set()
    for dirpath, _dirnames, filenames in os.walk(GUIDE_ROOT):
        if "README.md" in filenames:
            actual.add(dirpath)

    missing = sorted(actual - listed - SKIP_DIRS)
    extra = sorted((listed | SKIP_DIRS) - actual)
    overlap = sorted(listed & SKIP_DIRS)

    if missing:
        ok = False
        print("以下指南未配置验证 Job，也不在 SKIP_DIRS 中，请更新 "
              "ci-templates/tutorials-validate.yaml：")
        for d in missing:
            print(f"  + {d}")
    if extra:
        ok = False
        print("以下条目在 Job 清单/SKIP_DIRS 中，但 ray/ 下已不存在对应指南：")
        for d in extra:
            print(f"  - {d}")
    if overlap:
        ok = False
        print("以下条目同时出现在 Job 清单与 SKIP_DIRS 中，请二选一：")
        for d in overlap:
            print(f"  ! {d}")

    if ok:
        print(f"覆盖校验通过：{len(listed)} 个指南 ×3 尝试 Job + 跳过 "
              f"{len(SKIP_DIRS)} 个 = 指南 {len(actual)} 个")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
