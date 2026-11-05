package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	aiProviderDeepSeek = "deepseek"
	aiProviderCustom   = "openai-compatible"
	aiProviderLocal    = "local"
)

func normalizeAIProviderInput(input AIProviderInput) (aiProviderConfig, error) {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider == "" {
		provider = aiProviderCustom
	}
	switch provider {
	case aiProviderDeepSeek, aiProviderCustom, aiProviderLocal:
	default:
		return aiProviderConfig{}, errors.New("不支持的模型提供方")
	}

	baseURL := strings.TrimSpace(input.BaseURL)
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
	if baseURL == "" {
		return aiProviderConfig{}, errors.New("请填写模型接口地址")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return aiProviderConfig{}, errors.New("模型接口地址必须是有效的 http 或 https URL")
	}
	if parsed.User != nil {
		return aiProviderConfig{}, errors.New("模型接口地址不能包含用户名或密码")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return aiProviderConfig{}, errors.New("模型接口地址不能包含查询参数或片段")
	}

	model := strings.TrimSpace(input.Model)
	if model == "" {
		return aiProviderConfig{}, errors.New("请填写模型名称")
	}
	if len([]rune(model)) > 128 {
		return aiProviderConfig{}, errors.New("模型名称过长")
	}
	apiKey := strings.TrimSpace(input.APIKey)
	if len(apiKey) > 4096 {
		return aiProviderConfig{}, errors.New("API Key 过长")
	}

	remote := !isLoopbackEndpoint(parsed)
	if remote && !input.AllowRemote {
		return aiProviderConfig{}, errors.New("请先确认允许将聚合任务摘要发送到远程模型")
	}
	return aiProviderConfig{
		provider:    provider,
		baseURL:     baseURL,
		model:       model,
		apiKey:      apiKey,
		allowRemote: input.AllowRemote,
		remote:      remote,
		source:      "app",
	}, nil
}

// ValidateAIProvider checks an input without changing the active provider.
// The desktop bridge uses it before writing credentials or configuration.
func (s *Service) ValidateAIProvider(input AIProviderInput) error {
	_, err := normalizeAIProviderInput(input)
	return err
}

func (s *Service) SetAIProvider(input AIProviderInput, source string) error {
	cfg, err := normalizeAIProviderInput(input)
	if err != nil {
		return err
	}
	if source == "" {
		source = "app"
	}
	cfg.source = source
	s.aiMu.Lock()
	s.aiOverride = &cfg
	s.aiMu.Unlock()
	return nil
}

func (s *Service) ClearAIProvider() {
	s.aiMu.Lock()
	s.aiOverride = nil
	s.aiMu.Unlock()
}

func (s *Service) aiProvider() aiProviderConfig {
	s.aiMu.RLock()
	override := s.aiOverride
	if override != nil {
		cfg := *override
		s.aiMu.RUnlock()
		return cfg
	}
	s.aiMu.RUnlock()
	return loadAIProviderConfig()
}

func (s *Service) AISettings() AISettings {
	cfg := s.aiProvider()
	return AISettings{
		Provider:    providerOrDefault(cfg),
		BaseURL:     cfg.baseURL,
		Model:       cfg.model,
		AllowRemote: cfg.allowRemote,
		HasAPIKey:   cfg.apiKey != "",
		Source:      cfg.source,
	}
}

// TestAIConnection performs one tiny completion request. It deliberately does
// not include a process snapshot, and no response content is returned or logged.
func (s *Service) TestAIConnection(ctx context.Context, input AIProviderInput) (AITestResult, error) {
	cfg, err := normalizeAIProviderInput(input)
	if err != nil {
		return AITestResult{}, err
	}
	if cfg.remote && cfg.apiKey == "" {
		return AITestResult{}, errors.New("远程模型需要 API Key")
	}
	start := time.Now()
	payload := map[string]any{
		"model": cfg.model,
		"messages": []map[string]string{
			{"role": "system", "content": "This is a connection check. Reply with OK only."},
			{"role": "user", "content": "OK"},
		},
		"temperature": 0,
		"max_tokens":  8,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return AITestResult{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, cfg.baseURL+"/chat/completions", strings.NewReader(string(encoded)))
	if err != nil {
		return AITestResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.apiKey)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return AITestResult{}, fmt.Errorf("无法连接模型端点：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return AITestResult{}, fmt.Errorf("模型端点返回 HTTP %d，请检查地址、模型和 API Key", resp.StatusCode)
	}
	var completion struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&completion); err != nil {
		return AITestResult{}, errors.New("模型端点返回了无法解析的响应")
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return AITestResult{}, errors.New("模型端点已连通，但没有返回内容")
	}
	reportedModel := strings.TrimSpace(completion.Model)
	if reportedModel == "" {
		reportedModel = cfg.model
	}
	return AITestResult{
		OK:        true,
		Model:     reportedModel,
		LatencyMs: time.Since(start).Milliseconds(),
		Message:   "连接成功，模型可以正常响应",
		Remote:    cfg.remote,
	}, nil
}

func providerOrDefault(cfg aiProviderConfig) string {
	if cfg.provider != "" {
		return cfg.provider
	}
	if parsed, err := url.Parse(cfg.baseURL); err == nil {
		host := strings.ToLower(parsed.Hostname())
		if host == "api.deepseek.com" {
			return aiProviderDeepSeek
		}
		if isLoopbackEndpoint(parsed) {
			return aiProviderLocal
		}
	}
	return aiProviderCustom
}
