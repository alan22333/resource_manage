import type { ActionResult, Snapshot, Workload } from "../mygo";
import { formatBytes } from "./format";

export function applyLocalResult(snapshot: Snapshot, result: ActionResult): Snapshot {
  const workloads = snapshot.workloads.filter((workload) => workload.id !== result.workloadId);
  const removed = snapshot.workloads.find((workload) => workload.id === result.workloadId);
  const totalRss = Math.max(0, snapshot.summary.totalRssBytes - (removed?.rssBytes ?? result.reclaimedBytes));
  const reclaimBytes = Math.max(0, snapshot.summary.reclaimBytes - (removed?.rssBytes ?? result.reclaimedBytes));
  const top = [...workloads].sort((a, b) => b.rssBytes - a.rssBytes)[0];
  const resolvedThermalCause = Boolean(
    snapshot.thermal.causeWorkloadId && snapshot.thermal.causeWorkloadId === result.workloadId,
  );
  const event = {
    id: `audit-${result.finishedAt}`,
    time: result.finishedAt,
    kind: "reclaimed",
    workloadId: result.workloadId,
    workloadName: result.workloadName,
    title: "安全回收已记录",
    detail: result.message,
    tone: "healthy",
  };
  return {
    ...snapshot,
    generatedAt: result.finishedAt,
    revision: `local-${result.finishedAt}`,
    summary: {
      ...snapshot.summary,
      pressureLabel: workloads.length ? snapshot.summary.pressureLabel : "演示中的异常任务已回收",
      pressureDetail: top ? `${top.name} 成为当前最大的任务组` : "没有发现可回收的残留任务组",
      topWorkloadId: top?.id ?? "",
      topWorkloadName: top?.name ?? "",
      reclaimBytes,
      reclaimProcessCount: Math.max(0, snapshot.summary.reclaimProcessCount - result.stopped.length),
      totalRssBytes: totalRss,
      activeWorkloads: workloads.length,
      attentionCount: workloads.filter((workload) => workload.risk === "attention" || workload.risk === "reclaim").length,
      reclaimCount: workloads.filter((workload) => workload.reclaimable).length,
    },
    system: {
      ...snapshot.system,
      memoryUsed: Math.max(0, snapshot.system.memoryUsed - result.reclaimedBytes),
    },
    thermal: resolvedThermalCause
      ? {
          ...snapshot.thermal,
          state: "nominal",
          available: true,
          active: false,
          recovering: false,
          startedAt: 0,
          durationSeconds: 0,
          cause: "演示中的热压力已随残留后代回收而回落",
          causeWorkloadId: "",
          causeWorkloadName: "",
          evidence: [
            { label: "热压力", detail: "Nominal · 系统回到稳定区间", tone: "healthy" },
            { label: "回收回执", detail: `已释放 ${formatBytes(result.reclaimedBytes)}，逐 PID 记录已写入审计`, tone: "healthy" },
          ],
          recommendation: "无需处理；后台巡检会继续观察任务与系统热压力。",
          observedAt: result.finishedAt,
        }
      : snapshot.thermal,
    workloads,
    events: [event, ...snapshot.events],
  };
}

export function findPrimaryWorkload(snapshot: Snapshot): Workload | undefined {
  return snapshot.workloads.find((workload) => workload.id === snapshot.summary.topWorkloadId)
    ?? [...snapshot.workloads].sort((a, b) => b.rssBytes - a.rssBytes)[0];
}

export function findReclaimable(snapshot: Snapshot): Workload | undefined {
  return snapshot.workloads.find((workload) => workload.reclaimable);
}

export function describeWorkloadHeadline(workload?: Workload): string {
  if (!workload) return "后台正在安静运行";
  const mcpDescendants = Math.max(0, workload.processCount - 1);
  if (mcpDescendants > 0 && (workload.risk === "attention" || workload.risk === "reclaim")) {
    return `“${workload.name}” 留下了 ${mcpDescendants} 个 MCP 后代`;
  }
  if (mcpDescendants > 0) {
    return `“${workload.name}” 正在使用 ${mcpDescendants} 个 MCP 后代`;
  }
  return `“${workload.name}” 是当前最大的任务组`;
}

export interface OverviewVerdict {
  tone: "attention" | "calm";
  firstLine: string;
  emphasis: string;
}

export function describeOverviewVerdict(snapshot: Snapshot, hasReclaimTarget: boolean): OverviewVerdict {
  if (hasReclaimTarget) {
    return {
      tone: "attention",
      firstLine: `后台有 ${formatBytes(snapshot.summary.reclaimBytes)}`,
      emphasis: "可以安全释放。",
    };
  }

  // A high pressure label must never be paired with a contradictory "calm"
  // verdict. When no evidence-backed cleanup target exists, say so explicitly.
  if (/(偏高|较高|紧张|过高|吃紧)/.test(snapshot.summary.pressureLabel) || snapshot.summary.attentionCount > 0) {
    return {
      tone: "attention",
      firstLine: "资源偏紧，",
      emphasis: "暂无可安全释放。",
    };
  }

  return {
    tone: "calm",
    firstLine: "后台正在",
    emphasis: "安静运行。",
  };
}
