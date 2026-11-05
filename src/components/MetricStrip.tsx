import { Boxes, BrainCircuit, Container, Cpu, MemoryStick, Workflow } from "lucide-react";
import type { Summary, SystemMetrics } from "../mygo";
import { formatBytes, formatPercent } from "../lib/format";

interface MetricStripProps {
  system: SystemMetrics;
  summary: Summary;
}

export function MetricStrip({ system, summary }: MetricStripProps) {
  const memoryPercent = system.memoryTotal ? (system.memoryUsed / system.memoryTotal) * 100 : 0;
  const metrics = [
    {
      label: "CPU",
      value: `${formatPercent(system.cpuPercent)}%`,
      detail: `${system.processCount} 个进程`,
      icon: Cpu,
      meter: system.cpuPercent,
      tone: system.cpuPercent > 75 ? "danger" : system.cpuPercent > 50 ? "attention" : "accent",
    },
    {
      label: "统一内存",
      value: formatBytes(system.memoryUsed),
      detail: `共 ${formatBytes(system.memoryTotal)}`,
      icon: MemoryStick,
      meter: memoryPercent,
      tone: memoryPercent > 78 ? "danger" : "accent",
    },
    {
      label: "任务 RSS",
      value: formatBytes(summary.totalRssBytes),
      detail: `${summary.activeWorkloads} 个活跃任务`,
      icon: Boxes,
      meter: summary.totalRssBytes / Math.max(system.memoryTotal, 1) * 100,
      tone: "rss",
    },
    {
      label: "Agent 会话",
      value: String(system.agentSessions),
      detail: `${summary.attentionCount} 个需关注`,
      icon: BrainCircuit,
      meter: 0,
      tone: "neutral",
    },
    {
      label: "可回收",
      value: formatBytes(summary.reclaimBytes),
      detail: `${summary.reclaimProcessCount} 个进程`,
      icon: Workflow,
      meter: 0,
      tone: "reclaim",
    },
    {
      label: "容器",
      value: String(system.containerCount),
      detail: system.containerCount ? "Docker 已连接" : "未检测到",
      icon: Container,
      meter: 0,
      tone: "neutral",
    },
  ];

  return (
    <section className="metric-strip" aria-label="系统信号">
      {metrics.map(({ label, value, detail, icon: Icon, meter, tone }, index) => (
        <article className="metric-cell" key={label} style={{ "--metric-index": index } as React.CSSProperties}>
          <div className="metric-cell__head">
            <Icon size={15} aria-hidden="true" />
            <span>{label}</span>
          </div>
          <div className="metric-cell__value metric-number">{value}</div>
          <div className="metric-cell__detail">{detail}</div>
          {meter > 0 && (
            <div className={`metric-meter metric-meter--${tone}`} aria-hidden="true">
              <span style={{ width: `${Math.min(100, Math.max(1, meter))}%` }} />
            </div>
          )}
        </article>
      ))}
    </section>
  );
}
