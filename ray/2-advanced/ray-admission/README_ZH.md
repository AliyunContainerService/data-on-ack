# Ray Admission 使用说明

许可状态、故障处理及存储优化方案见 [控制器设计](DESIGN.md)。

本目录是独立的 `ray-admission` Python 安装包，提供 `ray-admission` CLI 和
`ray_admission.queue` 模块。当前 `ray-admission list` 只有 `queue` 实现：
它在 Ray task 或 actor 提交前检查资源配额，不足时让 `.remote()` 等待。

控制器的 Ray actor 名称和 namespace 均固定为 `ray-submission-queue`。
`RAY_NAMESPACE` 只用于确定业务的**配额 namespace**，配额键是
`(RAY_NAMESPACE, Ray Job ID)`；未设置时使用 `ray-submission-queue`。

## 构建与安装

在虚拟环境中构建 wheel，并把同一 wheel 安装到管理员、业务 driver 和 Ray
worker 的 Python 环境。代码使用 Ray 私有接口，已在 Ray 2.58 验证。

```bash
python build_wheel.py
python -m pip install dist/ray_admission-0.1.0-py3-none-any.whl
ray-admission list
```

## 管理员启动与配额配置

```bash
export RAY_ADMISSION_ADMIN_TOKEN='用安全随机源生成的长令牌'
ray-admission queue --address auto start
ray-admission queue --address auto status

export RAY_NAMESPACE=my-team
ray-admission queue --address auto quota set <ray-job-id> '{"CPU": 4, "GPU": 1}'
ray-admission queue --address auto quota delete <ray-job-id>
```

`<ray-job-id>` 是业务 driver 中
`ray.get_runtime_context().get_job_id()` 的返回值，和 Ray Jobs submission ID
不同。`quota set` 替换该 namespace 下这个 Job 的整个资源向量；未列出的资源维度
不限额，`{"GPU": 0}` 会阻止新的 GPU 申请。未配置的 Job 默认不限额，应在
业务提交前完成配额配置。`status` 按配额 namespace、Job ID 展示用量与额度。

`start` 创建 detached 控制器，CLI 退出后仍运行。
管理员令牌只用于启动和修改配额，不要放进业务 Job。`list` 和 `status` 不需要
该令牌。
配额配置、管理员令牌摘要和运行中的资源预留以独立记录保存在 Ray 自带的 GCS KV，
不需要额外数据库或存储配置。控制器 actor
意外失败后会自动重启，并恢复 task 的 ObjectRef 和 actor ID，继续监控其结束。
Ray 会重试进行中的准入调用；等待请求重新进入各 Job 的队列。计数和故障前
等待请求的精确顺序不会恢复。故障前已获许可但提交端尚未上报跟踪对象的预留会
保留，直到提交端主动释放。数据持久性遵循 GCS 配置，ObjectRef 只适用于原 Ray 集群。
释放记录在客户端确认水位后清理，每个进程的 session 水位仍保留，用于拒绝迟到重试。
旧控制器停止后，新控制器会在接受请求前迁移旧 GCS 快照。逐记录写入通过 redo 日志
恢复中途失败；GCS KV 不提供多键事务或独立的写入者 fencing，依赖固定命名 actor 单写。
若 actor 在控制器停机期间结束，重建后的控制器读到 GCS 的 `DEAD` 状态后会
释放其预留；若该状态已不可用，则保守地继续保留预留量。

## 业务 driver 接入

```python
import ray
from ray_admission.queue import enable_admission

ray.init(address="auto")
enable_admission()

@ray.remote(num_cpus=1)
def work(value):
    return value + 1

print(ray.get(work.remote(1)))
```

在业务 driver 中设置 `RAY_NAMESPACE=my-team`，就会使用管理员配置给
`my-team` 的配额。Kubernetes 可用 Downward API 注入：

```yaml
env:
  - name: RAY_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
```

新提交的 task 和 actor 的 worker 通过启动 hook 沿用同一配额 namespace；
已运行的 actor 和其他 driver 不会被追溯接管。普通 task 的全部返回引用就绪后
归还配额，actor 在 GCS 报告 `DEAD` 后归还。该原型不覆盖 Ray Client、Java
或 C++ 提交。
