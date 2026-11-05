import type { CSSProperties } from "react";
import { Clock3, RadioTower } from "lucide-react";
import type { Event as ActivityEvent, Workload } from "../mygo";
import { formatAge, formatTime, signalTone } from "../lib/format";

interface LifecycleRiverProps {
  workloads: Workload[];
  events: ActivityEvent[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}

export function LifecycleRiver({ workloads, events, selectedId, onSelect }: LifecycleRiverProps) {
  const sorted = [...workloads].sort((a, b) => b.ageSeconds - a.ageSeconds);
  const maxAge = Math.max(...sorted.map((workload) => workload.ageSeconds), 1);
  const selected = sorted.find((workload) => workload.id === selectedId) ?? sorted[0];
  const selectedEvents = selected ? events.filter((event) => event.workloadId === selected.id).slice(0, 2) : [];

  return (
    <section className="panel river-panel" aria-labelledby="river-title">
      <header className="panel-heading">
        <div>
          <p className="eyebrow">LIFECYCLE RIVER</p>
          <h2 id="river-title">生命周期河流</h2>
        </div>
        <span className="panel-kicker"><Clock3 size={13} aria-hidden="true" /> 现在 ← 过去 {formatAge(maxAge)}</span>
      </header>

      <div className="river-axis" aria-hidden="true">
        <span>过去</span>
        <span>现在</span>
      </div>
      <div className="river-lanes" aria-label="任务生命周期时间线">
        {sorted.map((workload, index) => {
          const start = Math.max(4, 100 - (workload.ageSeconds / maxAge) * 92);
          const tone = signalTone(workload.risk);
          const isSelected = selected?.id === workload.id;
          return (
            <button
              type="button"
              className={`river-lane ${isSelected ? "is-selected" : ""}`}
              key={workload.id}
              style={{ "--lane-index": index, "--lane-start": `${start}%` } as CSSProperties}
              onClick={() => onSelect(workload.id)}
              aria-label={`${workload.name}，持续 ${formatAge(workload.ageSeconds)}，${workload.riskLabel}`}
            >
              <span className="river-lane__label">
                <strong>{workload.name.split(" · ")[0]}</strong>
                <small>{formatAge(workload.ageSeconds)}</small>
              </span>
              <span className="river-lane__track" aria-hidden="true">
                <i className={`river-lane__bar river-lane__bar--${tone}`} style={{ left: `${start}%`, width: `${100 - start}%` }} />
                {workload.processes.slice(1, 4).map((process, processIndex) => (
                  <i
                    className={`river-node river-node--${tone}`}
                    key={process.id}
                    style={{ left: `${Math.min(94, start + 12 + processIndex * 22)}%` }}
                  />
                ))}
                {workload.risk === "reclaim" && <i className="river-node river-node--alert" style={{ left: "82%" }} />}
              </span>
            </button>
          );
        })}
      </div>

      {selected && (
        <div className="river-insight" role="status">
          <span className={`signal-glyph signal-glyph--${signalTone(selected.risk)}`} aria-hidden="true">
            <RadioTower size={16} />
          </span>
          <div>
            <strong>{selected.name}</strong>
            <p>
              {selected.riskDetail}
              {selectedEvents.length > 0 && <span className="river-event-time"> · {formatTime(selectedEvents[0].time)} 记录</span>}
            </p>
          </div>
        </div>
      )}

      <details className="river-text-fallback">
        <summary>文本时间线</summary>
        <ul>
          {sorted.map((workload) => (
            <li key={workload.id}>
              <span className="metric-number">{formatAge(workload.ageSeconds)}</span>
              {workload.name}，{workload.riskLabel}
            </li>
          ))}
        </ul>
      </details>
    </section>
  );
}
