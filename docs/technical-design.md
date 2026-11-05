# 技术方案

- 状态：MVP 技术方案已实现，macOS arm64 已完成开发验收
- 验收记录：[开发验收报告](acceptance.md)
- 目标：用 MyGo 构建一个可开源、可分发的本地桌面应用
- 首个完整平台：macOS
- 设计原则：本地采集、结构化事实、确定性动作、可选的 AI 解释、P0 视觉与动效质量
- 文档性质：本文同时保留后续架构规划；“已实现”与“规划”以第 0 节和支持矩阵为准

## 0. 实现状态（2026-11-05）

| 能力 | 状态 | 说明 |
|---|---|---|
| macOS arm64 实时采集 | 已实现并验收 | CPU、内存、Swap、进程数、任务、Agent 和 Docker 发现 |
| 任务归因与规则证据 | 已实现并验收 | 基于进程树和签名；保守标记 reclaimable |
| 任务热力树、生命周期河流、爆炸半径 | 已实现并验收 | 使用 React/SVG/CSS，引入确定性演示 fixture |
| 系统热压力巡检与通知 | 已实现并验收 | macOS 公开 `NSProcessInfo.thermalState`；25 秒巡检、Serious 60 秒阈值、Critical 立即提示、同原因 15 分钟冷却 |
| 本地规则 AI | 已实现并验收 | 无模型也可用，不联网 |
| OpenAI-compatible provider | 已实现并验收 | 已用 DeepSeek `deepseek-chat` 验证真实远程端点；失败自动回退本地规则 |
| API Key 安全存储 | 已实现并验收 | 非敏感配置写入用户目录，Key 写入系统凭据库（macOS Keychain）；重启自动恢复 |
| Unix 安全回收 | 已实现并验收 | 单次令牌、PID + 启动时间、保护规则、仅 SIGTERM、逐 PID 回执 |
| Windows 终止 | 明确未实现 | 当前返回“MVP 默认拒绝执行” |
| Docker | 只读 | 已完成容器列表采集，停止/清理未实现 |
| SQLite 历史 | 未实现 | 当前趋势和审计为进程内状态 |
| 菜单栏/tray、自动静默清理 | 未实现 | 属于后续里程碑 |
| Linux/Windows 运行时 | 未验收 | 代码包含跨平台基础依赖，不等同于平台可用 |
| 签名、notarization、跨平台 CI | 未完成 | 当前 macOS 构建为 ad-hoc 签名 |

## 1. 技术选型

| 层 | 选择 | 原因 |
|---|---|---|
| 桌面框架 | MyGo `v0.7.4+` | 用户指定；Go 后端 + 系统 WebView，单二进制，无 Electron |
| 后端语言 | Go `1.27.3+` | 与 MyGo 一致；适合进程、网络、信号、跨平台系统编程 |
| 前端 | React `19` + TypeScript + Vite | 复杂数据交互和可视化生态成熟；MyGo 模板直接支持 Vite |
| 前端基础组件 | Radix UI primitives | 对话框、菜单、Tooltip 等可访问性可靠，不引入完整设计系统 |
| 图标 | Lucide React | 轻量、风格统一 |
| 实时图表 | React + 内联 SVG/CSS | MVP 的轻量趋势可直接渲染；uPlot 保留为后续长历史图表候选 |
| 层级布局 | React DOM + SVG | MVP 使用可访问的任务行和进程树；大规模聚合后再评估 `d3-hierarchy` |
| 动画 | CSS transitions + Web Animations API | 足够覆盖桌面 UI；体积小、可取消、容易遵守 reduced-motion，无需 GSAP |
| UI 样式 | 原生 CSS + design tokens | 保持可控、轻量，方便主题和桌面适配 |
| 状态管理 | React state + 定时快照轮询 | 单窗口 MVP 保持简单；事件流规模扩大后再引入小型 store |
| 进程/系统指标 | `gopsutil/v4` + 平台适配器 | 跨平台基础能力可靠；增强字段由各平台自行实现 |
| 系统热压力 | macOS `NSProcessInfo.thermalState` + `purego/objc` | 通过 Objective-C runtime 读取公开压力等级，**生产构建不依赖 cgo**；不采集虚构温度，不需要 root、SMC 或 `powermetrics` |
| Docker | 直接使用 Docker Engine HTTP API | 避免完整 Docker SDK；**MVP 仅实现容器 list，stop 属于 roadmap** |
| 存储 | 进程内快照（MVP） / MyGo SQLite（规划） | MVP 不持久化；SQLite 历史、保留策略和重启恢复尚未实现 |
| 模型接入 | OpenAI-compatible chat completions over `net/http` | 兼容 OpenAI、Ollama、LM Studio、vLLM 等；不绑定单一厂商 |
| 测试 | Go `testing` + Vitest + Testing Library | 当前自动化覆盖后端规则和 React 交互；Playwright/视觉回归为规划项 |
| 打包 | `mygo build`（当前）+ GitHub Actions（规划） | 当前已产出 macOS arm64 `.app/.dmg`；跨平台 CI、签名和 notarization 未完成 |
| License | Apache-2.0 | 明确专利授权，适合长期开源项目 |

### 为什么不用

- **Electron**：内存开销与产品价值冲突；MyGo 已经满足 Web UI + Go 系统能力。
- **Tauri**：同样可行，但用户指定 MyGo，且 MyGo 直接提供 Go binding/event 与桌面 API。
- **Native UI**：进程树、实时曲线、文本问答更适合 Web 渲染；MyGo 的 WebView 模式仍保持单二进制。
- **完整 Docker SDK / Kubernetes SDK**：MVP 只需要本地 Docker 容器，不应引入大量依赖和权限面。
- **向量数据库 / RAG**：当前问题是实时结构化数据归因，不需要语义检索。
- **Agent 自主执行工具调用**：与安全目标冲突，模型不直接接触执行器。
- **GSAP / Lottie / 3D 动效运行时**：MVP 的动效是状态解释和空间连续性，不需要大型编排运行时；CSS/WAAPI 足够且更省常驻资源。
- **远程字体/CDN 图标**：违背默认离线原则，并会引入加载闪烁和视觉回归不确定性。

## 2. 总体架构

> 下图是目标架构，不是 MVP 全部已实现组件。当前 MVP 使用进程内 Snapshot，不与 SQLite 交互；前端每 4 秒调用 `Refresh` 轮询，尚未接入 MyGo event 推送；Docker 只有只读采集。

```text
┌─────────────────────────────────────────────────────────────┐
│ MyGo Window (WebView)                                       │
│ React UI: Overview / Tasks / Reclaim / Ask / Settings        │
└───────────────────────┬─────────────────────────────────────┘
                        │ typed bindings + events
┌───────────────────────▼─────────────────────────────────────┐
│ Go Application Service                                      │
│                                                            │
│  Collector Manager                                         │
│    ├─ System Collector                                     │
│    ├─ Process Collector                                    │
│    ├─ Port Collector                                       │
│    ├─ Docker Collector                                     │
│    └─ Platform Adapter                                     │
│             │                                              │
│             ▼                                              │
│  Workload Resolver                                          │
│    ├─ process tree / group / session                        │
│    ├─ signature rules                                      │
│    ├─ cwd / port / container labels                        │
│    └─ user corrections                                     │
│             │                                              │
│             ▼                                              │
│  Rule Engine ──────► Evidence + Risk + Reclaim Candidate    │
│             │                                              │
│             ▼                                              │
│  Snapshot Store (SQLite)                                    │
│             │                                              │
│      ┌──────┴────────┐                                     │
│      ▼               ▼                                     │
│  TaskService     AI Assistant                               │
│      │               │                                     │
│      ▼               ▼                                     │
│  Action Policy   Query Compiler / LLM Provider              │
│      │               │                                     │
│      ▼               └─ explanation + proposed plan         │
│  Safe Executor                                               │
└─────────────────────────────────────────────────────────────┘
```

### 数据流

1. Collector Manager 按自适应周期采样。
2. Workload Resolver 将进程映射为任务，并维护稳定的 workload ID。
3. Rule Engine 读取滑动窗口历史，生成风险和证据。
4. Snapshot Store 保存聚合样本和任务元数据。
5. 前端通过 MyGo event 接收快照，通过 binding 请求详情/动作。
6. AI Assistant 只接收脱敏后的结构化上下文，不直接访问采集器或执行器。

## 3. 建议目录结构

```text
.
├── main.go                       # MyGo app bootstrap / bindings
├── mygo.config.ts
├── package.json
├── src/
│   ├── main.tsx
│   ├── mygo.ts                   # generated typed client
│   ├── app/
│   │   ├── App.tsx
│   │   ├── store.ts
│   │   └── routes.ts
│   ├── components/
│   │   ├── Shell.tsx
│   │   ├── MetricStrip.tsx
│   │   ├── WorkloadTable.tsx
│   │   ├── ProcessTree.tsx
│   │   ├── ReclaimCard.tsx
│   │   ├── AssistantPanel.tsx
│   │   └── ActionPreview.tsx
│   ├── features/
│   │   ├── overview/
│   │   ├── workloads/
│   │   ├── reclaim/
│   │   ├── assistant/
│   │   └── settings/
│   ├── styles/
│   └── test/
├── internal/
│   ├── model/                    # shared IPC/domain structs
│   ├── collector/
│   │   ├── manager.go
│   │   ├── system.go
│   │   ├── process.go
│   │   ├── ports.go
│   │   ├── docker.go
│   │   └── platform/
│   │       ├── darwin/
│   │       ├── linux/
│   │       └── windows/
│   ├── workload/
│   │   ├── resolver.go
│   │   ├── signatures.go
│   │   ├── identity.go
│   │   └── merge_rules.go
│   ├── rules/
│   │   ├── engine.go
│   │   ├── duplicate.go
│   │   ├── fanout.go
│   │   ├── memory.go
│   │   ├── idle.go
│   │   └── orphan.go
│   ├── action/
│   │   ├── policy.go
│   │   ├── preview.go
│   │   ├── executor.go
│   │   └── platform.go
│   ├── ai/
│   │   ├── context.go
│   │   ├── redact.go
│   │   ├── prompt.go
│   │   └── provider.go
│   ├── store/
│   │   ├── sqlite.go
│   │   ├── migrations.go
│   │   └── queries.go
│   ├── audit/
│   └── service/
│       ├── task_service.go
│       └── events.go
├── docs/
└── build/                        # generated packages, gitignored
```

## 4. 核心数据模型

以下结构同时作为 Go 内部模型和 MyGo IPC schema 的基础。实际实现会避免复杂联合类型，保证生成 TypeScript 类型稳定。

```go
type Snapshot struct {
    Revision    uint64        `json:"revision"`
    GeneratedAt time.Time     `json:"generatedAt"`
    System      SystemMetrics `json:"system"`
    Workloads   []Workload    `json:"workloads"`
    Alerts      []Alert       `json:"alerts"`
}

type SystemMetrics struct {
    CPUPercent    float64 `json:"cpuPercent"`
    MemoryUsed    uint64  `json:"memoryUsed"`
    MemoryTotal   uint64  `json:"memoryTotal"`
    SwapUsed      uint64  `json:"swapUsed"`
    SwapTotal     uint64  `json:"swapTotal"`
    DiskReadBPS   uint64  `json:"diskReadBps"`
    DiskWriteBPS  uint64  `json:"diskWriteBps"`
    NetRxBPS      uint64  `json:"netRxBps"`
    NetTxBPS      uint64  `json:"netTxBps"`
    ProcessCount  int     `json:"processCount"`
    WorkloadCount int     `json:"workloadCount"`
}

type Workload struct {
    ID          string          `json:"id"`
    Kind        string          `json:"kind"` // agent, ide, devserver, container, experiment, service, unknown
    Name        string          `json:"name"`
    Source      string          `json:"source"`
    Workspace   string          `json:"workspace,omitempty"`
    RootPIDs    []int32         `json:"rootPids"`
    ProcessIDs  []int32         `json:"processIds"`
    StartedAt   time.Time       `json:"startedAt"`
    CPUPercent  float64         `json:"cpuPercent"`
    RSSBytes    uint64          `json:"rssBytes"`
    ProcessCount int            `json:"processCount"`
    Ports       []int           `json:"ports"`
    Risk        string          `json:"risk"` // healthy, attention, reclaim
    Confidence  float64         `json:"confidence"`
    Evidence    []Evidence      `json:"evidence"`
    Actions     []ActionSummary `json:"actions"`
}
```

### 身份与稳定性

工作负载 ID 不能只用 PID，因为 PID 会复用；也不能只用命令行，因为同一命令会启动多个会话。

```text
workload_id = sha256(kind | workspace | root_start_time | normalized_identity)
```

- 进程动作始终使用 `pid + process_start_time` 双重校验。
- 用户手动合并/拆分产生本地持久化规则。
- 重启后新建任务；历史记录按任务签名可聚合，但不把两次真实运行混成一次。

## 5. 采集层设计

### Collector 接口

```go
type Collector interface {
    Name() string
    Available() bool
    Collect(ctx context.Context) (PartialSample, error)
}
```

每个 collector 的失败必须局部降级。Docker 不可用不能导致系统概览失败。

### 5.1 系统指标

使用 `gopsutil/v4` 获取：

- CPU 使用率；
- 物理内存、Swap；
- 磁盘 I/O；
- 网络 I/O；
- 启动时间、负载。

平台特定增强：

- macOS：内存压力/swap 详情，必要时补充 `sysctl`；
- Linux：`/proc/meminfo`、`/proc/loadavg`、cgroup 信息；
- Windows：性能计数器 API。

GPU 和能耗不在 MVP 中伪装支持。后续分别实现：

- Linux：NVIDIA/AMD sysfs 或厂商库；
- macOS：签名后的原生辅助工具或明确要求权限的采样器；
- Windows：PDH GPU Engine counters。

### 5.2 进程采集

基础字段：

- PID、PPID、PGID、Session、UID；
- 可执行文件、命令行；
- 启动时间；
- CPU 百分比；
- RSS/VSZ；
- 状态；
- 父进程链；
- 可选 cwd、tty、端口。

macOS/Linux 通过 gopsutil 起始，增强字段按平台读取。Windows 使用系统进程 API 和 Job/进程组能力。

采样策略：

- 正常：2 秒；
- 有活跃资源压力：1 秒；
- 窗口隐藏：5 秒；
- 只对展开的详情页读取 cwd、完整端口等昂贵字段；
- 进程命令行按需刷新，避免每 tick 读取全部信息。

### 5.3 端口采集

目标：不是做网络扫描，而是把已监听端口映射到任务。

- 采集 LISTEN TCP 连接；
- 只记录本地绑定与进程映射；
- 不读取连接内容；
- 端口信息作为任务证据，而不是单独的安全告警。

### 5.4 Docker 采集

使用 Docker Engine HTTP API：

- `GET /_ping` 判断可用性；
- `GET /containers/json?all=0` 获取运行容器和 Compose labels；
- `GET /containers/{id}/stats?stream=false` 获取 CPU/RSS；
- `POST /containers/{id}/stop?t=10` 执行停止。

优先读取：

- Compose project、service；
- container name；
- image；
- 端口；
- 启动时间；
- CPU/RSS；
- 退出状态。

Docker socket 不可用、权限不足或 daemon 未启动时，所有相关 UI 显示明确不可用。

### 5.5 系统热压力

macOS 使用 `NSProcessInfo.thermalState` 的公开枚举映射。读取层通过 `purego` 加载 Foundation 并调用 Objective-C runtime，不依赖 cgo，确保 `CGO_ENABLED=0` 的 MyGo 生产构建与普通 Go 测试使用同一能力：

```text
nominal → fair → serious → critical
```

约束：

- 该 API 返回离散压力等级，不返回摄氏度；界面不展示 `°C` / `℃`，不根据 CPU 或进程负载反推温度；
- Linux/Windows 当前返回 `unavailable`，保留 CPU、内存和任务证据，不能伪造平台能力；
- 不使用 SMC、root 权限、私有 IOKit 调用或 `powermetrics`；
- 主窗口就绪后启动 25 秒后台巡检，退出时取消；
- `serious` 必须连续 60 秒才产生通知，`critical` 立即通知；`nominal` 连续 2 分钟才结束一次压力事件；
- 同一归因任务 15 分钟通知冷却，不同归因可再次提示；通知权限拒绝后停止重试；
- 通知文本和 AI 上下文均删除 PID、路径、命令行和密钥形态内容；
- 巡检与热压力解释均为只读，不生成动作令牌、不发送信号。

热压力归因只从当前快照中选择证据更强的任务，展示持续时间、置信度、证据和建议。即使归因任务可回收，也必须进入既有爆炸半径预览与用户确认流程。

## 6. 工作负载归因

### 6.1 归因流程

```text
process snapshot
  → build parent graph
  → identify roots
  → attach descendants
  → merge explicit identities
  → attach container metadata
  → attach ports/cwd
  → score confidence
  → emit Workload[]
```

### 6.2 根任务识别

按优先级：

1. 用户显式标记的任务；
2. 由本应用启动并带有标签的进程；
3. 已知 Agent 可执行文件；
4. IDE/终端/容器运行时的直接子任务；
5. 开发服务器/解释器/实验进程；
6. 未归类进程。

### 6.3 合并规则

合并为一个任务的条件示例：

- 同一 Agent 根进程的全部后代；
- 同一终端会话下由同一 root 产生的进程组；
- 同一 Compose project 的容器；
- 同一工作目录且命令行一致的重复 MCP server；
- 用户保存的 merge rule。

以下情况默认**不合并**：

- 两个独立终端中的同一命令；
- 两个不同 workspace 的 Agent；
- 系统服务的后代与用户任务；
- 无法确认归属的进程。

### 6.4 置信度

- `0.95+`：显式标签、容器 metadata、确定根进程；
- `0.8–0.95`：强签名 + 父进程链 + workspace；
- `0.6–0.8`：只有命令行/时长等启发式；
- `<0.6`：放入“未归类”，不生成高风险建议。

置信度必须显示，用户可手动纠正。

### 6.5 Agent 识别签名

签名规则使用可配置结构：

```go
type Signature struct {
    Matchers []Matcher
    Kind     string
    Name     string
    RootOnly bool
}
```

初始支持：

- `claude`
- `codex`
- `gemini`
- `opencode`
- `aider`
- `goose`
- MCP server 常见启动器：`npx`, `npm exec`, `uvx`, `python`, `node`, `docker`
- `node_repl`
- Playwright/browser helpers
- Jupyter kernel
- Vite/Next/Webpack/Vite/uvicorn/gunicorn 等开发服务器
- Docker/Compose 容器

签名只产生“候选任务”，最终归属仍由进程树和上下文确认。

## 7. 规则引擎

规则接口：

```go
type Rule interface {
    ID() string
    Evaluate(history History, current Workload) []Finding
}
```

Finding 包含：

```go
type Finding struct {
    RuleID      string
    Severity    string
    Confidence  float64
    Title       string
    Explanation string
    Evidence    []Evidence
}
```

### 默认规则

| 规则 | 默认条件 | 建议 |
|---|---|---|
| duplicate-mcp | 同一根下 ≥3 个近似命令行，持续 ≥5 分钟，合计 RSS ≥300 MB | 查看重复子进程，优先停止最旧组 |
| fanout-burst | 60 秒内后代增加 ≥20，或总数 ≥50 | 提醒检查 Agent 是否卡在工具循环 |
| helper-pile | 同父进程下 ≥10 个相似 helper | 标记 MCP/浏览器 helper 堆积 |
| idle-agent | Agent 运行 ≥90 分钟，10 分钟平均 CPU <0.5% | 标记疑似闲置，不自动关闭 |
| memory-growth | 10 分钟 RSS 增长 ≥500 MB 且 ≥15% | 检查泄漏或缓存失控 |
| orphan-known-task | 已知用户任务父进程消失 ≥30 分钟，原父子关系有历史记录 | 提示孤立任务 |
| high-sustained-cpu | CPU ≥80% 持续 ≥2 分钟 | 展示为压力来源，不一定可回收 |
| unbounded-rss | RSS ≥20% 物理内存 | 高风险关注 |

规则触发需要滑动窗口，不能只看单次采样，避免短时编译/推理误报。

### 误报控制

- 新任务前 2 分钟不进入 reclaim 等级；
- “可回收”需要至少两条独立证据或一条强证据；
- 用户保护/忽略优先于所有规则；
- 规则阈值可配置；
- 规则解释必须能展开到原始指标。

## 8. 动作执行与安全模型

> **MVP 实现状态**：当前只实现具备证据的 MCP 后代替换流程。实时路径使用 2 分钟单次令牌、PID + 启动时间校验、硬编码保护集合和 SIGTERM；不提供自动 SIGKILL、Docker stop、用户自定义保护名单或跨重启审计。Windows 终止路径明确拒绝执行。下文的强杀、容器停止和用户保护名单属于后续规划。

### 两层模型

```text
ActionPlan（可预览、不可执行）
    → 用户确认
    → ConfirmationToken（绑定 snapshot revision + workload + pid start time）
    → SafeExecutor
```

### 动作类型

- `StopWorkload`：优雅停止整个任务；
- `StopProcessSubtree`：只停止指定后代；
- `StopContainer`：通过 Docker API 停止；
- `ProtectWorkload` / `UnprotectWorkload`；
- `IgnoreRule`；
- 后续：`Freeze` 和 `Resume`。

### 预览内容

- 要停止的 PID 列表及每个进程的名称；
- 容器 ID/名称；
- 受保护项和阻断原因；
- 预计回收 RSS/CPU；
- 预计是否会关闭终端/IDE；
- 选择的通信方式；
- 超时和后续可执行操作。

### 受保护集合

- 当前进程、父进程和所有祖先；
- `launchd`/`systemd`/`WindowServer`/内核等关键系统进程；
- 系统服务和 Docker daemon；
- 当前前台 IDE/终端；
- 用户保护名单；
- 任何无法确认身份或启动时间的进程。

### 执行策略

Unix：

1. 校验 `pid + start_time`；
2. 先向叶节点/子进程组发送 `SIGTERM`；
3. 等待可配置超时（默认 5 秒）；
4. 仅对仍存活的进程提示“强制结束”，需要第二次确认；
5. 最后校验结果并写审计日志。

Windows：

- 优先向可创建的进程组发送 Ctrl-Break/正常关闭；
- 对无控制台进程使用 Win32 终止 API；
- 强杀必须二次确认；
- 所有 PID 同样绑定启动时间。

Docker：

- 只通过 Engine API 停止指定容器；
- 不拼接 shell 命令；
- 失败时保留原状态并显示 daemon 错误。

## 9. AI 助手设计

> **MVP 实现状态**：本地规则和 OpenAI-compatible provider 均已实现；默认不联网。`Explain` 只返回解释，不生成动作令牌或 shell 命令。自然语言建议与确定性 ActionPlan 保持分离，用户必须回到界面预览和确认。真实 DeepSeek `deepseek-chat` 端点已完成问答验收，模型失败时回退本地规则。

### 定位

AI 是**解释和规划层**，不是采集层或执行层。

### 处理流程

```text
用户问题
  → 本地意图分类（诊断/检索/解释/计划）
  → 构造最小结构化上下文
  → 脱敏
  → 可选本地/远程模型
  → 校验输出 schema
  → 展示事实/推断/建议/不确定性
  → 用户点击动作
  → ActionPolicy + 确认
```

### 本地能力

即使没有模型，也要支持：

- “什么占用最多 CPU？”
- “什么占用最多内存？”
- “有哪些 Agent 超过 2 小时？”
- “有哪些重复 MCP？”
- “有哪些疑似闲置任务？”
- “哪个任务占用了 5173 端口？”
- 将自然语言问题映射为内置筛选 DSL。

本地查询 DSL 示例：

```text
kind=agent AND age>2h AND cpu<1
risk=reclaim AND rss>500MB
port=5173
```

### 模型适配器

```go
type Provider interface {
    Name() string
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
```

MVP 实现 OpenAI-compatible `/v1/chat/completions`，配置：

- base URL；
- model；
- API key（可选）；
- temperature（默认低）；
- 超时；
- 是否允许远程数据。

MVP 的主配置路径是应用内 AI 设置：Provider、Base URL、Model 和远程同意开关写入用户目录；API Key 通过 `mygo.Secrets` 写入系统凭据库（macOS 为 Keychain），不会回传前端，也不会写入普通配置、日志或诊断请求。环境变量仍作为开发和 CI 的覆盖入口。

支持 Ollama/LM Studio 意味着用户可以在完全本地的情况下使用自然语言。

### Prompt 与防注入

- 进程名和命令行视为不可信数据，使用明确的数据分隔符；
- 系统提示明确禁止把进程输出当作指令；
- 模型不拥有工具调用权限；
- 模型返回 JSON 计划后由 Go 侧 schema 校验；
- 不接受模型自由生成的 shell 命令；
- 只允许预定义动作 ID；
- 失败时降级到本地答案。

## 10. MyGo Binding 与事件

### Go 绑定服务

```go
type TaskService struct{}

func (TaskService) Snapshot() (model.Snapshot, error)
func (TaskService) WorkloadDetails(id string) (model.WorkloadDetails, error)
func (TaskService) PreviewStop(req model.StopRequest) (model.ActionPlan, error)
func (TaskService) ExecuteStop(req model.ConfirmStopRequest) (model.ActionResult, error)
func (TaskService) ProtectWorkload(id string, protected bool) error
func (TaskService) Ask(req model.AskRequest) (model.AskResponse, error)
func (TaskService) GetAISettings() model.AISettings
func (TaskService) SaveAISettings(input model.AIProviderInput) (model.AIStatus, error)
func (TaskService) TestAIConnection(input model.AIProviderInput) (model.AITestResult, error)
func (TaskService) ClearAISettings() (model.AIStatus, error)
```

### 事件

```go
var SnapshotEvent = mygo.NewEvent[model.Snapshot]("snapshot")
var AlertEvent = mygo.NewEvent[model.Alert]("alert")
var ActionProgressEvent = mygo.NewEvent[model.ActionProgress]("actionProgress")
```

启动时：

1. 初始化 SQLite；
2. 启动 collector manager；
3. 绑定 services；
4. 创建窗口；
5. 按窗口生命周期发送事件。

前端初始调用 `Snapshot()`，随后订阅事件，避免事件到达早于页面加载时丢失数据。

## 11. 前端设计

### 页面结构

- `AppShell`：侧边导航、全局搜索、状态指示；
- `OverviewPage`：System metrics、InsightSummary、TopWorkloads、TrendChart；
- `WorkloadsPage`：筛选、排序、虚拟列表、详情抽屉；
- `ReclaimPage`：按风险/可回收收益分组；
- `AssistantPage`：对话、快捷问题、引用证据、建议动作；
- `SettingsPage`：阈值、保护规则、模型、隐私。

### 数据订阅

- `useSyncExternalStore` 或轻量 store 保存最新 Snapshot；
- `revision` 单调递增，防止乱序事件覆盖新状态；
- 事件频率高时只更新对应 selector，避免整页重渲染；
- 任务列表超过 100 行使用虚拟化；
- 图表只保留最近 15 分钟高频数据，历史由 SQLite 查询；
- 高频采样先批量归一化，再按 selector 通知组件，避免整页重渲染。

### 视觉实现

完整规范见 [视觉、动效与交互设计规范](design-system.md)。工程实现必须遵守以下约束：

- Signal Deck 设计语言：石墨黑/瓷白、信号青、资源蓝；红/橙只出现在风险与阻断语义中；
- 暗色优先、完整亮色主题，默认跟随系统；
- 所有颜色、间距、字号、圆角、阴影和动效通过 design tokens 使用，组件内禁止散落原始 hex；
- 字体随应用本地打包：Noto Sans SC 子集用于 UI，JetBrains Mono 用于数值和命令行；禁止远程字体；
- 图标统一使用 Lucide，禁止 emoji 作为结构图标；
- 任务热力树默认最多 2 层，下钻到第 3 层；面积表示 RSS，CPU 用同色相明度表示，风险用图标/文字/纹理；
- 任务热力树少于 200 节点用 SVG，200–1000 节点用 Canvas；超过 1000 节点先聚合或过滤；
- 生命周期河流使用 uPlot/Canvas 增量绘制，支持暂停、缩放和文本事件表；
- 爆炸半径预览复用任务树节点状态，但必须独立展示“将停止 / 受保护 / 阻断 / 关联容器”；
- 任务行到详情抽屉使用 220ms 空间连续转场；不支持共享元素时降级为淡入，不阻断交互；
- 所有 skeleton 预留真实内容尺寸，避免数据到达导致布局偏移。

### 动效实现

- 统一 motion tokens：press 80ms、hover 120ms、state 160ms、panel 220ms、structure 300ms；
- 优先动画 `transform` 和 `opacity`；必要时允许颜色与 SVG 属性过渡；
- 不通过动画改变可用状态；业务状态先提交，再触发视觉反馈；
- 动画可被下一次交互取消，不能依赖 `animationend` / `transitionend`；
- 常规 1Hz 采样不逐节点播放面积/高度动画，避免持续重排；
- 每个视图最多 1–2 个关键动效，空闲状态不得有循环动画；
- `prefers-reduced-motion: reduce` 下直接呈现最终状态，保留完整信息和操作；
- 动效执行期间不得阻塞停止、确认、取消或 Tab 焦点。

### 可访问性

- 正文对比度 ≥4.5:1，非文本交互元素 ≥3:1；
- 所有危险动作支持键盘焦点和明确文案；
- 颜色不是唯一风险信号；
- 每个图表提供表格或摘要替代；
- 进程树支持键盘展开/折叠、进入详情与返回；
- 1Hz 状态更新使用一个受控的 `role="status"`，不逐 tick 播报全部指标；
- 支持 200% 缩放、系统大字体和 reduced-motion。

## 12. SQLite 存储

### 表结构草案

```sql
CREATE TABLE system_samples (
  ts INTEGER NOT NULL,
  cpu_percent REAL NOT NULL,
  memory_used INTEGER NOT NULL,
  swap_used INTEGER NOT NULL,
  disk_read_bps INTEGER NOT NULL,
  disk_write_bps INTEGER NOT NULL,
  net_rx_bps INTEGER NOT NULL,
  net_tx_bps INTEGER NOT NULL,
  PRIMARY KEY (ts)
);

CREATE TABLE workload_samples (
  ts INTEGER NOT NULL,
  workload_id TEXT NOT NULL,
  cpu_percent REAL NOT NULL,
  rss_bytes INTEGER NOT NULL,
  process_count INTEGER NOT NULL,
  risk TEXT NOT NULL,
  PRIMARY KEY (ts, workload_id)
);

CREATE TABLE workload_meta (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  identity_json TEXT NOT NULL,
  first_seen INTEGER NOT NULL,
  last_seen INTEGER NOT NULL
);

CREATE TABLE audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts INTEGER NOT NULL,
  action TEXT NOT NULL,
  workload_id TEXT,
  pids_json TEXT,
  result TEXT NOT NULL,
  detail TEXT
);

CREATE TABLE settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
```

实现细节：

- SQLite plugin 的 Go API，WAL 模式；
- 高频趋势只保留在内存中供 UI 使用；系统指标约每 10 秒聚合落盘，工作负载指标约每 60 秒聚合落盘；
- 进程树和端口拓扑采用事件驱动写入，不按采样 tick 重复保存；
- 7 天保留策略，启动时和每日后台清理；
- 只保存命令行摘要/哈希，默认不持久化完整 env；
- 数据库损坏时允许重建，并保留导出诊断。

## 13. 跨平台策略

### 当前验收矩阵

| 能力 | macOS arm64 | Linux | Windows |
|---|---|---|---|
| 实时 CPU/内存/进程/任务归因 | 已实现并验收 | 未验收 | 未验收 |
| 本地规则 AI | 已实现并验收 | 未验收 | 未验收 |
| Docker 容器发现 | 已实现并验收（只读） | 未验收 | 未验收 |
| DeepSeek / OpenAI-compatible 远程模型 | 已实现并验收 | 未验收 | 未验收 |
| Keychain 保存与重启恢复 | 已实现并验收 | 未验收 | 未验收 |
| SIGTERM 安全回收 | 已实现并验收 | 未验收 | 明确拒绝 |
| 历史、tray、自动清理 | 未实现 | 未实现 | 未实现 |

### 目标抽象边界

```go
type Platform interface {
    CollectProcesses(ctx context.Context) ([]Process, error)
    CollectPorts(ctx context.Context) ([]PortBinding, error)
    StopProcesses(ctx context.Context, targets []ProcessTarget, force bool) error
    Capabilities() Capabilities
}
```

### 支持矩阵

### 目标能力矩阵（规划）

| 能力 | macOS | Linux | Windows |
|---|---|---|---|
| CPU/内存/磁盘/网络 | 完整 | 完整 | 完整 |
| 进程树 | 完整 | 完整 | 完整 |
| 工作目录 | 权限允许时 | 权限允许时 | 权限允许时 |
| 端口映射 | 完整 | 完整 | 完整 |
| Agent 签名 | 完整 | 完整 | Beta |
| Docker | 完整 | 完整 | 完整 |
| 优雅停止 | 完整 | 完整 | 平台适配 |
| GPU | 后续 | NVIDIA/AMD 后续 | 后续 |
| 菜单栏/tray | 完整 | 有限 | 有限 |

不同平台不可用的数据必须显示为空状态和能力提示，不能伪造。该表是目标形态，不是当前验收结论。

## 14. 安全与权限

MVP 已实现：

- 不请求 root 权限运行主进程；
- Docker socket 访问失败时独立降级；
- API key 不返回给前端，应用内配置写入系统凭据库，环境变量作为开发覆盖入口；
- 远程模型端点必须显式开启远程同意，或设置 `AITM_AI_ALLOW_REMOTE=true`；
- 模型上下文不包含 PID、完整命令行或环境变量，只发送聚合任务摘要和当前诊断问题；
- 模型没有工具调用、动作令牌或 shell 权限；
- 动作只接受预览生成的短时单次令牌；
- 审计记录只保留动作所需摘要，且仅存在于当前应用会话；
- WebView 不加载任意远程页面。

后续规划与当前限制：

- 尚未实现用户自定义保护名单及其持久化；
- 系统凭据库恢复可能触发 macOS 授权提示，因此恢复在后台执行，超时后退回本地规则；
- 尚未实现跨重启审计和命令行动态脱敏管线；
- Linux/Windows 尚未进行权限边界验收；
- 不为外部 Agent 开放可写接口。

## 15. 性能设计

- 自适应采样，窗口不可见时降频；
- 进程详情按需获取；
- 端口和 cwd 不每 tick 扫描全量；
- 规则计算在内存中完成；
- UI 使用增量更新和 selector；
- 历史记录批量写入；
- 图表仅渲染可见时间范围；
- React 更新按帧合并，采样事件不直接触发整页 rerender；
- 任务列表虚拟化阈值 100 行；
- 树图按节点数切换 SVG/Canvas，并限制可见节点；
- 字体子集化并随包加载，避免首屏字体闪烁；
- 目标：常驻 CPU <1.5%，UI 内存稳定 <250MB RSS，7 天数据库 <200MB；
- 采样更新期间 P95 帧时间 <16.7ms，无 >50ms 长任务；
- CLS <0.05；
- 空闲持续动画为 0。

### 性能验收方式（规划）

本次验收只完成真实采集耗时观察和视觉目检，未完成正式 60fps/P95/CLS 门禁。以下为后续性能回归方案：

- Playwright 使用固定 fixture 和固定字体运行暗色/亮色基准场景；
- 记录 30 秒 1Hz 更新期间的帧时间和长任务；
- 截图对比任务树、生命周期河流、详情抽屉和爆炸半径预览；
- 在 `prefers-reduced-motion` 下重复关键交互；
- 手动在 MacBook Air M1、1440×900、100% 缩放下抽测；
- 性能失败时先减少 DOM、Canvas 重绘和 selector 范围，不通过降低信息完整性“优化”。

## 16. 测试策略

### Go 单元测试

- 进程树构造与归因；
- identity 与 PID 复用；
- 每条规则的边界条件；
- 保护名单和 action policy；
- 命令脱敏；
- 热压力状态机、冷却、通知脱敏与权限拒绝边界；
- SQLite migration/retention；
- AI 输出 schema 校验。

### 集成测试

- 创建真实父子进程树，验证停止顺序；
- 验证受保护父 Shell 不会被误杀；
- 用 mock Docker Unix socket 测试 list/stats/stop；
- 无权限读取 cwd 时降级；
- 采样期间进程退出/重启。

### 前端测试

- 快照渲染；
- 事件乱序；
- 空状态/错误状态；
- 详情抽屉和危险操作确认；
- AI 事实/推断/建议的展示；
- 热压力卡可用/不可用、证据、归因跳转与回收后恢复；
- 键盘导航；
- 任务热力树键盘下钻与表格降级；
- 生命周期河流暂停、缩放和事件表；
- 爆炸半径预览的“将停止 / 受保护”语义；
- reduced-motion 下信息完整且操作可用；
- 暗色/亮色视觉回归；
- 1024×720 与 1440×900 布局快照。

### 视觉与性能回归

- Playwright 关闭非确定性动画或使用固定时间点截图；
- 用 fixture 固定任务名、数值宽度和字体，避免像素差异噪声；
- 对 design tokens 做对比度测试；
- 检查 1Hz 更新下无布局跳动和自动滚动；
- 检查空闲状态没有持续动画；
- 检查 TTI 期间无骨架到内容的明显尺寸变化。

### Golden fixtures

不同平台、不同 locale 的进程输出需要固定测试数据，确保不会因为系统语言或进程名含空格而解析错误。

## 17. 打包与分发

### 开源仓库

- 仓库 license：Apache-2.0；
- `README` 说明隐私、安全模型和支持矩阵；
- 不要求用户安装 Node/Go 才能运行发行包；
- 源码构建使用 Bun + Go；
- 发布产物包含 checksum 和 changelog。

### 构建

- `mygo dev`：本地开发；
- `mygo generate`：生成 typed bindings；
- `mygo build`：产出平台包；
- GitHub Actions 矩阵：macOS arm64/x64、Linux x64/arm64、Windows x64；
- macOS 后续加入签名和 notarization；
- Linux 提供 deb/AppImage 或 MyGo 支持的打包格式；
- Windows 后续提供签名安装包。

### 更新

- 首版先提供手动下载/GitHub Release；
- 后续接入 MyGo updater plugin，使用签名更新；
- 更新包必须校验签名和 sha256。

## 18. 实施顺序

### M0：风险验证 Spike `[MVP 完成；量化门禁待补]`

先验证最容易让产品失败的两件事，不投入完整 UI，但同步做一个低保真交互探针，避免把视觉问题拖到最后：

- 用固定进程树夹具和本机真实快照实现最小 workload resolver；
- 证明能区分 Agent 根、MCP 后代、独立终端任务和 Docker 容器；
- 用真实子进程树验证 `PID + start_time` 校验和保护名单；
- 建立 20 组归因 golden fixtures 与停止动作集成测试；
- 用低保真原型验证用户能否理解任务热力树与爆炸半径预览。

验收：归因正确率 ≥90%，测试停止动作零误杀，明确哪些能力在 macOS 上受权限限制；用户能说出热力树和爆炸半径各自回答的问题。未通过则不进入完整 UI 开发。

### M1：真实数据原型与视觉基线 `[已完成]`

视觉不是最后一个阶段；本阶段就建立可持续演进的 UI 基础，并在合成 fixture 上实现首个可传播演示。

- 初始化 MyGo + React；
- 系统/进程采集；
- Signal Deck design tokens、暗色/亮色主题和 AppShell；
- 概览页、任务页和事件流；
- 任务热力树的合成数据版本；
- skeleton、空状态、权限不足和容器不可用状态；
- 本地字体与图标打包；
- 单元测试和首批视觉回归。

验收：在 macOS 上看到真实 CPU/内存/进程树；刷新不阻塞；1024×720 和 1440×900 均无关键内容截断；1Hz 更新时无布局跳跃；热力树可由键盘访问并提供表格降级。

### M2：归因与洞察 `[MVP 完成；持久历史待补]`

- Agent/IDE/开发服务器识别；
- 任务归并和置信度；
- 重复 MCP、闲置、fanout、内存增长规则；
- 回收雷达；
- 生命周期河流接入真实历史。

验收：对真实 Agent 会话生成可解释任务，测试夹具归因正确率 ≥90%；生命周期异常能在河流中定位到证据。

### M3：安全回收 `[MVP 完成；仅 SIGTERM]`

- ActionPlan 预览；
- 爆炸半径预览；
- PID + start time 校验；
- 受保护进程；
- 优雅停止和强制停止流程；
- 逐 PID 动作进度；
- 审计日志；
- 集成测试。

验收：能安全停止测试任务树，不触碰父 Shell/IDE；用户能从预览中正确区分将停止和受保护节点。

### M4：AI 助手 `[MVP 完成；只读]`

- 本地意图/筛选；
- 可选 OpenAI-compatible provider；
- 脱敏上下文；
- 事实/推断/建议回答；
- 计划转 ActionPlan。

验收：无模型可用；模型失败时自动降级；模型不能直接执行动作。

### M5：视觉硬化与可访问性 `[部分完成]`

- 三套签名视图的完整视觉打磨；
- 动效编排、取消与 reduced-motion；
- 键盘全流程、200% 缩放和大字体；
- 亮色/暗色对比度；
- 图表文本替代；
- 性能预算和视觉回归门禁；
- 合成数据回放模式。

验收：视觉验收清单全部通过；60fps/P95/CLS 达标；reduced-motion 下功能完整；README 演示流程可在 fixture 上复现。

### M6：可分发与跨平台 `[未完成]`

- SQLite 历史；
- 菜单栏/tray；
- Linux/Windows collector；
- 构建 CI、签名、发布文档；
- README 首图与 20 秒演示视频；
- 性能与稳定性测试。

验收：三个平台均可启动，平台能力差异清晰可见，发行包可安装；发布截图和 GIF 与真实 UI 一致。

## 19. 关键技术风险

| 风险 | 缓解 |
|---|---|
| PID 复用导致误杀 | 每个动作绑定 PID + 启动时间 + workload revision |
| 任务归因错误 | 置信度、证据、不强行归类、用户纠正持久化 |
| macOS 权限/隐私限制 | 能力探测、按需读取、明确空状态 |
| 进程树在采样间变化 | 动作前重新快照并校验身份，不使用过期 PID |
| Docker socket 不可用 | 容器模块独立降级 |
| LLM 输出不可控 | 无工具权限、schema 校验、动作只接受预定义 ID |
| 令牌/命令行泄露 | 本地优先、脱敏、模型请求显式同意 |
| 采集开销过大 | 自适应采样、按需字段、批量存储、性能预算 |
| 跨平台差异 | collector contract + capability matrix + 平台集成测试 |
| 用户以为“可回收”就是安全 | 明确区分风险与动作；使用“疑似/建议”措辞；始终确认 |
| 信息密度导致视觉层级失控 | 首屏只保留一条结论；细节下钻；卡片清单在视觉验收中逐项删除 |
| 动效拖慢采样或造成布局抖动 | 只动画 transform/opacity；数据更新不逐节点补间；motion token + 性能预算 |
| 图表依赖颜色导致风险误读 | 图标、纹理、文本和表格降级；做色盲与对比度测试 |

## 20. 最终技术决策

采用：

- **MyGo WebView + Go backend**；
- **React + Vite**；
- **gopsutil + 平台增强采集器**；
- **自研 workload resolver + rule engine**；
- **Docker Engine HTTP API（MVP 只读，stop 为规划）**；
- **SQLite 历史（规划；MVP 未接入）**；
- **确定性 ActionPolicy + 用户确认**；
- **可选 OpenAI-compatible 模型**；
- **Signal Deck 视觉系统 + CSS/WAAPI 动效**；
- **Apache-2.0 开源**。

拒绝：

- 自动静默清理；
- 以 LLM 为唯一判断来源；
- 依赖特定 Agent 框架才能使用；
- 把远程遥测作为默认行为；
- 在 MVP 中伪造 GPU/能耗数据。
