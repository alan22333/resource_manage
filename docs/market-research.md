# 市场调研与机会判断

- 调研日期：2026-11-05
- 状态：已完成桌面研究，进入原型验证
- 结论：存在明确空白，但产品必须把“归因”和“安全动作”做成一等能力，不能停留在展示更多指标。

## 1. 执行摘要

AI 编程 Agent 已经进入高频本地使用阶段。Claude Code 和 Codex 的 GitHub 仓库分别有约 15.5 万和 13.3 万 stars；Stats、btop、Glances 等系统监控项目分别有约 4.3 万、3.5 万、3.4 万 stars；Portainer 等容器管理工具也有约 3.9 万 stars。这说明两件事：

1. 用户愿意为“看见本机状态”安装工具；
2. 开发者已经接受专门的容器/可观测性界面。

但现有产品形成了明显的割裂：系统监控器不理解 Agent 会话，容器工具不理解宿主机进程树，LLM 可观测性平台看不到本机 CPU/RSS，进程管理器又要求用户事先注册任务。

真正未被满足的需求是：

> **把系统资源重新归因到用户能理解的工作负载，并判断该负载是否仍然需要存在。**

“AI 任务管理器”的机会不在于再加一个聊天框，而在于成为本机工作负载的**解释层与安全操作层**。

## 2. 调研问题与方法

本轮重点回答：

1. 用户现在如何发现“电脑变卡/变热”的原因？
2. 系统监控、容器管理、进程托管和 Agent 可观测性工具各自覆盖了什么？
3. AI Agent 的进程行为是否让传统工具进一步失效？
4. 哪些需求是高频、强烈且愿意持续使用的？
5. 哪些功能看似相关，但会把产品做成大而无用的平台？

方法：

- 桌面研究：比较 7 类、20 余款相关产品；
- 公开问题分析：检查 Codex、Claude Code 的资源泄漏、进程堆积和过热问题；
- 工作流拆解：按“发现异常 → 定位来源 → 判断是否可收 → 执行回收 → 验证结果”分析现有方案的断点；
- 本地观察：在 macOS 上检查进程树、监听端口、Docker helper 和开发相关进程，确认跨层归因确实需要额外关联信息。

本轮没有进行大规模用户问卷，因此不把 GitHub stars 当作收入或市场规模证明。下一阶段需要用可交互原型验证时间收益和误杀担忧。

## 3. 竞品格局

| 类别 | 代表产品 | 做得好的部分 | 关键缺口 |
|---|---|---|---|
| 系统任务管理器 | macOS Activity Monitor、Windows Task Manager、GNOME System Monitor | 系统覆盖广，能查看 CPU、内存、磁盘、网络和 PID | 扁平进程视图；难以理解 Agent 会话、MCP、子 Agent 和容器之间的关系 |
| 终端/跨平台监控 | htop、btop、Glances | 资源指标丰富，进程操作直接，支持容器与远程监控 | 面向技术用户；缺少任务身份、生命周期语义和安全建议 |
| 菜单栏监控 | Stats、iStat Menus、iStatistica | 常驻可见、低打扰，适合看温度和基础负载 | 主要回答“整体是否忙”，不回答“哪个任务导致”；缺少清理闭环 |
| 容器管理 | Docker Desktop、OrbStack、Portainer、lazydocker、k9s | 容器、镜像、网络和日志管理完整 | 容器是孤岛；宿主机上运行 Agent、开发服务器、Jupyter 的上下文不可见 |
| 进程托管 | PM2、systemd、overmind、Procfile 工具 | 生命周期管理可靠，适合已知服务 | 要求预注册；无法管理临时产生、未托管的 Agent/IDE 子进程 |
| LLM/Agent 可观测性 | Langfuse、Opik、OpenAI Agents tracing | 能看调用链、耗时、token、工具调用和评测 | 这是应用层 tracing，不覆盖本机真实资源、MCP 进程、端口和残留进程 |
| Agent 编排/控制台 | 各类 agent dashboard、gateway、mission control | 能管理接入平台的 Agent 任务和状态 | 依赖特定网关/框架；不解决通用桌面环境和未接入任务 |
| 清理/优化工具 | CleanMyMac 等 | 能清理缓存、卸载软件、释放磁盘 | 不面向开发者工作负载；对活跃 Agent 和进程树缺少可信的安全模型 |

### 竞争结论

没有发现成熟产品同时满足以下四点：

1. 跨宿主进程、Agent 会话、开发服务器和容器进行统一归因；
2. 基于时间、进程树和资源行为判断工作负载是否可能失效；
3. 用证据解释原因，而不是只给一个模糊的“优化分数”；
4. 在不要求用户预先接入框架的前提下，提供安全、可预览的回收动作。

此外，现有 Agent 控制台的产品逻辑是“管理接入我的 Agent”，而不是“管理我电脑上实际存在的所有 Agent 和开发任务”。后者更接近用户遇到卡顿时真实需要解决的问题。

### 3.1 邻近 Agent 产品复核

Agent 生态已经出现一批开源监控产品，这说明需求不是凭空假设，但也意味着不能把“会话面板”“sub-agent 树”或“token 统计”当作本项目的核心创新。

| 邻近产品类型 | 代表项目 | 已解决 | 仍未解决 |
|---|---|---|---|
| Agent 会话/sub-agent 面板 | Claude-Code-Agent-Monitor、Agent-Quest、agent-monitor | 会话活动、工具调用、sub-agent 层级、任务状态 | 这些会话实际占用了多少宿主机 CPU/RSS；残留 MCP 是否还属于活跃会话 |
| token/额度监控 | Claude-Code-Usage-Monitor、token-monitor、ClaudeBar | token、成本、额度、重置时间 | 内存泄漏、进程爆发、端口、容器和清理动作 |
| MCP 配置管理 | mcp-hub、mcpm.sh、Claude Code Tool Manager | MCP 安装、配置、registry、profile | 已运行 MCP 的真实资源、重复实例、生命周期和孤立后代 |
| 系统监控器 | Activity Monitor、btop、Glances | CPU、内存、进程、端口 | 进程到 Agent 会话/工作负载的语义归属和安全回收 |
| 视觉化监控 | 通用 dashboard、Grafana、btop 主题 | 图表丰富、可定制、适合长时观察 | 进程层级与生命周期通常需要用户自己建立关联；视觉多为指标堆叠，不表达“为什么可收”和“关掉会发生什么” |

因此差异化必须落在**宿主资源语义**和**安全动作闭环**上，而不是再做一个 Agent 活动看板。Agent 会话看板回答“它正在做什么”，本项目首先回答“它现在占了我多少资源、是否还应该存在、关掉会发生什么”。

视觉表达同样是差异化的组成部分。现有工具大致走向两个极端：系统监视器信息密集但关系不明显，Agent 控制台好看但只展示接入后的会话和 token。本项目的机会是把宿主资源事实转化成有因果关系的视觉叙事，并让截图本身就能说明“重复 MCP 进程正在累积”。

## 4. 痛点证据

### 4.1 Agent 会制造大量后代进程，且生命周期与 UI 会话脱节

Codex issue #30408 报告：

- 25 个线程后出现 133 个残留 MCP 进程；
- 合计约 9.3 GB RSS；
- `playwright`、`davinci-resolve`、`node_repl` 和 `npm exec` wrapper 都在累积；
- 所有进程的 PPID 指向长期运行的 app-server，但无法从系统进程列表看出属于哪个已关闭线程。

后续评论还记录了 101 个直属子进程、128 个系统级 `node` 进程、219 个仍存活的 Docker MCP 客户端，以及文件描述符耗尽。这里的关键不是某个产品的单一 bug，而是：**操作系统的进程视图没有“会话所有权”这个概念**。

- https://github.com/openai/codex/issues/30408

### 4.2 高频 Agent 使用会直接表现为过热和卡顿

Codex issue #3022 的标题就是 “Codex extension makes VS Code extremely slow and overheats my computer”，复现步骤是安装扩展、打开中等规模项目、运行几分钟后 CPU 升高、风扇启动。

- https://github.com/openai/codex/issues/3022

另有 Codex issue #25719 记录了 macOS `syspolicyd`/`trustd` 被触发后 CPU 达到 125%–200%、RSS 超过 8 GB。用户需要看到的是“打开 Codex 之后哪个系统代理进程失控”，而不是在 Activity Monitor 中手工搜索。

- https://github.com/openai/codex/issues/25719

### 4.3 重复 MCP / 资源叠加会造成内存耗尽

Claude Code issue #20412 记录了重复注入 MCP server 后，在 16 GB 内存设备上 swap 被耗尽、会话被 OOM kill 的问题。远端 MCP 与本地 MCP 重复，用户很难从进程名判断哪些属于同一功能。

- https://github.com/anthropics/claude-code/issues/20412

### 4.4 传统工具要求用户自己完成关联和判断

当用户说“电脑好卡，帮我看看”时，一个专家实际会做：

1. 找到 CPU/内存占用最高的进程；
2. 沿 PPID 查看父进程和兄弟进程；
3. 读取命令行、启动时间、终端、工作目录和监听端口；
4. 判断它属于 IDE、Agent、Docker 还是系统服务；
5. 判断是否闲置、重复或异常；
6. 选择 SIGTERM、容器停止或进一步排查；
7. 确认是否误伤当前会话。

这些步骤需要跨工具、跨窗口操作，正是产品可以稳定替代的高频劳动。

## 5. 用户与场景

### 核心用户 A：AI 原生开发者

特征：

- 同时使用 Claude Code、Codex、Cursor、Copilot 或其他 Agent；
- 每台机器经常运行 2–10 个会话；
- 使用 VSCode/JetBrains、终端、Docker、本地开发服务器；
- 机器通常是 16–64 GB 内存的 Mac/Linux/Windows；
- 会工作很久，但不一定记得关闭旧会话。

典型触发：

- 风扇突然高速运转；
- 编译、测试或视频会议变卡；
- 电脑发热但 Activity Monitor 只显示一堆 `node`；
- 切到旧分支后发现旧 Agent 仍在跑；
- 想启动新容器/开发服务器但端口被占用。

### 核心用户 B：数据/ML 实验用户

- 同时运行 Jupyter、训练脚本、模型服务和数据预处理；
- 容易留下旧 notebook kernel、Python 子进程和 GPU 占用；
- 更关心“实验是否还在运行”和“是否值得继续”，而不只是进程名。

### 核心用户 C：独立开发者/小型 DevOps

- 本机运行 Docker Compose、数据库、反向代理和多套服务；
- 需要快速判断端口、容器和宿主机进程的对应关系；
- 不希望为了日常清理打开多个管理后台。

### 次要用户

- 需要做长期性能排查的高级用户；
- 需要向同事发送本机状态摘要的工程师；
- 未来可通过 MCP 向其他 Agent 提供只读系统状态。

## 6. Jobs to Be Done

### JTBD 1：快速定位

> 当电脑变热或变卡时，我想在 10 秒内知道是哪个任务、哪组进程造成的，而不是逐行查看 PID。

成功标准：首屏直接展示前三个造成压力的工作负载及其证据。

### JTBD 2：识别残留

> 当切换分支、关闭会话或让 Agent 运行很久后，我想知道哪些后台任务已经失去使用价值，以及它们占用了多少资源。

成功标准：能在任务列表中看到“疑似闲置”“重复 MCP”“子进程爆发”等可解释标签。

### JTBD 3：安全回收

> 我想回收资源，但必须确保不会误杀当前 IDE、Shell、系统服务和仍在运行的会话。

成功标准：动作前显示进程树、影响范围、通信方式和恢复策略；默认优雅停止，不静默强杀。

### JTBD 4：理解而不是猜测

> 我想用自然语言问“为什么这么卡”，并得到基于当前真实数据的答案和具体操作建议。

成功标准：回答引用任务名、PID、资源时间趋势和检测规则；无法确认时明确说明不确定。

### JTBD 5：持续低打扰观察

> 我希望在不一直打开完整窗口的情况下，随时知道系统是否健康、是否有新的资源浪费。

成功标准：菜单栏/托盘显示状态摘要；只在异常持续或出现高风险时提醒。

## 7. 产品机会与差异化

### 核心差异化

1. **工作负载归因**
   - 不是只按进程名分组，而是按父进程、进程组、会话、终端、工作目录、端口、容器标签和 Agent 签名联合归因。
   - 每个分组带置信度和证据，允许用户纠正并持久化。

2. **Agent 特有的生命周期问题**
   - 识别 MCP server、sub-agent、node_repl、浏览器 helper、Docker MCP 客户端等后代。
   - 检测重复、闲置、失控扩张和长时间持续增长。

3. **证据优先的 AI**
   - 不是让 LLM 看一张截图猜结论。
   - 规则引擎先产出结构化事实；LLM 只负责解释、排序和组织清理计划。
   - 没有模型时，核心检测和清理仍可用。

4. **默认安全**
   - 受保护进程、受保护任务、二次确认、PID 复用校验、进程树预览、审计日志。
   - Agent 永远不能直接执行破坏性动作；所有动作走确定性策略层。

5. **本地优先的开源桌面应用**
   - 单二进制、MyGo、无 Electron。
   - 默认不联网；云模型为显式可选能力。

6. **可解释的视觉语言**
   - 任务热力树把 RSS 分布和 CPU 强度映射到任务层级，让“谁在吃内存”一眼可见。
   - 生命周期河流展示进程出生、退出、子进程爆发和闲置，暴露会话结束后仍在累积的后代。
   - 爆炸半径预览直接标出将停止、受保护、阻断和可能影响 IDE/终端的节点，让危险边界可见。
   - 动效只用于解释状态变化，不做氛围装饰；所有图表有文本/表格替代。

### 7.1 为什么视觉质量是开源分发能力

开源桌面小工具的第一步往往不是安装，而是用户在 GitHub、社交媒体或朋友截图中看到它。视觉质量直接影响三个转化环节：

- **3 秒截图**：首屏必须同时展示“压力来源 + 主要任务 + 可回收容量”，否则看起来只是又一个系统监视器；
- **20 秒 GIF**：必须能从重复 MCP 堆积讲到生命周期异常、爆炸半径预览和安全回收，静音也能看懂；
- **60 秒试用**：安装后能完成一次真实清理，并且界面在 1Hz 更新下不抖、不闪、不自动滚动。

因此视觉、动效和可访问性被定义为 P0，而不是发布前的润色阶段。但“好看”必须服务于证据，不能变成与归因无关的仪表盘装饰。

### 不做的事

- 不做通用云端 APM；
- 不替代 Docker Desktop 或 Portainer；
- 不做自动杀进程的“一键加速”黑盒；
- 不承诺精确的跨进程共享内存计费；
- 不把未经确认的隐私数据发送给模型；
- 不在 MVP 阶段支持远程多机集群管理。

## 8. 市场风险与反证

### 风险：用户可能觉得 Activity Monitor 已经够用

反证：公开问题表明用户知道“电脑热”，但无法快速把进程、Agent 会话和 MCP 后代关联起来。产品只有在缩短定位和清理时间时才成立。

### 风险：AI 只是噱头

应对：先证明无 LLM 时的核心价值。规则检测、任务归因和安全清理必须独立可用；AI 用于降低提问和解释成本。

### 风险：误杀导致数据丢失

应对：不自动杀；默认保护编辑器、Shell、SSH、系统服务；显示完整影响范围；默认 SIGTERM；强杀单独确认；记录审计日志。

### 风险：跨平台采集成本高

应对：定义统一的 collector contract，macOS 先做深，Linux 做 beta，Windows 做 experimental；未支持的指标明确显示不可用，不伪造数据。

### 风险：把产品做成又一个过度复杂的平台

应对：MVP 只围绕“发现、解释、回收”三个动作。每个功能必须能指向一个 JTBD，否则推迟。

## 9. 结论

这个产品有真实痛点，但成立条件非常具体：

- 必须解决**归因**，否则只是另一个监控面板；
- 必须解决**安全回收闭环**，否则只是另一个告警工具；
- 必须坚持**本地优先和证据优先**，否则用户不会允许它处理进程和 AI 会话数据；
- 必须允许**零接入使用**，否则会退化成又一个要求特定 Agent 框架的 dashboard；
- 必须把复杂关系表达成**可截图、可演示、可一眼理解**的视觉叙事，否则开源传播和首次使用都会失败。

因此产品定位确定为：

> **AI 任务管理器：本地工作负载的发现、解释与安全回收工具。**

## 10. 继续投入的验证门槛

这个产品不能靠“功能看起来很酷”推进。下列指标用于决定继续投入、收窄范围或停止：

### 必须成立的核心假设

| 假设 | 验证方法 | 继续投入门槛 | 未达标时的处理 |
|---|---|---|---|
| 用户能理解“任务”抽象 | 8–12 名高频 Agent 用户的可用性测试 | ≥80% 能在 10 秒内说出主要压力任务 | 保留任务概念，但重做信息层级，不继续堆功能 |
| 任务归因明显优于进程列表 | 20 组真实/夹具任务，与专家标注对比 | 根任务归因正确率 ≥90%，高风险建议正确率 100% | 降低自动化等级，只展示证据，不提供批量清理 |
| 证据与预览能降低误杀焦虑 | 让用户解释“为什么可回收”并完成一次停止演练 | ≥80% 能正确说出影响范围；测试中零误杀 | 暂停回收功能，先解决安全模型 |
| 清理闭环确实节省时间 | 对比 Activity Monitor/终端工作流 | 中位定位时间 <10 秒，候选到完成清理 <30 秒 | 若只快几秒，产品没有独立价值，停止或重新定义 |
| 用户愿意长期保留 | 1 周 dogfooding + 使用频次/卸载意愿 | ≥60% 在一周后继续使用并至少触发一次有价值洞察 | 若只有故障时偶尔打开，改为轻量诊断工具而非常驻平台 |
| AI 带来增量价值 | 同一批任务对比本地规则问答与模型问答 | 多数场景缩短定位/解释时间，且不降低可信度 | AI 降为可选实验功能，不作为核心卖点 |
| 视觉降低理解成本 | 用首屏截图与 20 秒 GIF 测试 8–12 名目标用户 | ≥80% 在 5 秒内指出主要压力任务；≥90% 正确识别将停止/受保护节点 | 重做信息层级和可视化，不继续增加功能 |
| 设计具备开源传播力 | 对未安装产品的目标用户展示 README 首屏与 GIF | ≥60% 能复述“为什么值得试”且不误解为普通监控器 | 重新设计首屏和演示脚本，不以功能数量替代表达 |

### 先做“不可替代的四件事”

如果资源有限，只允许优先实现以下能力；其他功能都排在后面：

1. **可信归因**：用户能看懂任务由哪些进程组成，以及为什么这样归类。
2. **可解释回收候选**：至少能可靠发现重复 MCP、Agent 子进程爆发和疑似闲置会话。
3. **安全动作闭环**：预览影响范围、校验身份、优雅停止、记录结果。
4. **无 AI 的完整核心路径**：采集、归因、发现、清理均不依赖模型。

以上四件事必须通过一个可展示的视觉入口交付：首屏结论、任务热力树、生命周期河流和爆炸半径预览不是“四件事之外的装饰”，而是用户理解这四件事的主要界面。

### 停止条件

- 连续两轮原型中，用户仍认为任务列表只是“换了名字的进程列表”；
- 归因错误导致高风险建议无法达到 100% 可接受水平；
- 安全停止无法证明不会影响应用自身、父 Shell、IDE 和系统服务；
- 实际收益低于打开系统监视器超过 30 秒，且 AI 解释没有弥补差距；
- 只有加入特定 Agent 框架或预注册任务才有用，违背“零接入”原则；
- 视觉原型仍像通用任务管理器加皮肤，用户无法仅凭截图或短演示理解“归因、残留、回收边界”的差异。

达到任一停止条件时，不再增加功能，而是收窄为“Agent/MCP 进程诊断器”或重新验证需求。

## 11. 主要来源

> 访问日期均为 2026-11-05。GitHub stars 仅作为生态活跃度信号，不作为市场规模结论。

### 社区痛点与 Agent 进程证据

- Codex #30408：MCP server processes leak: per-thread processes never cleaned up  
  https://github.com/openai/codex/issues/30408
- Codex #3022：Codex extension makes VS Code extremely slow and overheats my computer  
  https://github.com/openai/codex/issues/3022
- Codex #25719：Codex Desktop repeatedly triggers `syspolicyd` / `trustd` CPU and memory runaway  
  https://github.com/openai/codex/issues/25719
- Claude Code #20412：MCP servers auto-injected without opt-in; duplicate servers and OOM  
  https://github.com/anthropics/claude-code/issues/20412

### 系统与资源监控

- Apple Activity Monitor User Guide  
  https://support.apple.com/guide/activity-monitor/welcome/mac
- Stats  
  https://github.com/exelban/stats
- btop  
  https://github.com/aristocratos/btop
- Glances  
  https://github.com/nicolargo/glances
- iStat Menus  
  https://bjango.com/mac/istatmenus/

### 容器与进程管理

- Docker Desktop / Docker Engine  
  https://docs.docker.com/
- Portainer  
  https://github.com/portainer/portainer
- k9s  
  https://github.com/derailed/k9s
- PM2  
  https://github.com/Unitech/pm2
- overmind  
  https://github.com/DarthSim/overmind

### Agent 可观测性

- Claude-Code-Agent-Monitor  
  https://github.com/hoangsonww/Claude-Code-Agent-Monitor
- Agent-Quest  
  https://github.com/FulAppiOS/Agent-Quest
- agent-monitor  
  https://github.com/donvito/agent-monitor
- MCP Hub  
  https://github.com/ravitemer/mcp-hub
- mcpm.sh  
  https://github.com/pathintegral-institute/mcpm.sh

- Langfuse  
  https://github.com/langfuse/langfuse
- Opik  
  https://github.com/comet-ml/opik
- OpenAI Agents SDK tracing  
  https://openai.github.io/openai-agents-python/tracing/
- OpenTelemetry GenAI semantic conventions  
  https://opentelemetry.io/docs/specs/semconv/gen-ai/

### 技术可行性

- MyGo  
  https://github.com/egoist/mygo
- MyGo SQLite plugin  
  https://www.npmjs.com/package/@mygo-plugins/sqlite
- gopsutil  
  https://github.com/shirou/gopsutil
- React  
  https://react.dev/
- Vite  
  https://vite.dev/
- uPlot  
  https://github.com/leeoniya/uPlot
