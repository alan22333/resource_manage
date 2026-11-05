import { useMemo, useState, type FormEvent } from "react";
import {
  ArrowRight,
  BrainCircuit,
  CheckCircle2,
  CircleHelp,
  Lightbulb,
  LockKeyhole,
  Send,
  Settings2,
  ShieldCheck,
  Sparkles,
  TriangleAlert,
} from "lucide-react";
import { TaskManager, type AIAnswer, type AIStatus, type Snapshot, type Workload } from "../mygo";
import { formatAge, formatBytes } from "../lib/format";

const suggestedQuestions = [
  "为什么电脑变卡？",
  "哪个任务留下了最多 MCP 后代？",
  "有什么可以安全回收？",
  "谁占用了 5173 端口？",
];

export function AIView({
  snapshot,
  selectedId,
  mode,
  bridgeAvailable,
  status,
  statusError,
  onOpenSettings,
}: {
  snapshot: Snapshot;
  selectedId: string | null;
  mode: "live" | "demo";
  bridgeAvailable: boolean;
  status: AIStatus | null;
  statusError: string | null;
  onOpenSettings: () => void;
}) {
  const [question, setQuestion] = useState("");
  const [submittedQuestion, setSubmittedQuestion] = useState("");
  const [answer, setAnswer] = useState<AIAnswer | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const selected = useMemo(
    () => snapshot.workloads.find((workload) => workload.id === selectedId) ?? null,
    [snapshot.workloads, selectedId],
  );

  const ask = async (override?: string) => {
    const prompt = (override ?? question).trim();
    if (!prompt) {
      setError("请输入一个诊断问题。");
      return;
    }
    if ([...prompt].length > 500) {
      setError("问题过长，请控制在 500 字以内。");
      return;
    }

    setQuestion(prompt);
    setSubmittedQuestion(prompt);
    setBusy(true);
    setError(null);
    try {
      const next = bridgeAvailable
        ? await TaskManager.explain(prompt, selectedId ?? "", mode === "demo")
        : answerInBrowser(prompt, snapshot, selectedId);
      setAnswer(next);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "诊断失败，请稍后重试。");
    } finally {
      setBusy(false);
    }
  };

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void ask();
  };

  const sourceLabel = answer ? sourceName(answer.source) : "";
  const statusLabel = status?.restoring ? "正在恢复模型配置" : status?.configured ? "模型增强已连接" : "使用本地规则";

  return (
    <div className="view-enter ai-page">
      <section className="ai-intro" aria-labelledby="assistant-hero-title">
        <div className="ai-intro__copy">
          <span className="section-kicker"><Sparkles size={15} aria-hidden="true" /> 只读 AI 诊断</span>
          <h2 id="assistant-hero-title">问清楚，再动手。</h2>
          <p>先由本地归因引擎整理任务关系和证据，再让 AI 把“电脑好卡”翻译成可以核查的原因。</p>
          <div className="ai-guardrails">
            <span><LockKeyhole size={14} /> 不执行动作</span>
            <span><ShieldCheck size={14} /> 不发送进程身份</span>
            <span><CheckCircle2 size={14} /> 失败自动降级</span>
          </div>
        </div>
        <button type="button" className="ai-engine-card" onClick={onOpenSettings}>
          <span className={`ai-engine-orb ${status?.configured ? "is-connected" : ""}`} aria-hidden="true"><BrainCircuit size={24} /></span>
          <span className="ai-engine-card__copy">
            <small>解释引擎</small>
            <strong>{statusLabel}</strong>
            <span>{status?.configured ? status.model : "无需 API Key，也能完整使用"}</span>
          </span>
          <Settings2 size={17} aria-hidden="true" />
        </button>
      </section>

      <div className="ai-workspace-grid">
        <section className="ai-chat-card" aria-labelledby="assistant-conversation-title">
          <header className="ai-card-head">
            <div>
              <span className="section-kicker">诊断工作台</span>
              <h2 id="assistant-conversation-title">现在想知道什么？</h2>
            </div>
            <span className={`ai-mode-pill ${status?.configured ? "is-model" : ""} ${status?.restoring ? "is-restoring" : ""}`}>
              <i aria-hidden="true" /> {statusLabel}
            </span>
          </header>

          <div className="ai-scope">
            <span className="ai-scope__label">分析范围</span>
            {selected ? (
              <button type="button" onClick={() => void ask(`分析当前选中的任务：${selected.name}`)}>
                <span>{selected.name}</span>
                <small>{selected.processCount} 个进程 · {formatBytes(selected.rssBytes)}</small>
                <ArrowRight size={15} aria-hidden="true" />
              </button>
            ) : (
              <p>整机快照。选择任务后可以聚焦到具体任务组。</p>
            )}
          </div>

          <div className="ai-suggestions" aria-label="常见诊断问题">
            {suggestedQuestions.map((item) => (
              <button type="button" key={item} onClick={() => void ask(item)} disabled={busy}>
                <CircleHelp size={15} aria-hidden="true" />
                {item}
              </button>
            ))}
          </div>

          <form className="ai-composer" onSubmit={submit}>
            <label htmlFor="assistant-question">你的问题</label>
            <div className="ai-composer__row">
              <input
                id="assistant-question"
                value={question}
                onChange={(event) => setQuestion(event.target.value)}
                placeholder="例如：为什么电脑变卡？有没有会话残留？"
                maxLength={500}
                autoComplete="off"
              />
              <button type="submit" className="primary-button primary-button--round" disabled={busy}>
                {busy ? "分析中" : "询问"}<Send size={16} aria-hidden="true" />
              </button>
            </div>
            <small>{[...question].length}/500 · AI 只解释当前快照，不会直接结束进程</small>
          </form>

          {error && (
            <div className="ai-error" role="alert">
              <TriangleAlert size={16} aria-hidden="true" />
              <span>{error}</span>
            </div>
          )}

          <div className="ai-answer-region" aria-live="polite">
            {busy ? (
              <div className="ai-thinking" role="status">
                <span className="ai-thinking__visual" aria-hidden="true"><i /><i /><i /></span>
                <div><strong>正在核对任务和回收证据</strong><small>事实来自当前快照；模型只负责解释。</small></div>
              </div>
            ) : answer ? (
              <article className="ai-answer" aria-labelledby="assistant-answer-title">
                <header>
                  <div>
                    <span className="ai-answer__source">{sourceLabel}</span>
                    <h3 id="assistant-answer-title">{answer.title}</h3>
                    {submittedQuestion && <p>“{submittedQuestion}”</p>}
                  </div>
                  {answer.model && <span className="answer-model">{answer.model}</span>}
                </header>

                <div className="ai-answer__section ai-answer__section--facts">
                  <span className="ai-answer__icon"><CheckCircle2 size={16} aria-hidden="true" /></span>
                  <div><strong>事实</strong><ul>{answer.facts.map((fact) => <li key={fact}>{fact}</li>)}</ul></div>
                </div>
                <div className="ai-answer__section ai-answer__section--inference">
                  <span className="ai-answer__icon"><BrainCircuit size={16} aria-hidden="true" /></span>
                  <div><strong>推断</strong><p>{answer.inference}</p></div>
                </div>
                <div className="ai-answer__section ai-answer__section--advice">
                  <span className="ai-answer__icon"><Lightbulb size={16} aria-hidden="true" /></span>
                  <div><strong>建议</strong><p>{answer.recommendation}</p></div>
                </div>
                <div className="ai-answer__section ai-answer__section--uncertainty">
                  <span className="ai-answer__icon"><TriangleAlert size={16} aria-hidden="true" /></span>
                  <div><strong>不确定项</strong><p>{answer.uncertainty}</p></div>
                </div>
                {answer.note && <p className="ai-answer__note">{answer.note}</p>}
              </article>
            ) : (
              <div className="ai-empty-answer">
                <span aria-hidden="true"><BrainCircuit size={25} /></span>
                <strong>回答会分成事实、推断、建议和不确定项</strong>
                <p>这样你能判断哪里是数据、哪里是解释，也能在清理前核对证据。</p>
              </div>
            )}
          </div>
        </section>

        <aside className="ai-side">
          <section className="ai-side-card" aria-labelledby="assistant-status-title">
            <header>
              <span className="section-kicker">解释引擎</span>
              <span className={`engine-light ${status?.configured ? "is-model" : ""}`} aria-hidden="true" />
            </header>
            <h2 id="assistant-status-title">{status?.restoring ? "正在恢复配置" : status?.configured ? "模型增强" : "本地优先"}</h2>
            <p>{status?.privacy ?? "正在读取解释引擎配置…"}</p>
            <dl>
              <div><dt>数据处理</dt><dd>{status?.remote ? "显式开启的远程端点" : "仅本机"}</dd></div>
              <div><dt>默认模式</dt><dd>无模型也可用</dd></div>
              <div><dt>失败策略</dt><dd>自动降级本地规则</dd></div>
            </dl>
            {statusError && <p className="ai-side__error">状态读取失败：{statusError}</p>}
            <button type="button" className="secondary-button secondary-button--full" onClick={onOpenSettings}>
              <Settings2 size={16} aria-hidden="true" /> 配置 AI 模型
            </button>
          </section>

          <section className="ai-side-card ai-safety-card" aria-labelledby="assistant-safety-title">
            <span className="ai-safety-icon"><ShieldCheck size={19} aria-hidden="true" /></span>
            <h2 id="assistant-safety-title">AI 只读，动作始终由你确认</h2>
            <p>回答里没有动作令牌或 shell 命令。任何清理都必须回到爆炸半径预览并手动确认。</p>
            <ul>
              <li>最多发送 12 个任务的聚合摘要</li>
              <li>不发送 PID、完整命令行或环境变量</li>
              <li>不读取 Agent 对话内容</li>
            </ul>
          </section>
        </aside>
      </div>
    </div>
  );
}

function sourceName(source: string) {
  switch (source) {
    case "openai-compatible":
      return "模型解释";
    case "local-rules-fallback":
      return "本地规则 · 已降级";
    case "browser-rules":
      return "浏览器本地规则";
    default:
      return "本地规则";
  }
}

function answerInBrowser(question: string, snapshot: Snapshot, selectedId: string | null): AIAnswer {
  const normalized = question.toLowerCase();
  const tasks = snapshot.workloads;
  const selected = tasks.find((task) => task.id === selectedId) ?? highestBy(tasks, "rssBytes") ?? tasks[0];
  const topRSS = highestBy(tasks, "rssBytes");
  const topCPU = highestBy(tasks, "cpuPercent");
  const reclaimable = tasks.filter((task) => task.reclaimable).sort((a, b) => b.rssBytes - a.rssBytes);

  if (normalized.includes("mcp") || normalized.includes("子进程") || normalized.includes("后代")) {
    const target = [...tasks].sort((a, b) => countMCP(b) - countMCP(a))[0];
    const mcpCount = target ? countMCP(target) : 0;
    return {
      title: mcpCount > 0 ? `${target.name} 的 MCP 后代最多` : "没有识别到 MCP 后代",
      facts: mcpCount > 0
        ? [
            `${target.name} 中有 ${mcpCount} 个 MCP 后代，任务总 RSS 为 ${formatBytes(target.rssBytes)}。`,
            `该任务组当前包含 ${target.processCount} 个进程，风险标记为“${target.riskLabel}”。`,
          ]
        : [`当前 ${tasks.length} 个任务组中没有命中 MCP 进程签名。`],
      inference: mcpCount > 0 ? "多个同签名后代长期存活，符合会话切换后残留的典型模式。" : "未知命令或权限限制可能让 MCP 进程未被识别。",
      recommendation: mcpCount > 0 ? "打开爆炸半径预览，核对将停止的后代与受保护的 Agent 根。" : "继续检查 CPU 或内存占用最高的任务。",
      uncertainty: "浏览器预览使用确定性演示快照；最终判断应以桌面窗口中的当前证据为准。",
      source: "browser-rules",
      model: "",
      note: "浏览器预览只在本地分析合成快照，不会执行任何动作。",
    };
  }

  const port = question.match(/(?:端口|port|:)\s*(\d{2,5})/i)?.[1];
  if (normalized.includes("端口") || normalized.includes("port")) {
    const matches = port ? tasks.filter((task) => task.processes.some((process) => process.command.includes(`:${port}`)) || task.evidence.some((evidence) => evidence.detail.includes(port))) : [];
    return {
      title: matches.length && port ? `端口 ${port} 归属于 ${matches[0].name}` : port ? `没有找到端口 ${port} 的归属` : "请提供端口号",
      facts: matches.length && port
        ? matches.map((task) => `${task.name} 的命令或证据包含端口 ${port}，RSS ${formatBytes(task.rssBytes)}。`)
        : [`当前 ${tasks.length} 个任务组中没有足够证据关联该端口。`],
      inference: matches.length ? "监听中的开发服务通常是仍在使用的任务。" : "端口可能未监听、由未归类进程占用，或采集权限不足。",
      recommendation: matches.length ? "如果这是预期的开发服务请保留；确认是遗留服务后再处理。" : "刷新快照，并使用系统级端口工具交叉验证。",
      uncertainty: "浏览器预览没有读取系统套接字。",
      source: "browser-rules",
      model: "",
      note: "浏览器预览只在本地分析合成快照，不会执行任何动作。",
    };
  }

  if (normalized.includes("回收") || normalized.includes("清理") || normalized.includes("释放")) {
    return {
      title: reclaimable.length ? "先处理证据最完整的残留后代" : "没有足够证据支持自动清理",
      facts: [
        `当前有 ${reclaimable.length} 个任务组满足回收证据，涉及 ${snapshot.summary.reclaimProcessCount} 个可分离后代。`,
        `保守估计可释放 ${formatBytes(snapshot.summary.reclaimBytes)}。`,
      ],
      inference: reclaimable.length ? "这些任务更可能是会话结束后遗留的 MCP 或 worker 后代。" : "当前快照没有任务同时满足宿主关系、空闲和重复后代等条件。",
      recommendation: reclaimable.length ? `先预览“${reclaimable[0].name}”的爆炸半径；确认前不会发送任何信号。` : "继续保持观察，不要结束归属不明确的进程。",
      uncertainty: "释放量基于当前 RSS 估算，实际回收值可能不同。",
      source: "browser-rules",
      model: "",
      note: "浏览器预览只在本地分析合成快照，不会执行任何动作。",
    };
  }

  if (normalized.includes("cpu") || normalized.includes("处理器") || normalized.includes("内存") || normalized.includes("memory") || normalized.includes("卡") || normalized.includes("慢") || normalized.includes("热")) {
    return {
      title: "电脑变卡时先看这三个信号",
      facts: [
        `整机 CPU ${snapshot.system.cpuPercent.toFixed(1)}%，已用内存 ${formatBytes(snapshot.system.memoryUsed)} / ${formatBytes(snapshot.system.memoryTotal)}。`,
        topCPU ? `CPU 最高的任务组是 ${topCPU.name}（${topCPU.cpuPercent.toFixed(1)}%）。` : "当前没有任务级 CPU 数据。",
        topRSS ? `RSS 最高的任务组是 ${topRSS.name}（${formatBytes(topRSS.rssBytes)}，${topRSS.processCount} 个进程）。` : "当前没有任务级内存数据。",
        snapshot.summary.reclaimBytes > 0 ? `另有 ${snapshot.summary.reclaimCount} 个任务组满足回收证据，估计可释放 ${formatBytes(snapshot.summary.reclaimBytes)}。` : "当前没有足够证据支持自动清理。",
      ],
      inference: "整机压力是结果，真正的处置对象应是可解释、可分离且证据完整的任务组。",
      recommendation: reclaimable[0] ? `先打开“${reclaimable[0].name}”的爆炸半径预览，核对将停止的后代与受保护的 Agent 根。` : "继续观察 CPU、RSS 与最后活动时间，不要结束归属不明确的进程。",
      uncertainty: "单次采样无法区分持续负载与瞬时峰值，GPU、温度和磁盘 I/O 也不在当前快照内。",
      source: "browser-rules",
      model: "",
      note: "浏览器预览只在本地分析合成快照，不会执行任何动作。",
    };
  }

  return {
    title: selected?.name ?? "当前没有足够的任务数据",
    facts: selected
      ? [
          `${selected.name} 来自 ${selected.source}，已运行 ${formatAge(selected.ageSeconds)}。`,
          `RSS ${formatBytes(selected.rssBytes)}，任务级 CPU ${selected.cpuPercent.toFixed(1)}%，包含 ${selected.processCount} 个进程。`,
          `风险：${selected.riskLabel}。${selected.riskDetail}`,
        ]
      : ["采集器没有返回可分析的任务组。"],
    inference: selected ? "该结论来自确定性演示快照与本地归因规则。" : "无法判断资源消耗来源。",
    recommendation: selected?.reclaimable
      ? "打开爆炸半径预览，核对将停止的后代、受保护的 Agent 根和预计释放量。"
      : "当前没有足够证据回收该任务，继续观察其活动时间和资源趋势。",
    uncertainty: selected ? `归因置信度为 ${selected.confidence}%；浏览器预览使用的是合成数据。` : "空快照不能被视为健康状态。",
    source: "browser-rules",
    model: "",
    note: "浏览器预览只在本地分析合成快照，不会执行任何动作。",
  };
}

function countMCP(workload: Workload) {
  return workload.processes.filter((process) => process.kind === "mcp").length;
}

function highestBy(workloads: Workload[], field: "rssBytes" | "cpuPercent") {
  return [...workloads].sort((a, b) => b[field] - a[field])[0];
}
