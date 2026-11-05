import { useEffect, useState, type FormEvent } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import {
  Check,
  ChevronDown,
  Eye,
  EyeOff,
  Globe2,
  KeyRound,
  Laptop,
  LoaderCircle,
  LockKeyhole,
  ServerCog,
  ShieldCheck,
  Sparkles,
  Trash2,
  X,
} from "lucide-react";
import { TaskManager, type AIProviderInput, type AISettings, type AIStatus, type AITestResult } from "../mygo";

type ProviderID = "deepseek" | "openai-compatible" | "local";

interface AISettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  bridgeAvailable: boolean;
  status: AIStatus | null;
  onStatusChange: (status: AIStatus) => void;
}

const providerPresets: Array<{
  id: ProviderID;
  name: string;
  note: string;
  baseUrl: string;
  model: string;
  icon: typeof Sparkles;
}> = [
  {
    id: "deepseek",
    name: "DeepSeek",
    note: "云端 · 推荐",
    baseUrl: "https://api.deepseek.com/v1",
    model: "deepseek-chat",
    icon: Sparkles,
  },
  {
    id: "openai-compatible",
    name: "兼容接口",
    note: "OpenAI API",
    baseUrl: "https://api.openai.com/v1",
    model: "gpt-4.1-mini",
    icon: Globe2,
  },
  {
    id: "local",
    name: "本地模型",
    note: "数据不出本机",
    baseUrl: "http://127.0.0.1:11434/v1",
    model: "qwen2.5-coder:7b",
    icon: Laptop,
  },
];

const emptySettings: AISettings = {
  provider: "deepseek",
  baseUrl: "https://api.deepseek.com/v1",
  model: "deepseek-chat",
  allowRemote: false,
  hasApiKey: false,
  source: "",
  restoring: false,
};

export function AISettingsDialog({
  open,
  onOpenChange,
  bridgeAvailable,
  status,
  onStatusChange,
}: AISettingsDialogProps) {
  const [provider, setProvider] = useState<ProviderID>("deepseek");
  const [baseUrl, setBaseUrl] = useState(emptySettings.baseUrl);
  const [model, setModel] = useState(emptySettings.model);
  const [apiKey, setApiKey] = useState("");
  const [allowRemote, setAllowRemote] = useState(false);
  const [hasStoredKey, setHasStoredKey] = useState(false);
  const [hasSavedSettings, setHasSavedSettings] = useState(false);
  const [showKey, setShowKey] = useState(false);
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<AITestResult | null>(null);
  const [restoringSettings, setRestoringSettings] = useState(false);

  useEffect(() => {
    if (!open) return;
    setApiKey("");
    setShowKey(false);
    setError(null);
    setNotice(null);
    setTestResult(null);

    if (!bridgeAvailable) {
      setLoading(false);
      return;
    }

    let active = true;
    let retryTimer: number | undefined;
    setLoading(true);
    const loadSettings = () => {
      TaskManager.getAISettings()
        .then((settings) => {
          if (!active) return;
          applySettings(settings);
          setRestoringSettings(settings.restoring);
          if (settings.restoring) retryTimer = window.setTimeout(loadSettings, 500);
        })
        .catch((reason: unknown) => {
          if (!active) return;
          setError(reason instanceof Error ? reason.message : "无法读取 AI 配置");
        })
        .finally(() => {
          if (active) setLoading(false);
        });
    };
    loadSettings();

    return () => {
      active = false;
      if (retryTimer !== undefined) window.clearTimeout(retryTimer);
    };
  }, [bridgeAvailable, open]);

  const applySettings = (settings: AISettings) => {
    const nextProvider = normalizeProvider(settings.provider);
    setProvider(nextProvider);
    setBaseUrl(settings.baseUrl || presetFor(nextProvider).baseUrl);
    setModel(settings.model || presetFor(nextProvider).model);
    setAllowRemote(nextProvider === "local" ? false : settings.allowRemote);
    setHasStoredKey(settings.hasApiKey);
    setHasSavedSettings(settings.source === "app");
  };

  const chooseProvider = (next: ProviderID) => {
    const preset = presetFor(next);
    setProvider(next);
    setBaseUrl(preset.baseUrl);
    setModel(preset.model);
    // Every remote provider switch starts with consent off. A saved configuration
    // may restore consent, but a preset must never opt the user in implicitly.
    setAllowRemote(false);
    setTestResult(null);
    setError(null);
    setNotice(null);
  };

  const buildInput = (): AIProviderInput => ({
    provider,
    baseUrl: baseUrl.trim(),
    model: model.trim(),
    apiKey: apiKey.trim(),
    allowRemote: provider === "local" ? false : allowRemote,
  });

  const validateLocally = () => {
    if (!baseUrl.trim()) return "请填写模型接口地址";
    if (!/^https?:\/\//i.test(baseUrl.trim())) return "接口地址必须以 http:// 或 https:// 开头";
    if (!model.trim()) return "请填写模型名称";
    if (provider !== "local" && !allowRemote) return "请先确认允许发送聚合任务摘要到远程模型";
    if (provider !== "local" && !apiKey.trim() && !hasStoredKey) return "远程模型需要 API Key";
    return null;
  };

  const testConnection = async () => {
    const message = validateLocally();
    if (message) {
      setError(message);
      return;
    }
    if (!bridgeAvailable) {
      setError("浏览器预览无法访问模型端点，请在桌面应用中测试。");
      return;
    }
    setTesting(true);
    setError(null);
    setNotice(null);
    setTestResult(null);
    try {
      const result = await TaskManager.testAIConnection(buildInput());
      setTestResult(result);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "连接测试失败");
    } finally {
      setTesting(false);
    }
  };

  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const message = validateLocally();
    if (message) {
      setError(message);
      return;
    }
    if (!bridgeAvailable) {
      setError("浏览器预览不会保存密钥，请在桌面应用中配置。");
      return;
    }
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      const nextStatus = await TaskManager.saveAISettings(buildInput());
      setApiKey("");
      setHasStoredKey(nextStatus.apiKeyAvailable);
      setHasSavedSettings(nextStatus.source === "app");
      onStatusChange(nextStatus);
      setNotice("配置已保存。API Key 已安全存入系统凭据库。");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "保存 AI 配置失败");
    } finally {
      setSaving(false);
    }
  };

  const clear = async () => {
    if (!bridgeAvailable) return;
    setClearing(true);
    setError(null);
    setNotice(null);
    setTestResult(null);
    try {
      const nextStatus = await TaskManager.clearAISettings();
      setApiKey("");
      setHasStoredKey(false);
      setHasSavedSettings(false);
      onStatusChange(nextStatus);
      setNotice("配置与密钥已清除，已恢复本地规则。");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "清除 AI 配置失败");
    } finally {
      setClearing(false);
    }
  };

  const activeStatus = status?.restoring ? "正在恢复模型配置" : status?.configured ? "模型解释已启用" : "当前使用本地规则";
  const remoteLocked = provider === "local";
  const restoreBlocking = restoringSettings && !apiKey.trim();

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="dialog-overlay settings-overlay" />
        <Dialog.Content className="dialog-content settings-dialog" aria-describedby="ai-settings-description">
          <header className="settings-head">
            <div>
              <span className="settings-kicker"><ServerCog size={15} aria-hidden="true" /> AI 服务</span>
              <Dialog.Title>连接你的 AI 模型</Dialog.Title>
              <Dialog.Description id="ai-settings-description">
                本地规则始终可用。远程模型只负责解释聚合后的任务摘要，不能停止任何进程。
              </Dialog.Description>
            </div>
            <Dialog.Close asChild>
              <button type="button" className="icon-button" aria-label="关闭 AI 设置"><X size={19} /></button>
            </Dialog.Close>
          </header>

          <div className={`settings-status ${status?.configured ? "is-active" : ""} ${status?.restoring ? "is-restoring" : ""}`}>
            <span className="settings-status__icon" aria-hidden="true">
              {status?.configured ? <Sparkles size={18} /> : <ShieldCheck size={18} />}
            </span>
            <div>
              <strong>{activeStatus}</strong>
              <p>{status?.privacy || "仅使用本机快照，不发起网络请求。"}</p>
            </div>
            <span className="status-chip">{status?.restoring ? "正在恢复" : status?.configured ? "模型增强" : "本地优先"}</span>
          </div>

          <form className="settings-form" onSubmit={(event) => void save(event)}>
            <fieldset className="provider-fieldset">
              <legend>选择服务</legend>
              <div className="provider-grid" role="radiogroup" aria-label="AI 服务提供方">
                {providerPresets.map((preset) => {
                  const Icon = preset.icon;
                  const selected = provider === preset.id;
                  return (
                    <button
                      type="button"
                      role="radio"
                      aria-checked={selected}
                      className={`provider-option ${selected ? "is-selected" : ""}`}
                      onClick={() => chooseProvider(preset.id)}
                      key={preset.id}
                    >
                      <span className="provider-option__icon"><Icon size={18} aria-hidden="true" /></span>
                      <span><strong>{preset.name}</strong><small>{preset.note}</small></span>
                      {selected && <Check className="provider-check" size={15} aria-hidden="true" />}
                    </button>
                  );
                })}
              </div>
            </fieldset>

            <div className="settings-two-columns">
              <label className="field">
                <span>接口地址</span>
                <input
                  value={baseUrl}
                  onChange={(event) => setBaseUrl(event.target.value)}
                  placeholder="https://api.deepseek.com/v1"
                  autoComplete="url"
                  spellCheck={false}
                />
                <small>兼容 OpenAI Chat Completions；末尾的 /chat/completions 会自动移除。</small>
              </label>
              <label className="field">
                <span>模型名称</span>
                <input
                  value={model}
                  onChange={(event) => setModel(event.target.value)}
                  placeholder="deepseek-chat"
                  autoComplete="off"
                  spellCheck={false}
                />
                <small>填写服务商控制台中的准确模型 ID。</small>
              </label>
            </div>

            <label className="field field--secret">
              <span>API Key</span>
              <div className="secret-input">
                <KeyRound size={17} aria-hidden="true" />
                <input
                  type={showKey ? "text" : "password"}
                  value={apiKey}
                  onChange={(event) => setApiKey(event.target.value)}
                  placeholder={hasStoredKey ? "已安全保存在系统凭据库；留空则不更改" : remoteLocked ? "本地模型通常无需 API Key" : "输入 API Key"}
                  autoComplete="new-password"
                  spellCheck={false}
                />
                <button
                  type="button"
                  className="secret-toggle"
                  onClick={() => setShowKey((current) => !current)}
                  aria-label={showKey ? "隐藏 API Key" : "显示 API Key"}
                >
                  {showKey ? <EyeOff size={17} /> : <Eye size={17} />}
                </button>
              </div>
              <small><LockKeyhole size={12} aria-hidden="true" /> 只写入 macOS 系统凭据库，不写入配置文件、日志或诊断请求。</small>
            </label>

            <button
              type="button"
              role="switch"
              aria-checked={!remoteLocked && allowRemote}
              className={`remote-consent ${remoteLocked ? "is-locked" : ""}`}
              onClick={() => { if (!remoteLocked) setAllowRemote((current) => !current); }}
              disabled={remoteLocked}
            >
              <span className="switch-track" aria-hidden="true"><i /></span>
              <span>
                <strong>允许发送聚合任务摘要到远程模型</strong>
                <small>不发送 PID、完整命令行、环境变量或 Agent 对话内容。可以随时在本页关闭。</small>
              </span>
              <ChevronDown className="consent-chevron" size={16} aria-hidden="true" />
            </button>

            {testResult && (
              <div className="connection-success" role="status">
                <span><Check size={17} aria-hidden="true" /></span>
                <div><strong>连接成功</strong><p>{testResult.message} · {testResult.model} · {testResult.latencyMs} ms</p></div>
              </div>
            )}

            {error && <div className="settings-error" role="alert">{error}</div>}
            {notice && <div className="settings-notice" role="status">{notice}</div>}

            <footer className="settings-actions">
              <button
                type="button"
                className="text-danger-button"
                onClick={() => void clear()}
                disabled={(!hasSavedSettings && !hasStoredKey) || clearing || !bridgeAvailable}
              >
                {clearing ? <LoaderCircle className="spin" size={16} /> : <Trash2 size={16} />}
                清除配置与密钥
              </button>
              <div>
                <button type="button" className="secondary-button" onClick={() => void testConnection()} disabled={testing || loading || restoreBlocking}>
                  {testing && <LoaderCircle className="spin" size={16} aria-hidden="true" />}
                  {testing ? "正在测试…" : "测试连接"}
                </button>
                <button type="submit" className="primary-button" disabled={saving || loading || restoreBlocking}>
                  {saving && <LoaderCircle className="spin" size={16} aria-hidden="true" />}
                  {saving ? "正在保存…" : "保存配置"}
                </button>
              </div>
            </footer>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function normalizeProvider(provider: string): ProviderID {
  if (provider === "local" || provider === "openai-compatible") return provider;
  return "deepseek";
}

function presetFor(provider: ProviderID) {
  return providerPresets.find((preset) => preset.id === provider) ?? providerPresets[0];
}
