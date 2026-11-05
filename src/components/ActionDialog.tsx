import { useEffect, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import {
  AlertTriangle,
  Check,
  CircleCheck,
  Clock3,
  LoaderCircle,
  LockKeyhole,
  ShieldCheck,
  TerminalSquare,
  X,
} from "lucide-react";
import { isMyGo } from "mygo-runtime";
import type { ActionPlan, ActionResult, Workload } from "../mygo";
import { TaskManager } from "../mygo";
import { formatBytes } from "../lib/format";
import { makeLocalPreview, makeLocalResult } from "../lib/preview";

interface ActionDialogProps {
  workload: Workload | null;
  mode: "live" | "demo";
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCompleted: (result: ActionResult) => void;
}

export function ActionDialog({ workload, mode, open, onOpenChange, onCompleted }: ActionDialogProps) {
  const [plan, setPlan] = useState<ActionPlan | null>(null);
  const [result, setResult] = useState<ActionResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);

  useEffect(() => {
    if (!open || !workload) return;
    let active = true;
    setPlan(null);
    setResult(null);
    setError(null);
    setLoading(true);

    const load = async () => {
      try {
        const nextPlan = isMyGo()
          ? await TaskManager.preview(workload.id, mode === "demo")
          : makeLocalPreview(workload);
        if (active) setPlan(nextPlan);
      } catch (reason) {
        if (active) setError(reason instanceof Error ? reason.message : "无法生成回收预览");
      } finally {
        if (active) setLoading(false);
      }
    };
    void load();

    return () => {
      active = false;
    };
  }, [open, workload, mode]);

  const execute = async () => {
    if (!plan) return;
    setExecuting(true);
    setError(null);
    try {
      const nextResult = isMyGo() ? await TaskManager.execute(plan.token) : makeLocalResult(plan);
      setResult(nextResult);
      onCompleted(nextResult);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "执行失败，请重新预览");
    } finally {
      setExecuting(false);
    }
  };

  const close = () => {
    if (executing) return;
    onOpenChange(false);
  };

  return (
    <Dialog.Root open={open} onOpenChange={(nextOpen) => { if (!executing) onOpenChange(nextOpen); }}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay" />
        <Dialog.Content className="dialog-content" aria-describedby={undefined}>
          <div className="dialog-topline">
            <span className="dialog-kicker">{result ? "ACTION RECEIPT" : "BLAST RADIUS"}</span>
            <Dialog.Close asChild>
              <button type="button" className="icon-button" aria-label="关闭" disabled={executing}><X size={18} /></button>
            </Dialog.Close>
          </div>

          {loading ? (
            <div className="dialog-loading" role="status">
              <LoaderCircle className="spin" size={24} aria-hidden="true" />
              <strong>正在核对 PID 与启动时间</strong>
              <p>预览只读，不会停止任何进程。</p>
            </div>
          ) : result ? (
            <div className="result-view">
              <div className={`result-hero ${result.failed.length ? "result-hero--attention" : "result-hero--success"}`}>
                <span className="result-icon"><CircleCheck size={25} aria-hidden="true" /></span>
                <div>
                  <Dialog.Title>{result.failed.length ? "回收部分完成" : "安全回收完成"}</Dialog.Title>
                  <Dialog.Description>{result.message}</Dialog.Description>
                </div>
              </div>
              <div className="plan-stats">
                <div><span>已退出</span><strong className="metric-number">{result.stopped.length}</strong><small>个 PID</small></div>
                <div><span>预计释放</span><strong className="metric-number">{formatBytes(result.reclaimedBytes)}</strong><small>已回到系统</small></div>
                <div><span>失败 / 待退出</span><strong className="metric-number">{result.failed.length + result.pending.length}</strong><small>需要复核</small></div>
              </div>
              <div className="receipt-panel">
                <div className="receipt-header"><ShieldCheck size={15} aria-hidden="true" /> 逐 PID 审计</div>
                <div className="receipt-grid">
                  {result.stopped.map((pid) => (
                    <span className="receipt-pid receipt-pid--ok" key={pid}><Check size={12} aria-hidden="true" /> {pid}</span>
                  ))}
                  {result.pending.map((pid) => (
                    <span className="receipt-pid receipt-pid--pending" key={pid}><Clock3 size={12} aria-hidden="true" /> {pid}</span>
                  ))}
                  {result.failed.map((failure) => (
                    <span className="receipt-pid receipt-pid--failed" key={failure.pid} title={failure.reason}>
                      <AlertTriangle size={12} aria-hidden="true" /> {failure.pid}
                    </span>
                  ))}
                </div>
              </div>
              <div className="dialog-actions">
                <button type="button" className="primary-button" onClick={close}>完成并回到概览</button>
              </div>
            </div>
          ) : error ? (
            <div className="dialog-error">
              <AlertTriangle size={23} aria-hidden="true" />
              <Dialog.Title>暂时不能执行回收</Dialog.Title>
              <Dialog.Description>{error}</Dialog.Description>
              <div className="dialog-actions">
                <button type="button" className="secondary-button" onClick={close}>关闭</button>
                <button type="button" className="primary-button" onClick={() => onOpenChange(false)}>刷新后重试</button>
              </div>
            </div>
          ) : plan && workload ? (
            <div className="plan-view">
              <div className="dialog-title-row">
                <span className="plan-emblem"><TerminalSquare size={21} aria-hidden="true" /></span>
                <div>
                  <Dialog.Title>爆炸半径预览</Dialog.Title>
                  <Dialog.Description>{plan.workloadName} · 只发送 SIGTERM</Dialog.Description>
                </div>
              </div>

              <div className="plan-stats">
                <div><span>将停止</span><strong className="metric-number">{plan.targets.length}</strong><small>个 MCP 后代</small></div>
                <div><span>受保护</span><strong className="metric-number">{plan.protected.length}</strong><small>个关键进程</small></div>
                <div><span>预计释放</span><strong className="metric-number">{formatBytes(plan.estimatedRssBytes)}</strong><small>RSS 估算</small></div>
              </div>

              <div className="impact-map" aria-label="回收影响范围">
                <span className="impact-node impact-node--protected"><LockKeyhole size={14} /> Agent 根</span>
                <span className="impact-link" aria-hidden="true" />
                <span className="impact-node impact-node--target">{plan.targets.length} 个 MCP 后代</span>
                <span className="impact-link" aria-hidden="true" />
                <span className="impact-node impact-node--keep">IDE / Shell 保留</span>
              </div>

              <div className="impact-columns">
                <section className="impact-section" aria-labelledby="targets-title">
                  <h3 id="targets-title"><span className="status-dot status-dot--target" /> 将停止 <b>{plan.targets.length}</b></h3>
                  <div className="impact-list">
                    {plan.targets.map((target) => (
                      <div className="impact-target" key={target.pid}>
                        <span className="metric-number">{target.pid}</span>
                        <span>{target.name}</span>
                        <small className="metric-number">{formatBytes(target.rssBytes)}</small>
                      </div>
                    ))}
                  </div>
                </section>
                <section className="impact-section" aria-labelledby="protected-title">
                  <h3 id="protected-title"><span className="status-dot status-dot--protected" /> 受保护 <b>{plan.protected.length}</b></h3>
                  <div className="impact-list">
                    {plan.protected.map((process) => (
                      <div className="impact-protected" key={`${process.pid}-${process.name}`}>
                        <LockKeyhole size={13} aria-hidden="true" />
                        <span className="metric-number">{process.pid}</span>
                        <span>{process.name}</span>
                        <small>{process.reason}</small>
                      </div>
                    ))}
                    {(plan.containerNames ?? []).map((name) => (
                      <div className="impact-protected" key={name}>
                        <LockKeyhole size={13} aria-hidden="true" />
                        <span>{name}</span>
                        <small>容器操作不在本次计划中</small>
                      </div>
                    ))}
                  </div>
                </section>
              </div>

              <ul className="safety-notes">
                {plan.warnings.map((warning) => <li key={warning}><ShieldCheck size={13} aria-hidden="true" />{warning}</li>)}
              </ul>

              <div className="dialog-actions">
                <button type="button" className="secondary-button" onClick={close}>取消</button>
                <button type="button" className="danger-button" onClick={() => void execute()} disabled={executing}>
                  {executing ? <LoaderCircle className="spin" size={16} aria-hidden="true" /> : <ShieldCheck size={16} aria-hidden="true" />}
                  {executing ? "正在优雅停止…" : `确认停止 ${plan.targets.length} 个进程`}
                </button>
              </div>
              <p className="dialog-footnote">操作令牌将在 2 分钟后过期；本次确认只能使用一次。</p>
            </div>
          ) : null}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
