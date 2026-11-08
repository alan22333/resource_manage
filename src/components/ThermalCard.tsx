import {
  ArrowRight,
  Clock3,
  Flame,
  ShieldCheck,
  Snowflake,
  ThermometerSun,
} from "lucide-react";
import type { ThermalSample, ThermalStatus, Workload } from "../mygo";

interface ThermalCardProps {
  thermal: ThermalStatus;
  workloads: Workload[];
  onInspect: (workloadId: string) => void;
  onPreview: (workload: Workload) => void;
}

const thermalCopy: Record<string, { label: string; title: string }> = {
  fair: { label: "Fair", title: "系统热压力开始升高" },
  serious: { label: "Serious", title: "系统正在变热，原因已关联到后台任务" },
  critical: { label: "Critical", title: "系统热压力严重，需要立即排查" },
};

function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "刚刚开始";
  const whole = Math.floor(seconds);
  const hours = Math.floor(whole / 3600);
  const minutes = Math.floor((whole % 3600) / 60);
  const remaining = whole % 60;
  if (hours > 0) return `${hours} 小时 ${minutes} 分`;
  if (minutes > 0) return `${minutes} 分 ${remaining} 秒`;
  return `${remaining} 秒`;
}

const thermalHistoryCopy: Record<string, string> = {
  nominal: "稳定",
  fair: "偏高",
  serious: "严重",
  critical: "临界",
  unavailable: "不可用",
};

function formatSampleTime(timestamp: number): string {
  if (!Number.isFinite(timestamp) || timestamp <= 0) return "未知时间";
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(new Date(timestamp));
}

function ThermalHistory({ history }: { history?: ThermalSample[] }) {
  const samples = (history ?? []).slice(-12);
  if (!samples.length) {
    return (
      <div className="thermal-history thermal-history--empty" aria-label="本次运行热压力采样">
        <span className="thermal-history__label">本次运行采样</span>
        <span>等待真实采样</span>
      </div>
    );
  }

  return (
    <div className="thermal-history" aria-label="本次运行热压力采样">
      <div className="thermal-history__heading">
        <span className="thermal-history__label">本次运行采样</span>
        <span>最近 {samples.length} 条 · 仅公开热压力等级</span>
      </div>
      <div className="thermal-history__track" role="list">
        {samples.map((sample, index) => {
          const label = thermalHistoryCopy[sample.state] ?? "不可用";
          const time = formatSampleTime(sample.observedAt);
          return (
            <span
              className={`thermal-history__sample thermal-history__sample--${sample.state}`}
              role="listitem"
              key={`${sample.observedAt}-${index}`}
              aria-label={`${label}，${time}`}
              title={`${label} · ${time}`}
            />
          );
        })}
      </div>
    </div>
  );
}

function thermalTone(thermal: ThermalStatus): string {
  if (thermal.recovering) return "recovering";
  if (thermal.state === "critical") return "critical";
  if (thermal.state === "serious") return "serious";
  return "fair";
}

export function ThermalCard({ thermal, workloads, onInspect, onPreview }: ThermalCardProps) {
  if (!thermal.available) {
    return (
      <section className="thermal-card thermal-card--unavailable" aria-labelledby="thermal-title" aria-live="polite">
        <span className="thermal-card__signal" aria-hidden="true"><ThermometerSun size={25} /></span>
        <div className="thermal-card__copy">
          <span className="thermal-card__kicker">系统热压力</span>
          <h2 id="thermal-title">此平台未提供公开热压力状态</h2>
          <p>应用不会猜测温度，仍会使用 CPU、内存与任务证据帮助排查。</p>
        </div>
        <span className="thermal-card__state">Unavailable</span>
        <ThermalHistory history={thermal.history} />
      </section>
    );
  }

  if (!thermal.active) {
    return (
      <section className="thermal-card thermal-card--nominal" aria-labelledby="thermal-title" aria-live="polite">
        <span className="thermal-card__signal" aria-hidden="true"><Snowflake size={24} /></span>
        <div className="thermal-card__copy">
          <span className="thermal-card__kicker">系统热压力</span>
          <h2 id="thermal-title">热压力稳定</h2>
          <p>{thermal.cause || "后台巡检持续观察，发现持续热压力时会自动关联任务。"}</p>
        </div>
        <span className="thermal-card__state">Nominal</span>
        <ThermalHistory history={thermal.history} />
      </section>
    );
  }

  const tone = thermalTone(thermal);
  const copy = thermal.recovering
    ? { label: "Recovering", title: "热压力已回落，正在确认是否稳定" }
    : thermalCopy[thermal.state] ?? thermalCopy.fair;
  const causeWorkload = thermal.causeWorkloadId
    ? workloads.find((workload) => workload.id === thermal.causeWorkloadId)
    : undefined;
  const canPreview = causeWorkload?.reclaimable === true;
  const evidence = thermal.evidence.slice(0, 3);

  return (
    <section
      className={`thermal-card thermal-card--active thermal-card--${tone}`}
      aria-labelledby="thermal-title"
      aria-live="polite"
    >
      <span className="thermal-card__signal" aria-hidden="true">
        {thermal.recovering ? <Snowflake size={25} /> : <Flame size={25} />}
      </span>

      <div className="thermal-card__copy">
        <div className="thermal-card__meta">
          <span className="thermal-card__kicker">系统热压力</span>
          <span className="thermal-card__state">{copy.label}</span>
          <span className="thermal-card__duration"><Clock3 size={14} aria-hidden="true" />已持续 {formatDuration(thermal.durationSeconds)}</span>
          <span className="thermal-card__confidence">归因置信度 {thermal.confidence}%</span>
        </div>
        <h2 id="thermal-title">{copy.title}</h2>
        <p>{thermal.cause}</p>
      </div>

      <div className="thermal-card__actions">
        {thermal.causeWorkloadId && (
          <button type="button" className="secondary-button" onClick={() => onInspect(thermal.causeWorkloadId)}>
            查看排查详情 <ArrowRight size={16} aria-hidden="true" />
          </button>
        )}
        {canPreview && causeWorkload && (
          <button type="button" className="primary-button" onClick={() => onPreview(causeWorkload)}>
            <ShieldCheck size={16} aria-hidden="true" /> 查看回收预览
          </button>
        )}
      </div>

      <div className="thermal-card__details">
        <ul className="thermal-evidence" aria-label="热压力证据">
          {evidence.map((item, index) => (
            <li className={`thermal-evidence__item thermal-evidence__item--${item.tone}`} key={`${item.label}-${index}`}>
              <span>{item.label}</span>
              <strong>{item.detail}</strong>
            </li>
          ))}
        </ul>
        <div className="thermal-card__recommendation">
          <span><ShieldCheck size={15} aria-hidden="true" /> 建议</span>
          <p>{thermal.recommendation}</p>
        </div>
      </div>
      <ThermalHistory history={thermal.history} />
    </section>
  );
}
