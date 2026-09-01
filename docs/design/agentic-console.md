# Agentic Dev Console 设计文档（v2 修订版）
## 基于 agentscope-go 的开发控制台智能化设计

- 状态：**已按对抗性评审修订，待最终批准**（v1→v2 变更记录见附录 A）
- 评审输入：可行性评审（A）、安全红队评审（B）、产品评审（C），报告见 `/tmp/data-on-ack-eval/design-eval-{a,b,c}-*.md`
- 依赖：`github.com/alanfokco/agentscope-go/v2 v2.0.9`（已集成：go.mod 直接依赖、Go 1.25、全模块构建/测试通过、`internal/agent` scaffold 已落地）
- 目标组件：ai-dev-console 新栈（`console/backend/internal/` + `console/frontend-new/`）

---

## 1. 背景与目标

开发控制台目前是"K8s API 之上的 GUI"。引入 agentscope-go（Go 原生、可嵌入的多智能体框架）后，控制台增加一条**智能交互面**：用户以自然语言表达意图，Agent 在**用户自身身份与权限边界内**调用控制台既有能力完成查询、诊断与（经确认的）操作。

**设计三原则（评审后确立）**
1. **Agent 是附加件，不是新依赖**：模型不可达/未配置时，控制台其余功能 100% 不受影响（入口自动隐藏）。
2. **Agent 即用户**：Agent 无独立身份，一切行为 = 调用者身份 × 既有权限边界 × 可审计。
3. **不重复造轮子，也不裸用轮子**：框架 Experimental 层（runtime/service/schedule/workspace/mcp）一律经 `internal/agent` 防腐层使用；框架已自带的（流式/确认/审计原语）优先复用。

## 2. 能力集（评审裁剪后）

### v1 交付（P1+P2 = 最小可信交付）
| 编号 | 能力 | 形态 |
|---|---|---|
| **C2 旗舰：任务故障问答与诊断** | "我的任务为什么失败了" → Agent 拉取任务状态/事件/日志/指标，跨源推理，输出**带引用**的根因分析 | 只读工具；深度诊断 = W1 只读部分并入 |
| C1 自然语言创建（收窄） | 仅两类：创建 Notebook、提交训练任务；参数经确认后执行 | 写入工具 + 强制 HITL |
| C3 资源问答 | "为什么 Pending / 配额还够吗" | **前置工作：先补 quota 数据源 API**（现状：新后端无 quota 接口，前端 GPU 用量硬编码 0——评审实证） |
| 基础设施 | 审计、每日预算硬上限、内存会话、Assistant 抽屉（SSE） | 与能力同阶段交付（评审要求前置） |

### 后续阶段
| 能力 | 阶段 | 说明 |
|---|---|---|
| W1 自愈重提 | P2+ | 诊断给出修正参数 → 参数 diff 预览 → 确认后重提 |
| BYOK + 管理端 | P3 | 用户自带模型 key（成本治理的必要组成）；审计/成本看板；策略只读展示 |
| W5-lite 定时报告 | P3 | 用量报告/闲置提醒（不自动清理）；调度任务持久化 + 启动重注册（框架调度器为内存态） |
| M2 只读 MCP Server | P4（默认关闭） | 供 IDE/外部 Agent 只读查询平台；写入永不开放（MCP 无交互式 HITL 语义）；注意框架 `mcp.NewServer` 是私有网关协议，标准 MCP 需自研 Streamable-HTTP 适配（工作量单列） |
| M4 沙箱代码执行 | P4（独立立项） | `workspace.K8sWorkspace`，六项强制见 §5 |
| W2 实验顾问 | P5 | 基于 Experiments 已有 run 数据做分析建议（**不做** LLM 试错式 HPO——与 Ray Tune/Optuna 竞争必输且双倍烧钱） |
| W3 模型评估选优 | P5 | 依赖评估管道，平台暂不具备 |
| M3 自定义工具插件 | P5 | 需求驱动 |
| ❌ 移出 | — | M1 Agent 团队、M5 A2A 联邦、W4 数据准备代理（依赖的 Jupyter 程序化执行后端不存在；v1 头脑风暴含 W4 而路线图无此条目的矛盾按"砍掉"解决）、自主循环 HPO |

## 3. 总体架构（修订版）

```
浏览器 (frontend-new)
│  Assistant 抽屉：SSE 可重连流（思考/工具调用/确认请求/回答）
▼
ai-dev-console 新后端 (Gin)
├── 既有: /api/v1/{notebooks,training-jobs,serving,datasets,models,metrics,...}
│         （注：读路径走 adminClient——工具化前必须逐接口补属主校验，见 §5.1）
└── internal/agent（防腐层，隔离框架 Experimental API）
    ├── router.go      /api/v1/agent/*（不响应 DISABLE_AUTH 旁路）
    ├── runregistry.go Run = 服务端对象：状态机 + 可重连事件流（解耦 HTTP 生命周期）
    ├── tools/         consoleToolkit（FunctionTool 包装 service 层）
    │                  分级：read / write / destructive + 每工具独立属主/参数校验
    ├── confirm.go     确认单：{runID, toolCallID, 参数快照哈希, 单次, TTL, 属主}
    ├── budget.go      三重上限中间件：轮数 / 工具调用数 / 按计价表的日花费
    ├── audit.go       脱敏全量 + 哈希链审计（非"仅哈希"）
    ├── persist.go     P1 内存会话；P2 AgentSession CRD（仅元数据+压缩摘要）
    ├── scheduler.go   P3：CRD 持久化定时任务 + 启动重注册
    └── model.go       BuildChatModel（已落地）+ FallbackChain + 按 agent 类型模型档案
```

### 3.1 运行生命周期（修复 v1 致命矛盾）
**v1 缺陷**（评审 A 指出）："SSE 断连即取消 run" 与 HITL 无限等待确认互斥——等待确认期间 SSE 空闲必被网关 60s 超时掐断，确认永远无法完成。
**v2 模型**：
- `Run` 是注册在 `RunRegistry` 的**服务端对象**（状态：running → awaiting_confirmation → running → done/failed/canceled）；
- SSE 仅是**订阅通道**，断连不取消；客户端凭 `runID` 重连补发事件（事件带序号）；
- Run 有总时限（如 30min）与空闲时限（等待确认超 24h 自动过期）；
- 用户显式 `stop` 才取消；后台定时/诊断任务同一模型。
- P1 内存态（重启丢失活跃 run，会话历史在内存）；P2 CRD 持久化会话元数据与压缩上下文（**原始消息流水不进 etcd**：etcd 对象上限 ~1.5MB、含敏感数据、watch 压力——评审 B/C 共识）。

### 3.2 工具层（关键安全面）
- 包装方式：`tool.NewFunctionTool(name, description, schema, fn)`（4 参，评审 A 更正）+ `agent.WithToolkit` 注入（框架 `NewReActAgent` 已废弃，统一用 `UnifiedAgent`）。
- **工具层独立强校验**（不依赖上层）：每个工具的 namespace/资源参数都经过 ①会话属主集合校验（**每次执行实时查**，不用登录快照——评审 B：cookie 快照 7 天不随吊销失效）②资源属主校验 ③JSON Schema 校验。
- **读路径真相**（评审 B）：service 层读接口走 adminClient，"RBAC 兜底"只对写路径成立 → 读工具必须逐个做属主校验后才可暴露；本设计 P1 前置工作包含修复评审发现的现存越权读接口（已完成 3 个，见 §5.1）。
- 截断从第一轮开始：框架默认单条工具结果上限 50000 token，日志类工具必须自身先截断（长度+字符集+token 预算）。

### 3.3 HITL 确认（修复"框架无绑定原语"缺口）
框架 `BehaviorAsk` 只把拒绝文本回给模型（模型可换参重发），**不构成安全边界**。v2 自研确认门（工具中间件实现）：
1. write/destructive 工具调用 → 生成**确认单**：冻结参数快照 + 哈希，持久化于 Run；
2. 执行阶段**只执行已确认的快照**（确认 A 执行 B 不可能）；
3. 确认单单次消费、带 TTL、属主绑定（确认 ID 重放/他人批准均失败）；
4. 模型侧：拒绝信息回给模型供调整，但任何新参数 = 新确认单；
5. UX 缓解（不动安全默认）：**计划级批量确认**（一次批准 N 步计划）+ 会话内"本次任务信任该工具"显式 opt-in；destructive 永不放开（二次输入资源名）。

### 3.4 模型配置
- 默认 **qwen-plus**（评审 C：诊断上下文是 token 大户，max 成本不可控）+ `model.NewFallbackChain` 降级链；"深度根因分析"可手动升档；重度用户 BYOK（P3）。
- 平台 key：chart Secret；**BYOK BaseURL 白名单**（评审 B：模型客户端无 SSRF 防护，框架防护只覆盖 WebFetch）。
- key 一律 `model.NewSecretStr`（不落日志/转储）。

### 3.5 会话与代理类型
- agent 类型：`copilot`（问答+创建）、`diagnose`（深度诊断，可升档模型）；
- 管理员**不得**以 admin 身份运行消费租户内容的代理（confused deputy，评审 B）——代理永远在请求者自己的身份下运行；管理端只有聚合视图。

## 4. API 设计（v2）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/agent/sessions` | 创建会话（agentType,title；userName 服务端写入，请求体不可携带） |
| GET | `/api/v1/agent/sessions` | 仅本人（服务端身份过滤） |
| POST | `/api/v1/agent/sessions/:namespace/:name/messages` | 发消息 → 返回 `runID`；SSE 订阅 |
| GET | `/api/v1/agent/runs/:runID/events?after=<seq>` | **可重连**事件流（思考/工具/确认请求/结果） |
| POST | `/api/v1/agent/runs/:runID/confirmations/:cfmID` | 批准（校验快照哈希/单次/属主）；或拒绝 |
| POST | `/api/v1/agent/runs/:runID/stop` | 取消 |
| DELETE | `/api/v1/agent/sessions/:namespace/:name` | 删除（级联删 run/确认单） |
| GET/POST | `/api/v1/agent/policies`（admin） | 工具策略 |

- 全部走 `auth.CheckAuth`；**agent 路由显式不响应 `DISABLE_AUTH`**（仓库各中间件普遍存在该旁路，评审 B/C 均点名）。
- AgentSession 属主模型：CRD 存于系统命名空间（如 `kube-ai`），console SA 独占读写，路由层按 `status.ownerUser` 过滤；**不放共享配额 ns**（评审 B：同 ns 跨用户可读对话史）。

## 5. 安全模型（按红队清单逐项设防）

### 5.1 现存漏洞处置（评审期间实锤，**已修复**）
| # | 漏洞 | 修复 |
|---|---|---|
| 1 | `proxyAuthCheck` 对 `Upgrade: websocket` 跳过 namespace 校验 → 跨租户 Jupyter 终端（RCE） | WebSocket 与 HTTP 同校验（已合入） |
| 2 | `/training-jobs/:ns/:name/logs?pod=` pod 参数无属主校验 → 同 ns 任意 Pod 日志 | 新增 `EnsurePodBelongsToJob`（job-name label / RayCluster 命名 / 名称前缀三重判定，已合入） |
| 3 | `POST /experiments` 缺 `isNamespaceAllowed` → admin 客户端写任意 ns ConfigMap | 补校验（已合入） |

### 5.2 威胁-缓解对照（v2）
| 威胁 | 缓解 |
|---|---|
| 提示词注入（任务名/日志/数据集元数据中的恶意指令） | 模型输出仅是"工具调用提案"；执行面 = 工具层独立校验 + 写操作强制确认 + 用户 RBAC；日志入上下文前截断+字符集过滤；**渲染安全**：回答 Markdown 渲染禁用外链/外部图片（防数据外渗出站腿） |
| 租户逃逸 | 每次执行实时校验属主集合（不用登录快照）；工具参数校验；写路径租户客户端 + RBAC 兜底；读路径工具层独立校验（§3.2） |
| HITL 绕过 | 确认单快照绑定/单次/TTL/属主（§3.3） |
| 预算与 DoS | 自研三重上限（框架 v2.0.9 **无** spend cap，`runtime.Budget` 零值=无限——评审 A 实证）；**按用户**并发会话上限（AgentPool 无公平性，单用户长会话可耗尽池）；SSE 连接数上限 |
| 凭据 | SecretStr；BYOK BaseURL 白名单；审计脱敏；（跟进项：TenantRegistry 长期 SA token → TokenRequest 短期化、cookie 去令牌化） |
| 租户边界过期 | 实时校验 + （跟进项）吊销联动 `TenantRegistry.InvalidateClient`（现状无人调用） |
| 沙箱（P4 门禁，六项强制） | ①平台维护的版本化镜像白名单 ②PVC 仅允许本人数据集（防跨租户挂载外带）③默认拒绝出向 NetworkPolicy（含云元数据 100.100.100.200）④强制 resources+securityContext ⑤令牌不入 argv（tmpfs 0600 文件，多租户进程可见性问题）⑥pods/exec 权限最小化+全审计；运行镜像需补 kubectl（现状无）；`PodTTLSeconds` 实为 activeDeadlineSeconds（只杀不删）→ 配清理机制 |
| MCP（P4 门禁） | 服务端强制只读 + 逐用户身份 + 白名单 + 默认关闭；写入永不开放 |
| 审计不可取证 | 脱敏**全量** + 哈希链（非仅哈希）；OTLP tracing 为自研接入（框架**非**内置，评审 A 更正） |
| 数据出境/隐私 | 日志/对话发往外部模型 API = 数据出集群：**显式 opt-in**（chart 开关+文档声明），提供 VPC 网关/私有化模型替代路径；会话 30 天 TTL + 用户删除级联 + CRD 最小读权限 |

## 6. 前端设计（修订）
1. **现状**（评审 C 实证）：frontend-new 无任何流式代码，api 层是 fetch 薄封装 → Assistant 抽屉为全新建设（约占前端工作量 70%）；需 SSE 客户端、断线重连、确认对话框（含参数 diff 预览）、工具调用卡片。
2. **定位**：Training/Notebook 详情页已有日志/事件/指标抽屉——Copilot 叙事必须是"**跨源推理 + 引用**"，不重复展示原始数据；入口：全局抽屉 + 任务详情"诊断"按钮（预填上下文）。
3. **框架 WebUI Studio 复用评估**（评审 C 指出重复造轮子风险）：Studio 自带流式聊天/思考块/工具卡片/确认/会话管理。取舍结论：**v1 自研薄抽屉**（与产品视觉/鉴权/属主模型一致，Studio 的 `HandlerWithWebUI` 是裸 http.Handler，挂载不继承控制台会话鉴权）；**Studio 仅作管理员调试入口**（独立路由 + adminOnly 包裹 + chart 默认关闭 + 不进 Ingress）。
4. i18n：Agent 回复语言跟随用户 locale（系统提示词约束）；新词条同步 zh/en。
5. 错误体验（评审 C 补漏）：模型超时/限流/密钥失效、工具失败、SSE 重连、确认拒绝后的状态机——全部需要显式设计。

## 7. 构建/部署影响（修订）
| 项 | 状态/决议 |
|---|---|
| Go 1.25 + agentscope-go v2.0.9 | ✅ 已集成（go.mod/Dockerfile.console-new/CI 已同步） |
| 传递依赖归因 | 评审 A 更正：gcloud/grpc 等源自 arena→kserve 依赖链，agentscope 仅抬版本 |
| **旧栈闭环**（评审 A 指出） | `Dockerfile.console`（legacy）对已升 1.25 的 go.mod 仍用 golang:1.22 → 待处理：基础镜像升级或标记旧栈不再可构建（建议后者，随旧栈废弃） |
| chart | 新增 `agent.enabled`（默认 false）/`agent.model.*`/agent-credentials Secret；**内存上限 500Mi→1Gi**（agent.enabled 时；评审 C 实证不足）；`replicaCount=1` 单实例约束显式声明（内存会话 + SSE 亲和） |
| 灰度 | `agent.enabled` 总开关 + 用户/命名空间灰度名单 + 运行时 kill switch（框架 hotreload 可用） |
| 边缘/离线 | 模型不可达 → Assistant 入口自动隐藏，控制台功能不受影响（chart 明确支持 ACK@Edge，DashScope 不可达是现实场景） |
| 文档 | user-guide/deploy-guide 增 agent 章节（含数据出境声明、离线说明） |
| P0 边界（诚实声明） | 已完成 = 依赖可编译 + BuildChatModel scaffold；路由/RunRegistry/工具装配是 P1 工作量 |

## 8. 分阶段路线图（修订版，含退出标准）
| 阶段 | 内容 | 退出标准 |
|---|---|---|
| **P1**（~2-3 周） | 只读工具面（任务/Notebook/指标/日志/事件）+ **审计 + 预算上限** + Run/确认基础设施 + SSE Assistant 抽屉 + C2 故障问答 + C3（含 quota 数据源补齐）+ 内存会话 | 失败任务诊断带引用；每用户每日花费硬上限生效；审计可查；模型不可用时入口隐藏 |
| **P2** | 写入工具（Notebook 创建/停止/启动、训练提交/停止）+ HITL 确认单 + AgentSession CRD（含重启恢复）+ W1 重提闭环 | 全部写入经确认；确认单快照绑定过对抗测试；destructive 双重确认 |
| **P3** | BYOK + 管理端（审计/成本看板）+ W5-lite（持久化调度）+ 模型档案 | 用户可自带 key；管理员可见全平台成本 |
| **P4**（各自独立，不捆绑） | 只读 MCP Server（默认关闭+白名单）｜沙箱（六项强制全部落地才可发布） | 各自独立发布 |
| **P5** | W2 顾问、W3 评估、M3 插件 | 需求驱动 |
| ❌ 移出 | M1、M5、W4、自主循环 HPO | — |
| **安全前置** | §5.1 现存漏洞修复（✅ 已完成）+ 读工具属主校验清单 + AgentSession 属主模型 | **先于或平行于 P2 写入工具**——在带越权历史的 API 面上跑自主写入代理是风险乘法（评审 B 结论） |

## 9. 开放问题裁决（评审后）
1. **持久化**：P1 内存；P2 CRD 仅存元数据+压缩摘要；原始流水不进 etcd（未来需要走对象存储）。
2. **HITL 默认级别**：write 默认确认，不妥协；体验靠计划级批量确认与显式信任。
3. **默认模型**：qwen-plus + 降级链；深度诊断手动升档；重度用户 BYOK。
4. **MCP 范围**：只读起步 + 默认关闭 + 白名单 + 全审计；写入永不开放；消费外部 MCP 不做。
5. **Studio**：仅管理员 + 独立路由 + 自包鉴权 + 默认关闭。
6. **沙箱镜像**：平台维护版本化基础镜像为默认；用户镜像走管理员白名单。

## 10. 质量度量与回归（评审补漏）
- 线上埋点：会话/消息 DAU、工具调用错误率、HITL 批准/拒绝率、诊断点赞/点踩、自愈重提后任务成功率、每会话成本；
- 离线评测：框架 `replay`（录制带进 CI 零成本回放，防提示词/工具回归）+ `eval_harness`；
- 灰度：hotreload 配置热更 + 灰度名单。

## 11. 已验证事实（v2 更正版）
- 依赖集成：`go build ./...` / `go vet` / `go test` 全绿（实跑）；
- 框架 API（逐行核对 v2.0.9 源码）：`tool.NewFunctionTool(name, description, schema, fn)`；`agent.NewUnifiedAgent` + `agent.WithToolkit`（NewReActAgent 已废弃）；`model.NewDashScopeChatModel/NewOpenAIChatModel`（SecretStr 优先）、`model.NewFallbackChain`；`service.New(...).Handler()/HandlerWithWebUI`（裸 http.Handler，挂载需自包鉴权）；`workspace.NewK8sWorkspace`（kubectl shell-out；PodTTLSeconds=activeDeadlineSeconds）+ `workspace.NewKubectlGetTool/NewKubectlLogTool`（只读）；`mcp.NewServer`（**私有网关协议**，非标准 MCP）、`mcp.NewMCPToolkit`（stdio 客户端可用）；`schedule.NewInMemoryScheduler`（内存态）；`runtime.NewAgentPool`；`memory.CompressMessages`（截断）+ `agent.WithContextConfig`（摘要）；`middleware.CostTrackerMiddleware`（计价，**非**"spend cap 上限"——上限需自研）；`audit` 包；`replay` 包；
- 框架稳定性：runtime、service 等在 STABILITY.md 标记 Experimental → internal/agent 防腐层设计（§3）；
- 控制台挂点：`checkNamespaceAccess`、`isNamespaceAllowed`、`getUserNamespaces`、`auth.CheckAuth`、`k8s.TenantRegistry`、User CR 同族 CRD 模式（apis/data/v1 + chart crds）均核实存在；新后端**无**持久层、**无** quota 接口（P1 前置补齐）。

---

## 附录 A：v1→v2 变更记录
| v1 内容 | 评审 | v2 处置 |
|---|---|---|
| "SSE 断连即取消" | A | Run 服务端对象 + 可重连事件流（§3.1） |
| NewFunctionTool(schema,fn)、ReActAgent、OTLP 原生、memory/compress 包 | A | 全部更正（§11） |
| "MCP Server 供 IDE 操作平台" | A | 私有协议澄清；只读起步 + 适配工作量单列（§2/§8） |
| "框架预算/计价上限" | A/B | v2.0.9 无 spend cap → 自研三重上限（§5.2） |
| RBAC 兜底（对读路径） | B | 读路径真相澄清 + 工具层独立校验（§3.2） |
| HITL 依赖框架 | B | 自研确认单快照绑定（§3.3） |
| AgentSession 存共享 ns + 全量消息 | B | 系统 ns + 属主过滤 + 仅元数据/摘要（§4） |
| W4 头脑风暴有/路线图无 | C | 显式砍掉（§2） |
| C3 依赖"已有 quota" | C | 更正：不存在，列入 P1 前置（§2） |
| 默认 qwen-max | C | qwen-plus（§3.4） |
| 沙箱各强制项"可选配置" | B | 六项强制门禁（§5.2） |
| 无：数据出境/保留期/离线降级/质量度量/i18n/内存 500Mi/DISABLE_AUTH | B/C | 全部补入（§5.2/§6/§7/§10） |
