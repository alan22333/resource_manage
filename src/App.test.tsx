import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import fixture from "./fixtures/demo-snapshot.json";
import type { Snapshot, Workload } from "./mygo";
import { HeatTree } from "./components/HeatTree";
import { ThermalCard } from "./components/ThermalCard";
import { applyLocalResult, describeOverviewVerdict, describeWorkloadHeadline } from "./lib/snapshot";
import App from "./App";
import stylesheet from "./style.css?raw";

vi.mock("mygo-runtime", () => ({
  isMyGo: () => false,
}));

const demoSnapshot = fixture as unknown as Snapshot;
const oldBranch = demoSnapshot.workloads.find((workload) => workload.id === "demo-codex-old-branch") as Workload;

beforeEach(() => {
  localStorage.clear();
  document.documentElement.dataset.theme = "dark";
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("AI task manager acceptance", () => {
  it("puts the verdict, reclaim visualization, and safety boundary first", () => {
    render(<App />);

    expect(screen.getByText("演示数据")).toBeInTheDocument();
    expect(screen.getByText(/所有 PID 与资源均为合成数据/)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /后台有/ })).toBeInTheDocument();
    expect(screen.getByText("内存压力偏高")).toBeInTheDocument();
    expect(screen.getByLabelText(/预计可释放/)).toBeInTheDocument();
    expect(screen.getByText("单次令牌")).toBeInTheDocument();
    expect(screen.getAllByText(/疑似残留/).length).toBeGreaterThan(0);
    expect(screen.getByRole("heading", { name: /系统正在变热/ })).toBeInTheDocument();
    expect(screen.getByText("Serious")).toBeInTheDocument();
    expect(screen.getByLabelText("本次运行热压力采样")).toBeInTheDocument();
    expect(within(screen.getByLabelText("本次运行热压力采样")).getAllByRole("listitem")).toHaveLength(8);
    expect(document.body.textContent).not.toMatch(/[°℃]/);

    // Secondary evidence views stay out of the calm overview.
    expect(screen.queryByRole("heading", { name: "任务热力树" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "生命周期河流" })).not.toBeInTheDocument();
  });

  it("keeps task and process navigation keyboard reachable", () => {
    render(<App />);
    fireEvent.click(screen.getByRole("button", { name: "任务" }));

    expect(screen.getByRole("heading", { name: "任务热力树" })).toBeInTheDocument();
    const taskButton = screen.getByRole("button", { name: /^Codex · feature\/old-branch，占用/ });
    expect(taskButton).toHaveAttribute("aria-expanded", "true");

    fireEvent.click(taskButton);
    fireEvent.keyDown(taskButton, { key: "Enter", code: "Enter" });

    const processTree = screen.getByRole("tree", { name: "任务进程树" });
    const treeItems = within(processTree).getAllByRole("treeitem");
    expect(treeItems.length).toBeGreaterThan(1);
    expect(treeItems[0]).toHaveAttribute("tabindex", "0");
    expect(treeItems[0]).toHaveAttribute("aria-level", "1");
    expect(screen.getAllByLabelText("受保护").length).toBeGreaterThan(0);
  });

  it("previews blast radius, separates targets from protected processes, and returns a receipt", async () => {
    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: /查看安全回收/ }));

    const dialog = await screen.findByRole("dialog");
    await waitFor(() => expect(within(dialog).getByText("爆炸半径预览")).toBeInTheDocument());

    expect(within(dialog).getAllByText("将停止").length).toBeGreaterThan(0);
    expect(within(dialog).getAllByText("受保护").length).toBeGreaterThan(0);
    expect(within(dialog).getByText("Agent 根")).toBeInTheDocument();
    expect(within(dialog).getByText("IDE / Shell 保留")).toBeInTheDocument();
    expect(within(dialog).getByText(/只发送 SIGTERM/)).toBeInTheDocument();

    const confirmation = within(dialog).getByRole("checkbox", { name: /我已核对影响范围/ });
    expect(confirmation).not.toBeChecked();
    expect(within(dialog).getByRole("button", { name: "请先勾选确认" })).toBeDisabled();

    fireEvent.click(confirmation);
    expect(confirmation).toBeChecked();
    fireEvent.click(within(dialog).getByRole("button", { name: /确认并停止 12 个进程/ }));

    await waitFor(() => expect(within(dialog).getByText("安全回收完成")).toBeInTheDocument());
    expect(within(dialog).getByText("逐 PID 审计")).toBeInTheDocument();
    expect(within(dialog).getByText("已退出")).toBeInTheDocument();
    expect(within(dialog).getByText("预计释放")).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: "完成并回到概览" }));
    await waitFor(() => expect(screen.getByRole("heading", { name: "热压力稳定" })).toBeInTheDocument());
    expect(screen.getByText("演示中的热压力已随残留后代回收而回落")).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/[°℃]/);
  });

  it("shows thermal evidence and routes inspection to the attributed task", () => {
    const onInspect = vi.fn();
    const onPreview = vi.fn();
    render(
      <ThermalCard
        thermal={demoSnapshot.thermal}
        workloads={demoSnapshot.workloads}
        onInspect={onInspect}
        onPreview={onPreview}
      />,
    );

    expect(screen.getByRole("heading", { name: /系统正在变热/ })).toBeInTheDocument();
    expect(screen.getByText(/旧分支仍有 12 个 MCP 后代/)).toBeInTheDocument();
    expect(screen.getByText("归因置信度 91%")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /查看排查详情/ }));
    expect(onInspect).toHaveBeenCalledWith("demo-codex-old-branch");

    fireEvent.click(screen.getByRole("button", { name: /查看回收预览/ }));
    expect(onPreview).toHaveBeenCalledWith(oldBranch);
  });

  it("does not offer reclaim preview without an evidence-backed reclaimable cause", () => {
    render(
      <ThermalCard
        thermal={demoSnapshot.thermal}
        workloads={[{ ...oldBranch, reclaimable: false }]}
        onInspect={() => {}}
        onPreview={() => {}}
      />,
    );

    expect(screen.getByRole("button", { name: /查看排查详情/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /查看回收预览/ })).not.toBeInTheDocument();
  });

  it("keeps nominal and unavailable thermal states explicit without inventing temperature", () => {
    const nominal = {
      ...demoSnapshot.thermal,
      state: "nominal",
      active: false,
      cause: "未检测到持续热压力",
      causeWorkloadId: "",
      evidence: [],
    };
    const { rerender } = render(
      <ThermalCard thermal={nominal} workloads={demoSnapshot.workloads} onInspect={() => {}} onPreview={() => {}} />,
    );
    expect(screen.getByRole("heading", { name: "热压力稳定" })).toBeInTheDocument();
    expect(screen.getByText("未检测到持续热压力")).toBeInTheDocument();

    rerender(
      <ThermalCard
        thermal={{ ...nominal, state: "unavailable", available: false, cause: "" }}
        workloads={demoSnapshot.workloads}
        onInspect={() => {}}
        onPreview={() => {}}
      />,
    );
    expect(screen.getByRole("heading", { name: "此平台未提供公开热压力状态" })).toBeInTheDocument();
    expect(screen.getByText(/不会猜测温度/)).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/[°℃]/);
  });

  it("only resolves the thermal cause that was actually reclaimed", () => {
    const unrelated = applyLocalResult(demoSnapshot, {
      token: "local-token",
      workloadId: "demo-claude-auth",
      workloadName: "Claude Code · auth-refactor",
      mode: "demo",
      stopped: [63740],
      failed: [],
      pending: [],
      message: "安全回收完成",
      reclaimedBytes: 1290000000,
      confirmedAt: 1793845890000,
      finishedAt: 1793845900000,
    });
    expect(unrelated.thermal.state).toBe("serious");
    expect(unrelated.thermal.causeWorkloadId).toBe("demo-codex-old-branch");

    const resolved = applyLocalResult(demoSnapshot, {
      token: "local-token",
      workloadId: "demo-codex-old-branch",
      workloadName: oldBranch.name,
      mode: "demo",
      stopped: [61202],
      failed: [],
      pending: [],
      message: "安全回收完成",
      reclaimedBytes: 3140000000,
      confirmedAt: 1793845890000,
      finishedAt: 1793845900000,
    });
    expect(resolved.thermal).toMatchObject({
      state: "nominal",
      available: true,
      active: false,
      recovering: false,
      causeWorkloadId: "",
      observedAt: 1793845900000,
    });
  });

  it("renders an explicit empty state instead of fabricating a healthy snapshot", () => {
    render(<HeatTree workloads={[]} selectedId={null} onSelect={() => {}} onPreview={() => {}} />);

    expect(screen.getByText("没有发现活跃的 AI 或开发任务")).toBeInTheDocument();
    expect(screen.getByText(/后台保持安静/)).toBeInTheDocument();
  });

  it("keeps a reduced-motion escape hatch in the shipped stylesheet", () => {
    expect(stylesheet).toContain("@media (prefers-reduced-motion: reduce)");
    expect(stylesheet).toContain("animation-duration: 0.01ms !important");
    expect(stylesheet).toContain("transition-duration: 0.01ms !important");
  });

  it("uses a meaningful headline when the largest task has no descendants", () => {
    expect(describeWorkloadHeadline({ ...oldBranch, processCount: 1, risk: "healthy" })).toBe(
      "“Codex · feature/old-branch” 是当前最大的任务组",
    );
    expect(describeWorkloadHeadline(oldBranch)).toBe(
      "“Codex · feature/old-branch” 留下了 12 个 MCP 后代",
    );
  });

  it("never calls the machine calm while pressure is high and no safe target exists", () => {
    const highPressure = {
      ...demoSnapshot,
      summary: { ...demoSnapshot.summary, pressureLabel: "内存压力偏高", attentionCount: 0 },
    };
    expect(describeOverviewVerdict(highPressure, false)).toEqual({
      tone: "attention",
      firstLine: "资源偏紧，",
      emphasis: "暂无可安全释放。",
    });
    expect(describeOverviewVerdict(demoSnapshot, true)).toEqual({
      tone: "attention",
      firstLine: "后台有 4.9 GB",
      emphasis: "可以安全释放。",
    });
  });

  it("restores the saved theme before the first render", () => {
    localStorage.setItem("ai-task-manager-theme", "light");

    render(<App />);

    expect(document.documentElement.dataset.theme).toBe("light");
    expect(screen.getByRole("button", { name: "切换到暗色主题" })).toBeInTheDocument();
  });

  it("answers a diagnostic question with separated evidence and a read-only boundary", async () => {
    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "AI 诊断" }));
    expect(screen.getByRole("heading", { name: "现在想知道什么？" })).toBeInTheDocument();
    expect(screen.getByText("不执行动作")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "为什么电脑变卡？" }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "电脑变卡时先看这三个信号" })).toBeInTheDocument();
    });
    expect(screen.getByText("事实")).toBeInTheDocument();
    expect(screen.getByText("推断")).toBeInTheDocument();
    expect(screen.getByText("建议")).toBeInTheDocument();
    expect(screen.getByText("不确定项")).toBeInTheDocument();
    expect(screen.getByText(/浏览器预览只在本地分析合成快照/)).toBeInTheDocument();
  });
});
