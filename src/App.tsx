import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import {
  Activity,
  ArrowRight,
  Boxes,
  BrainCircuit,
  Container,
  Cpu,
  GitBranch,
  LayoutDashboard,
  LockKeyhole,
  MemoryStick,
  Moon,
  PanelLeftClose,
  RefreshCw,
  ScrollText,
  Search,
  Settings2,
  ShieldCheck,
  Sparkles,
  Sun,
  TriangleAlert,
  X,
} from "lucide-react";
import { isMyGo } from "mygo-runtime";
import type { ActionResult, AIStatus, Event as ActivityEvent, Snapshot, Workload } from "./mygo";
import { TaskManager } from "./mygo";
import fixture from "./fixtures/demo-snapshot.json";
import { ActionDialog } from "./components/ActionDialog";
import { AISettingsDialog } from "./components/AISettingsDialog";
import { AIView } from "./components/AIView";
import { AuditLog } from "./components/AuditLog";
import { HeatTree } from "./components/HeatTree";
import { LifecycleRiver } from "./components/LifecycleRiver";
import { ThermalCard } from "./components/ThermalCard";
import { applyLocalResult, describeOverviewVerdict, describeWorkloadHeadline, findPrimaryWorkload, findReclaimable } from "./lib/snapshot";
import { formatAge, formatBytes, formatPercent, formatTime, signalTone } from "./lib/format";

type View = "overview" | "tasks" | "lifecycle" | "audit" | "assistant";
type Mode = "live" | "demo";
type Theme = "dark" | "light";

const demoSnapshot = fixture as unknown as Snapshot;
const bridgeAvailable = isMyGo();

const navigation: Array<{ id: View; label: string; icon: typeof LayoutDashboard }> = [
  { id: "overview", label: "概览", icon: LayoutDashboard },
  { id: "tasks", label: "任务", icon: Boxes },
  { id: "lifecycle", label: "生命周期", icon: GitBranch },
  { id: "assistant", label: "AI 诊断", icon: Sparkles },
  { id: "audit", label: "审计", icon: ScrollText },
];

const browserAIStatus: AIStatus = {
  mode: "local-rules",
  provider: "local",
  model: "",
  baseUrl: "",
  remote: false,
  privacy: "浏览器预览只分析内置演示快照，不发起网络请求。",
  configured: false,
  apiKeyAvailable: false,
  source: "browser",
  restoring: false,
};

const THEME_STORAGE_KEY = "ai-task-manager-theme";

function getInitialTheme(): Theme {
  try {
    const stored = localStorage.getItem(THEME_STORAGE_KEY);
    if (stored === "light" || stored === "dark") return stored;
  } catch {
    // WebView privacy settings can make storage unavailable; visual defaults still work.
  }
  return document.documentElement.dataset.theme === "light" ? "light" : "dark";
}

function App() {
  const [view, setView] = useState<View>("overview");
  const [mode, setMode] = useState<Mode>(bridgeAvailable ? "live" : "demo");
  const [snapshot, setSnapshot] = useState<Snapshot | null>(bridgeAvailable ? null : demoSnapshot);
  const [selectedId, setSelectedId] = useState<string | null>(demoSnapshot.summary.topWorkloadId);
  const [previewWorkload, setPreviewWorkload] = useState<Workload | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [aiSettingsOpen, setAISettingsOpen] = useState(false);
  const [theme, setTheme] = useState<Theme>(getInitialTheme);
  const [loading, setLoading] = useState(bridgeAvailable);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [kindFilter, setKindFilter] = useState("all");
  const [aiStatus, setAIStatus] = useState<AIStatus | null>(bridgeAvailable ? null : browserAIStatus);
  const [aiStatusError, setAIStatusError] = useState<string | null>(null);
  const snapshotRef = useRef(snapshot);
  snapshotRef.current = snapshot;

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    try {
      localStorage.setItem(THEME_STORAGE_KEY, theme);
    } catch {
      // Theme persistence is a convenience; rendering must not depend on it.
    }
  }, [theme]);

  const loadSnapshot = useCallback(async (requestedMode: Mode, quiet = false) => {
    if (!bridgeAvailable) {
      if (requestedMode === "demo") {
        setSnapshot(demoSnapshot);
        setError(null);
        setLoading(false);
        return;
      }
      setError("浏览器预览无法读取本机进程。请在 MyGo 桌面窗口中使用实时模式，或打开演示数据。");
      setLoading(false);
      return;
    }

    if (quiet) setRefreshing(true);
    else setLoading(true);
    try {
      const next = await TaskManager.bootstrap(requestedMode === "demo");
      setSnapshot(next);
      setMode(requestedMode);
      setSelectedId((current) => {
        if (current && next.workloads.some((workload) => workload.id === current)) return current;
        return next.summary.topWorkloadId || next.workloads[0]?.id || null;
      });
      setError(null);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "无法读取任务快照");
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  const refreshAIStatus = useCallback(async () => {
    if (!bridgeAvailable) {
      setAIStatus(browserAIStatus);
      return;
    }
    try {
      const next = await TaskManager.aiStatus();
      setAIStatus(next);
      setAIStatusError(null);
    } catch (reason) {
      setAIStatusError(reason instanceof Error ? reason.message : "无法读取 AI 服务状态");
    }
  }, []);

  useEffect(() => {
    void loadSnapshot(mode);
  }, [loadSnapshot, mode]);

  useEffect(() => {
    void refreshAIStatus();
    if (!bridgeAvailable || (aiStatus && !aiStatus.restoring)) return;
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") void refreshAIStatus();
    }, 700);
    return () => window.clearInterval(timer);
  }, [aiStatus?.restoring, bridgeAvailable, refreshAIStatus]);

  useEffect(() => {
    if (!bridgeAvailable) return;
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") void loadSnapshot(mode, true);
    }, 4000);
    return () => window.clearInterval(timer);
  }, [bridgeAvailable, loadSnapshot, mode]);

  useEffect(() => {
    if (!snapshot) return;
    const exists = selectedId && snapshot.workloads.some((workload) => workload.id === selectedId);
    if (!exists) setSelectedId(snapshot.summary.topWorkloadId || snapshot.workloads[0]?.id || null);
  }, [snapshot, selectedId]);

  const primaryWorkload = snapshot ? findPrimaryWorkload(snapshot) : undefined;
  const reclaimable = snapshot ? findReclaimable(snapshot) : undefined;
  const filteredWorkloads = useMemo(() => {
    if (!snapshot) return [];
    const needle = query.trim().toLowerCase();
    return snapshot.workloads.filter((workload) => {
      if (kindFilter !== "all" && workload.kind !== kindFilter) return false;
      if (!needle) return true;
      return [workload.name, workload.source, workload.riskDetail, ...workload.processes.map((process) => process.command)]
        .some((value) => value.toLowerCase().includes(needle));
    });
  }, [snapshot, query, kindFilter]);

  const openPreview = (workload: Workload) => {
    setPreviewWorkload(workload);
    setDialogOpen(true);
  };

  const handleCompleted = (result: ActionResult) => {
    if (bridgeAvailable) {
      void loadSnapshot(mode, true);
    } else {
      setSnapshot((current) => current ? applyLocalResult(current, result) : current);
    }
  };

  const switchMode = (nextMode: Mode) => {
    if (nextMode === mode && snapshot) return;
    setMode(nextMode);
    if (nextMode === "demo" && !bridgeAvailable) {
      setSnapshot(demoSnapshot);
      setSelectedId(demoSnapshot.summary.topWorkloadId);
      setError(null);
      setLoading(false);
    }
  };

  const primaryAction = () => {
    const target = primaryWorkload?.reclaimable ? primaryWorkload : reclaimable;
    if (target) openPreview(target);
  };

  const currentNav = navigation.find((item) => item.id === view) ?? navigation[0];
  const generatedLabel = snapshot ? formatTime(snapshot.generatedAt) : "正在采集";

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">跳到主要内容</a>

      <aside className="sidebar" aria-label="主导航">
        <div className="brand-mark" aria-label="AI 任务管理器">
          <span className="brand-mark__glyph"><Activity size={21} /></span>
          <span className="brand-mark__pulse" aria-hidden="true" />
        </div>

        <nav className="nav-list">
          {navigation.map(({ id, label, icon: Icon }) => (
            <button
              type="button"
              className={`nav-item ${view === id ? "is-active" : ""}`}
              aria-current={view === id ? "page" : undefined}
              onClick={() => setView(id)}
              key={id}
            >
              <Icon size={21} strokeWidth={1.8} aria-hidden="true" />
              <span>{label}</span>
            </button>
          ))}
        </nav>

        <div className="sidebar-bottom">
          <span className="sidebar-trust" title="本地优先 · 无遥测">
            <ShieldCheck size={18} aria-hidden="true" />
          </span>
          <span className="sidebar-version">0.2.0</span>
        </div>
      </aside>

      <div className="workspace">
        <header className="topbar">
          <div className="page-heading">
            <span className="page-eyebrow">AI TASK MANAGER</span>
            <h1>{currentNav.label}</h1>
          </div>
          <div className="topbar-actions">
            <span className={`collection-state collection-state--${mode}`}>
              <i aria-hidden="true" />
              {mode === "live" ? "实时" : "演示"}
              {snapshot && <small>{snapshot.capabilities.collectionMs} ms</small>}
            </span>
            <div className="mode-switch" role="group" aria-label="数据模式">
              <button type="button" className={mode === "live" ? "is-active" : ""} onClick={() => switchMode("live")} aria-pressed={mode === "live"}>实时</button>
              <button type="button" className={mode === "demo" ? "is-active" : ""} onClick={() => switchMode("demo")} aria-pressed={mode === "demo"}>演示</button>
            </div>
            <button type="button" className="ai-settings-trigger" onClick={() => setAISettingsOpen(true)}>
              <span className={`ai-settings-trigger__dot ${aiStatus?.configured ? "is-connected" : ""}`} aria-hidden="true" />
              <BrainCircuit size={17} aria-hidden="true" />
              <span>AI 设置</span>
            </button>
            <button
              type="button"
              className="icon-button"
              onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
              aria-label={theme === "dark" ? "切换到亮色主题" : "切换到暗色主题"}
              title={theme === "dark" ? "亮色主题" : "暗色主题"}
            >
              {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
            </button>
            <button
              type="button"
              className={`icon-button ${refreshing ? "is-refreshing" : ""}`}
              onClick={() => void loadSnapshot(mode, true)}
              aria-label="刷新任务快照"
              title="刷新任务快照"
              disabled={refreshing || loading}
            >
              <RefreshCw size={18} />
            </button>
          </div>
        </header>

        <main id="main-content" className="main-content" tabIndex={-1}>
          {mode === "demo" && (
            <div className="demo-ribbon" role="status">
              <span className="demo-badge">演示数据</span>
              <span>所有 PID 与资源均为合成数据，用于验证完整流程。</span>
              <span className="demo-ribbon__time">快照 {generatedLabel}</span>
            </div>
          )}

          {error && (
            <div className="error-banner" role="alert">
              <span className="error-banner__icon"><TriangleAlert size={18} aria-hidden="true" /></span>
              <div><strong>实时采集暂不可用</strong><p>{error}</p></div>
              <button type="button" className="secondary-button" onClick={() => switchMode("demo")}>打开演示数据</button>
              <button type="button" className="icon-button" onClick={() => setError(null)} aria-label="关闭错误提示"><X size={17} /></button>
            </div>
          )}

          {loading && !snapshot ? (
            <DashboardSkeleton />
          ) : snapshot ? (
            <>
              {view === "overview" && (
                <OverviewView
                  snapshot={snapshot}
                  selectedId={selectedId}
                  aiStatus={aiStatus}
                  onSelect={setSelectedId}
                  onPreview={openPreview}
                  onPrimaryAction={primaryAction}
                  onNavigate={setView}
                />
              )}
              {view === "tasks" && (
                <TasksView
                  workloads={filteredWorkloads}
                  allWorkloads={snapshot.workloads}
                  selectedId={selectedId}
                  query={query}
                  kindFilter={kindFilter}
                  onQuery={setQuery}
                  onKindFilter={setKindFilter}
                  onSelect={setSelectedId}
                  onPreview={openPreview}
                />
              )}
              {view === "lifecycle" && (
                <LifecycleView snapshot={snapshot} selectedId={selectedId} onSelect={setSelectedId} />
              )}
              {view === "audit" && (
                <AuditView snapshot={snapshot} />
              )}
              {view === "assistant" && (
                <AIView
                  snapshot={snapshot}
                  selectedId={selectedId}
                  mode={mode}
                  bridgeAvailable={bridgeAvailable}
                  status={aiStatus}
                  statusError={aiStatusError}
                  onOpenSettings={() => setAISettingsOpen(true)}
                />
              )}
            </>
          ) : (
            <EmptyState onDemo={() => switchMode("demo")} />
          )}
        </main>
      </div>

      <ActionDialog
        workload={previewWorkload}
        mode={mode}
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onCompleted={handleCompleted}
      />

      <AISettingsDialog
        open={aiSettingsOpen}
        onOpenChange={setAISettingsOpen}
        bridgeAvailable={bridgeAvailable}
        status={aiStatus}
        onStatusChange={(next) => {
          setAIStatus(next);
          setAIStatusError(null);
        }}
      />
    </div>
  );
}

function OverviewView({
  snapshot,
  selectedId,
  aiStatus,
  onSelect,
  onPreview,
  onPrimaryAction,
  onNavigate,
}: {
  snapshot: Snapshot;
  selectedId: string | null;
  aiStatus: AIStatus | null;
  onSelect: (id: string) => void;
  onPreview: (workload: Workload) => void;
  onPrimaryAction: () => void;
  onNavigate: (view: View) => void;
}) {
  const primary = findPrimaryWorkload(snapshot);
  const reclaimable = findReclaimable(snapshot);
  const target = primary?.reclaimable ? primary : reclaimable;
  const topWorkloads = [...snapshot.workloads].sort((a, b) => b.rssBytes - a.rssBytes).slice(0, 4);
  const memoryPercent = snapshot.system.memoryTotal ? (snapshot.system.memoryUsed / snapshot.system.memoryTotal) * 100 : 0;
  const activeWorkloads = snapshot.summary.activeWorkloads;
  const attentionCount = snapshot.summary.attentionCount;
  const verdict = describeOverviewVerdict(snapshot, Boolean(target));

  return (
    <div className="view-enter overview-page">
      <section className="overview-hero" aria-labelledby="verdict-title">
        <div className="hero-copy">
          <span className={`hero-status ${verdict.tone === "attention" ? "is-attention" : "is-calm"}`}>
            <i aria-hidden="true" /> {snapshot.summary.pressureLabel}
          </span>
          <h2 id="verdict-title">
            {verdict.firstLine}<br /><em>{verdict.emphasis}</em>
          </h2>
          <p>
            {primary
              ? describeWorkloadHeadline(primary)
              : "当前没有发现证据完整的会话残留。继续观察，不需要结束任何进程。"}
            {target && <> 涉及 {snapshot.summary.reclaimCount} 个任务组、{snapshot.summary.reclaimProcessCount} 个可分离进程。</>}
          </p>
          <div className="hero-actions">
            {target ? (
              <button type="button" className="primary-button primary-button--large" onClick={onPrimaryAction}>
                <ShieldCheck size={18} aria-hidden="true" /> 查看安全回收 <ArrowRight size={17} aria-hidden="true" />
              </button>
            ) : (
              <span className="all-clear"><ShieldCheck size={18} aria-hidden="true" /> 暂无可回收任务</span>
            )}
            <button type="button" className="secondary-button secondary-button--large" onClick={() => onNavigate("tasks")}>
              查看全部任务
            </button>
          </div>
          <div className="hero-footnote">
            <ShieldCheck size={15} aria-hidden="true" />
            <span>只发送 SIGTERM · 执行前核对 PID 与启动时间 · 关键进程始终保留</span>
          </div>
        </div>

        <ResourceOrb
          reclaimBytes={snapshot.summary.reclaimBytes}
          reclaimCount={snapshot.summary.reclaimProcessCount}
          totalRssBytes={snapshot.summary.totalRssBytes}
          cpuPercent={snapshot.system.cpuPercent}
          memoryPercent={memoryPercent}
          attentionCount={attentionCount}
          confidence={snapshot.summary.confidence}
        />
      </section>

      <ThermalCard
        thermal={snapshot.thermal}
        workloads={snapshot.workloads}
        onInspect={(workloadId) => {
          onSelect(workloadId);
          onNavigate("tasks");
        }}
        onPreview={onPreview}
      />

      <section className="overview-metrics" aria-label="系统信号">
        <MetricCard
          icon={Cpu}
          label="CPU"
          value={`${formatPercent(snapshot.system.cpuPercent)}%`}
          detail={`${snapshot.system.processCount} 个进程`}
          meter={snapshot.system.cpuPercent}
          tone={snapshot.system.cpuPercent > 70 ? "warning" : "mint"}
        />
        <MetricCard
          icon={MemoryStick}
          label="统一内存"
          value={formatBytes(snapshot.system.memoryUsed)}
          detail={`共 ${formatBytes(snapshot.system.memoryTotal)}`}
          meter={memoryPercent}
          tone={memoryPercent > 80 ? "warning" : "blue"}
        />
        <MetricCard
          icon={Boxes}
          label="活跃任务"
          value={String(activeWorkloads)}
          detail={`${topWorkloads.length} 个任务组在视野内`}
          meter={Math.min(100, activeWorkloads * 8)}
          tone="violet"
        />
        <MetricCard
          icon={BrainCircuit}
          label="需要关注"
          value={String(attentionCount)}
          detail={`${snapshot.system.agentSessions} 个 Agent 会话`}
          meter={Math.min(100, attentionCount * 20)}
          tone={attentionCount > 0 ? "coral" : "mint"}
        />
      </section>

      <div className="overview-lower-grid">
        <section className="focus-card" aria-labelledby="focus-title">
          <header className="card-heading">
            <div>
              <span className="section-kicker">任务热点</span>
              <h2 id="focus-title">谁在占用资源</h2>
            </div>
            <button type="button" className="text-button" onClick={() => onNavigate("tasks")}>
              全部任务 <ArrowRight size={15} aria-hidden="true" />
            </button>
          </header>

          <div className="workload-list">
            {topWorkloads.map((workload, index) => {
              const Icon = workloadIcon(workload.kind);
              const tone = signalTone(workload.risk);
              const share = workload.rssBytes / Math.max(...topWorkloads.map((item) => item.rssBytes), 1) * 100;
              return (
                <div className={`workload-row ${selectedId === workload.id ? "is-selected" : ""}`} key={workload.id}>
                  <button
                    type="button"
                    className="workload-row__main"
                    onClick={() => { onSelect(workload.id); onNavigate("tasks"); }}
                  >
                    <span className={`workload-icon workload-icon--${tone}`}><Icon size={18} aria-hidden="true" /></span>
                    <span className="workload-identity">
                      <strong>{workload.name}</strong>
                      <small>{workload.source} · {workload.processCount} 个进程</small>
                    </span>
                    <span className="workload-usage">
                      <strong>{formatBytes(workload.rssBytes)}</strong>
                      <small>{formatPercent(workload.cpuPercent)}% CPU</small>
                    </span>
                    <span className={`risk-label risk-label--${tone}`}><i aria-hidden="true" />{workload.riskLabel}</span>
                  </button>
                  <span className="workload-progress" aria-hidden="true">
                    <i className={`workload-progress__fill workload-progress__fill--${tone}`} style={{ width: `${Math.max(4, share)}%` }} />
                  </span>
                  <span className="workload-rank" aria-hidden="true">0{index + 1}</span>
                </div>
              );
            })}
          </div>
        </section>

        <section className="ai-preview-card" aria-labelledby="ai-preview-title">
          <span className="ai-preview-card__halo" aria-hidden="true" />
          <header className="card-heading">
            <div>
              <span className="section-kicker"><Sparkles size={14} aria-hidden="true" /> AI 诊断</span>
              <h2 id="ai-preview-title">先问一句，再决定。</h2>
            </div>
            <span className={`ai-mode-pill ${aiStatus?.configured ? "is-model" : ""}`}><i aria-hidden="true" />{aiStatus?.configured ? "模型增强" : "本地规则"}</span>
          </header>
          <p className="ai-preview-copy">AI 会把当前任务关系整理成事实、推断、建议和不确定项，但不会生成可执行命令。</p>

          {primary && (
            <div className="insight-snapshot">
              <span className={`insight-snapshot__icon insight-snapshot__icon--${signalTone(primary.risk)}`}>
                <BrainCircuit size={18} aria-hidden="true" />
              </span>
              <div>
                <strong>{primary.riskDetail}</strong>
                <span>归因置信度 {primary.confidence}%</span>
              </div>
            </div>
          )}

          <div className="ai-preview-actions">
            <button type="button" className="primary-button" onClick={() => onNavigate("assistant")}>
              <Sparkles size={16} aria-hidden="true" /> 打开 AI 诊断
            </button>
            <button type="button" className="text-button" onClick={() => onNavigate("assistant")}>
              为什么电脑变卡？ <ArrowRight size={15} aria-hidden="true" />
            </button>
          </div>
        </section>
      </div>

      <section className="trust-strip" aria-label="安全边界">
        <div><span><ShieldCheck size={17} /></span><strong>单次令牌</strong><p>预览 2 分钟后失效，确认只执行一次。</p></div>
        <div><span><Activity size={17} /></span><strong>身份二次校验</strong><p>执行前重新核对 PID 与启动时间。</p></div>
        <div><span><LockKeyhole size={17} /></span><strong>关键进程保护</strong><p>IDE、Shell、Agent 根和容器不批量停止。</p></div>
        <button type="button" className="text-button" onClick={() => onNavigate("audit")}>查看完整边界 <ArrowRight size={15} /></button>
      </section>

      <AuditLog events={snapshot.events} limit={3} />
    </div>
  );
}

function ResourceOrb({
  reclaimBytes,
  reclaimCount,
  totalRssBytes,
  cpuPercent,
  memoryPercent,
  attentionCount,
  confidence,
}: {
  reclaimBytes: number;
  reclaimCount: number;
  totalRssBytes: number;
  cpuPercent: number;
  memoryPercent: number;
  attentionCount: number;
  confidence: number;
}) {
  const orbitProgress = reclaimBytes > 0
    ? Math.max(0.04, Math.min(1, reclaimBytes / Math.max(totalRssBytes, 1)))
    : 0;
  return (
    <div className="resource-orb" aria-label={`预计可释放 ${formatBytes(reclaimBytes)}，CPU ${formatPercent(cpuPercent)}%，内存 ${formatPercent(memoryPercent)}%`}>
      <span className="resource-orb__glow" aria-hidden="true" />
      <span className="resource-orb__orbit resource-orb__orbit--outer" aria-hidden="true" />
      <span className="resource-orb__orbit resource-orb__orbit--inner" aria-hidden="true" />
      <svg className="resource-orb__gauge" viewBox="0 0 300 300" aria-hidden="true">
        <circle className="resource-orb__track" cx="150" cy="150" r="118" />
        <circle
          className="resource-orb__progress"
          cx="150"
          cy="150"
          r="118"
          style={{ "--orb-progress": orbitProgress } as CSSProperties}
        />
      </svg>
      <span className="resource-orb__satellite resource-orb__satellite--one" aria-hidden="true"><Cpu size={15} /></span>
      <span className="resource-orb__satellite resource-orb__satellite--two" aria-hidden="true"><MemoryStick size={15} /></span>
      <span className="resource-orb__satellite resource-orb__satellite--three" aria-hidden="true"><BrainCircuit size={15} /></span>

      <div className="resource-orb__center">
        <span>预计可释放</span>
        <strong>{formatBytes(reclaimBytes)}</strong>
        <small>{reclaimCount} 个可分离进程</small>
        <i><span style={{ width: `${Math.max(12, confidence)}%` }} /></i>
      </div>

      <div className="orb-stat orb-stat--cpu">
        <span>CPU</span><strong>{formatPercent(cpuPercent)}%</strong>
      </div>
      <div className="orb-stat orb-stat--memory">
        <span>内存</span><strong>{formatPercent(memoryPercent)}%</strong>
      </div>
      <div className="orb-stat orb-stat--attention">
        <span>关注</span><strong>{attentionCount}</strong>
      </div>
    </div>
  );
}

function MetricCard({
  icon: Icon,
  label,
  value,
  detail,
  meter,
  tone,
}: {
  icon: typeof Cpu;
  label: string;
  value: string;
  detail: string;
  meter: number;
  tone: "mint" | "blue" | "violet" | "coral" | "warning";
}) {
  return (
    <article className={`metric-card metric-card--${tone}`}>
      <div className="metric-card__head">
        <span><Icon size={17} aria-hidden="true" /></span>
        <small>{label}</small>
      </div>
      <strong className="metric-card__value">{value}</strong>
      <p>{detail}</p>
      <span className="metric-card__meter" aria-hidden="true"><i style={{ width: `${Math.min(100, Math.max(4, meter))}%` }} /></span>
    </article>
  );
}

function TasksView({
  workloads,
  allWorkloads,
  selectedId,
  query,
  kindFilter,
  onQuery,
  onKindFilter,
  onSelect,
  onPreview,
}: {
  workloads: Workload[];
  allWorkloads: Workload[];
  selectedId: string | null;
  query: string;
  kindFilter: string;
  onQuery: (value: string) => void;
  onKindFilter: (value: string) => void;
  onSelect: (id: string) => void;
  onPreview: (workload: Workload) => void;
}) {
  const kinds = Array.from(new Set(allWorkloads.map((workload) => workload.kind)));
  return (
    <div className="view-enter compact-view">
      <div className="page-intro">
        <div>
          <span className="section-kicker">任务归因</span>
          <h2>每个进程，都能找到归属。</h2>
          <p>按任务聚合零散 PID。展开后仍可核对每个进程，而不会一上来就铺满命令行。</p>
        </div>
        <div className="task-toolbar">
          <label className="search-field">
            <Search size={17} aria-hidden="true" />
            <span className="sr-only">搜索任务</span>
            <input value={query} onChange={(event) => onQuery(event.target.value)} placeholder="搜索任务、端口或命令…" />
          </label>
          <select aria-label="按任务类型筛选" value={kindFilter} onChange={(event) => onKindFilter(event.target.value)}>
            <option value="all">全部类型</option>
            {kinds.map((kind) => <option value={kind} key={kind}>{kindLabel(kind)}</option>)}
          </select>
        </div>
      </div>
      {workloads.length ? (
        <HeatTree workloads={workloads} selectedId={selectedId} onSelect={onSelect} onPreview={onPreview} />
      ) : (
        <div className="empty-state panel"><Search size={28} aria-hidden="true" /><strong>没有匹配的任务</strong><p>清除搜索或切换任务类型后再试。</p></div>
      )}
    </div>
  );
}

function LifecycleView({ snapshot, selectedId, onSelect }: { snapshot: Snapshot; selectedId: string | null; onSelect: (id: string) => void }) {
  return (
    <div className="view-enter lifecycle-view">
      <div className="page-intro">
        <div>
          <span className="section-kicker">为什么还在运行</span>
          <h2>看懂任务从开始到现在的轨迹。</h2>
          <p>时间轴越靠左，任务存活得越久。重复后代和长时间低活动会在这里显现。</p>
        </div>
        <div className="lifecycle-summary"><Cpu size={18} aria-hidden="true" /><span><strong>{snapshot.summary.attentionCount}</strong> 个任务组需要关注</span></div>
      </div>
      <div className="lifecycle-grid">
        <LifecycleRiver workloads={snapshot.workloads} events={snapshot.events} selectedId={selectedId} onSelect={onSelect} />
        <LifecycleFacts snapshot={snapshot} />
      </div>
      <AuditLog events={snapshot.events} limit={20} />
    </div>
  );
}

function LifecycleFacts({ snapshot }: { snapshot: Snapshot }) {
  const reclaimable = snapshot.workloads.filter((workload) => workload.reclaimable);
  const oldest = [...snapshot.workloads].sort((a, b) => b.ageSeconds - a.ageSeconds)[0];
  return (
    <section className="panel facts-panel" aria-labelledby="facts-title">
      <header className="panel-heading"><div><span className="section-kicker">证据摘要</span><h2 id="facts-title">为什么它们还活着</h2></div></header>
      <div className="fact-block fact-block--danger">
        <span className="fact-number metric-number">{reclaimable.length}</span>
        <div><strong>个任务组满足回收证据</strong><p>重复 MCP、父会话退出或长时间低活动，三者至少命中一项。</p></div>
      </div>
      {oldest && (
        <div className="fact-block">
          <span className="fact-number metric-number">{formatAge(oldest.ageSeconds)}</span>
          <div><strong>{oldest.name}</strong><p>当前存活最久的任务组，采样状态：{oldest.lastActivity}。</p></div>
        </div>
      )}
      <div className="fact-rule"><ShieldCheck size={17} aria-hidden="true" /><span>只把明确可分离的 MCP 后代放入计划；IDE、Shell、Agent 根和容器保持保护。</span></div>
      <details className="evidence-details">
        <summary>查看全部证据</summary>
        {snapshot.workloads.map((workload) => (
          <div key={workload.id}><strong>{workload.name}</strong><ul>{workload.evidence.map((evidence) => <li key={evidence.detail}>{evidence.label}：{evidence.detail}</li>)}</ul></div>
        ))}
      </details>
    </section>
  );
}

function AuditView({ snapshot }: { snapshot: Snapshot }) {
  return (
    <div className="view-enter audit-view">
      <div className="page-intro">
        <div>
          <span className="section-kicker">本地回执</span>
          <h2>每一次动作，都留下记录。</h2>
          <p>回收动作会记录令牌、受影响 PID 和最终结果。所有记录只保存在本机。</p>
        </div>
        <div className="capability-chips">
          <span><ShieldCheck size={14} /> 本机处理</span>
          <span className={snapshot.capabilities.docker ? "is-on" : ""}>Docker {snapshot.capabilities.docker ? "已连接" : "不可用"}</span>
          <span>{snapshot.capabilities.platform}</span>
        </div>
      </div>
      <div className="audit-layout">
        <AuditLog events={snapshot.events} limit={30} />
        <section className="panel boundary-panel" aria-labelledby="boundary-title">
          <header className="panel-heading"><div><span className="section-kicker">信任边界</span><h2 id="boundary-title">动作边界</h2></div></header>
          <ul className="boundary-list">
            <li><ShieldCheck size={18} /><span><strong>SIGTERM 优先</strong><small>不会自动升级为 SIGKILL。</small></span></li>
            <li><ShieldCheck size={18} /><span><strong>身份二次校验</strong><small>执行前比对 PID 与启动时间，防止 PID 复用。</small></span></li>
            <li><ShieldCheck size={18} /><span><strong>单次令牌</strong><small>预览 2 分钟后过期，成功执行后立即作废。</small></span></li>
            <li><ShieldCheck size={18} /><span><strong>关键进程保护</strong><small>IDE、Shell、Agent 根和容器不会进入批量停止。</small></span></li>
          </ul>
        </section>
      </div>
    </div>
  );
}

function DashboardSkeleton() {
  return (
    <div className="dashboard-skeleton" role="status" aria-label="正在读取本机任务">
      <div className="skeleton-hero"><span /><span /><span /></div>
      <div className="skeleton-metrics">{Array.from({ length: 4 }, (_, index) => <i key={index} />)}</div>
      <div className="skeleton-grid"><i /><i /></div>
    </div>
  );
}

function EmptyState({ onDemo }: { onDemo: () => void }) {
  return (
    <div className="empty-dashboard panel">
      <PanelLeftClose size={30} aria-hidden="true" />
      <strong>没有可显示的任务快照</strong>
      <p>实时采集失败时可以打开确定性演示，检查完整交互流程。</p>
      <button type="button" className="primary-button" onClick={onDemo}>打开演示数据</button>
    </div>
  );
}

function workloadIcon(kind: string) {
  switch (kind) {
    case "agent":
      return BrainCircuit;
    case "container":
      return Container;
    case "server":
      return Activity;
    default:
      return Boxes;
  }
}

function kindLabel(kind: string) {
  switch (kind) {
    case "agent": return "Agent 会话";
    case "mcp": return "MCP 进程组";
    case "server": return "开发服务";
    case "container": return "Docker 容器";
    case "build": return "构建任务";
    case "ide": return "IDE";
    default: return kind || "本地任务";
  }
}

export default App;
