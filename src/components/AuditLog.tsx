import { Activity, CircleCheck, CircleDot, ShieldAlert, TriangleAlert } from "lucide-react";
import type { Event } from "../mygo";
import { formatShortDate } from "../lib/format";

interface AuditLogProps {
  events: Event[];
  limit?: number;
}

export function AuditLog({ events, limit = 5 }: AuditLogProps) {
  const visible = events.slice(0, limit);
  return (
    <section className="panel audit-panel" aria-labelledby="audit-title">
      <header className="panel-heading">
        <div>
          <p className="eyebrow">AUDIT STREAM</p>
          <h2 id="audit-title">现场记录</h2>
        </div>
        <span className="panel-kicker"><Activity size={13} aria-hidden="true" /> 本地审计</span>
      </header>
      {visible.length === 0 ? (
        <div className="audit-empty"><CircleDot size={16} aria-hidden="true" /> 暂无异常事件</div>
      ) : (
        <ol className="audit-list">
          {visible.map((event) => {
            const Icon = event.tone === "danger" ? ShieldAlert : event.tone === "attention" ? TriangleAlert : CircleCheck;
            return (
              <li className={`audit-item audit-item--${event.tone}`} key={event.id}>
                <span className="audit-icon" aria-hidden="true"><Icon size={14} /></span>
                <div>
                  <div className="audit-title-row">
                    <strong>{event.title}</strong>
                    <time className="metric-number" dateTime={new Date(event.time).toISOString()}>{formatShortDate(event.time)}</time>
                  </div>
                  <p>{event.detail}</p>
                  <small>{event.workloadName}</small>
                </div>
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}
