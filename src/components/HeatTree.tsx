import type { CSSProperties } from "react";
import {
  Boxes,
  BrainCircuit,
  ChevronRight,
  Container,
  HardDrive,
  LockKeyhole,
  Server,
  ShieldAlert,
  ShieldCheck,
  TerminalSquare,
  TriangleAlert,
} from "lucide-react";
import type { ProcessNode, Workload } from "../mygo";
import { formatAge, formatBytes, formatPercent, kindLabel, signalTone } from "../lib/format";

interface HeatTreeProps {
  workloads: Workload[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  onPreview: (workload: Workload) => void;
}

const icons = {
  agent: BrainCircuit,
  mcp: TerminalSquare,
  server: Server,
  container: Container,
  build: Boxes,
  ide: HardDrive,
};

const riskIcons = {
  danger: ShieldAlert,
  attention: TriangleAlert,
  healthy: ShieldCheck,
  neutral: Boxes,
};

function Sparkline({ values, risk }: { values: number[]; risk: string }) {
  if (!values.length) return null;
  const max = Math.max(...values, 1);
  const points = values
    .map((value, index) => `${(index / Math.max(values.length - 1, 1)) * 72},${24 - (value / max) * 20}`)
    .join(" ");
  return (
    <svg className={`sparkline sparkline--${signalTone(risk)}`} viewBox="0 0 72 28" role="img" aria-label="最近资源趋势">
      <polyline points={points} fill="none" stroke="currentColor" strokeWidth="1.8" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

function ProcessRows({ processes }: { processes: ProcessNode[] }) {
  return (
    <div className="process-tree" role="tree" aria-label="任务进程树">
      {processes.map((process) => {
        const tone = process.protected ? "protected" : process.kind === "mcp" ? "mcp" : "normal";
        return (
          <div
            className={`process-row process-row--${tone}`}
            key={process.id}
            role="treeitem"
            aria-level={process.depth + 1}
            tabIndex={0}
          >
            <span className="process-branch" style={{ "--depth": process.depth } as CSSProperties} aria-hidden="true" />
            <span className="process-pid metric-number">{process.pid}</span>
            <span className="process-name">
              {process.protected && <LockKeyhole size={13} aria-label="受保护" />}
              {process.name}
              <small>{process.command}</small>
            </span>
            <span className="process-meta metric-number">{formatPercent(process.cpuPercent)}%</span>
            <span className="process-meta process-meta--rss metric-number">{formatBytes(process.rssBytes)}</span>
            <span className={`node-kind node-kind--${tone}`}>{process.protected ? "受保护" : kindLabel(process.kind)}</span>
          </div>
        );
      })}
    </div>
  );
}

export function HeatTree({ workloads, selectedId, onSelect, onPreview }: HeatTreeProps) {
  const sorted = [...workloads].sort((a, b) => b.rssBytes - a.rssBytes);
  const maxRSS = Math.max(...sorted.map((workload) => workload.rssBytes), 1);

  return (
    <section className="panel heat-panel" aria-labelledby="heat-title">
      <header className="panel-heading">
        <div>
          <p className="eyebrow">TASK HEAT TREE</p>
          <h2 id="heat-title">任务热力树</h2>
        </div>
        <div className="legend" aria-label="图例">
          <span><i className="legend-dot legend-dot--rss" />面积 = RSS</span>
          <span><i className="legend-dot legend-dot--cpu" />明度 = CPU</span>
        </div>
      </header>

      {sorted.length === 0 ? (
        <div className="empty-state">
          <ShieldCheck size={26} aria-hidden="true" />
          <strong>没有发现活跃的 AI 或开发任务</strong>
          <p>后台保持安静。任务出现时会自动归因到这里。</p>
        </div>
      ) : (
        <div className="heat-list">
          {sorted.map((workload) => {
            const Icon = icons[workload.kind as keyof typeof icons] ?? Boxes;
            const RiskIcon = riskIcons[signalTone(workload.risk)];
            const selected = selectedId === workload.id;
            return (
              <article className={`heat-task ${selected ? "is-selected" : ""}`} key={workload.id}>
                <div className="heat-task__row">
                  <button
                    type="button"
                    className="heat-task__select"
                    aria-expanded={selected}
                    aria-label={`${workload.name}，占用 ${formatBytes(workload.rssBytes)}，${workload.riskLabel}`}
                    onClick={() => onSelect(workload.id)}
                  >
                    <ChevronRight className="heat-task__chevron" size={15} aria-hidden="true" />
                    <span className={`task-icon task-icon--${signalTone(workload.risk)}`}>
                      <Icon size={17} aria-hidden="true" />
                    </span>
                    <span className="heat-task__identity">
                      <strong>{workload.name}</strong>
                      <small>{workload.source} · {kindLabel(workload.kind)} · {formatAge(workload.ageSeconds)}</small>
                    </span>
                    <span className="heat-task__trend"><Sparkline values={workload.trend} risk={workload.risk} /></span>
                    <span className="heat-task__numbers">
                      <strong className="metric-number">{formatBytes(workload.rssBytes)}</strong>
                      <small className="metric-number">{formatPercent(workload.cpuPercent)}% CPU · {workload.processCount} 进程</small>
                    </span>
                    <span className={`risk-badge risk-badge--${signalTone(workload.risk)}`}>
                      <RiskIcon size={13} aria-hidden="true" />
                      {workload.riskLabel}
                    </span>
                  </button>
                  {workload.reclaimable ? (
                    <button
                      type="button"
                      className="row-action"
                      onClick={() => onPreview(workload)}
                      aria-label={`预览 ${workload.name} 的安全回收影响范围`}
                    >
                      预览回收
                    </button>
                  ) : (
                    <span className="row-action row-action--protected" title={workload.protectedReason || "保留运行"}>
                      <LockKeyhole size={13} aria-hidden="true" /> 保留
                    </span>
                  )}
                </div>
                <div className="heat-task__bar" aria-hidden="true">
                  <span
                    className={`heat-task__fill heat-task__fill--${signalTone(workload.risk)}`}
                    style={{
                      width: `${Math.max(4, (workload.rssBytes / maxRSS) * 100)}%`,
                      "--cpu-warmth": Math.min(1, workload.cpuPercent / 30),
                    } as CSSProperties}
                  />
                </div>
                {selected && (
                  <div className="heat-task__details">
                    <div className="detail-note">
                      <span className={`risk-mark risk-mark--${signalTone(workload.risk)}`}>
                        <RiskIcon size={15} aria-hidden="true" />
                      </span>
                      <p><strong>{workload.riskLabel}</strong>{workload.riskDetail}</p>
                      <span className="confidence metric-number">置信度 {workload.confidence}%</span>
                    </div>
                    <ProcessRows processes={workload.processes} />
                  </div>
                )}
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
