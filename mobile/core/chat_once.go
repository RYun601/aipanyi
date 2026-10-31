package mobilecore

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"

	"aipanyi/backend/data"
	"aipanyi/backend/logger"
)

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// chatOnce 做一次非流式 OpenAI 兼容调用，用于“从分析中提取价格提醒”。
// 不复用 NewChatStream：后者会注入大量行情/资讯上下文，对结构化提取是浪费。
func chatOnce(cfg data.AIConfig, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if strings.TrimSpace(cfg.BaseUrl) == "" || strings.TrimSpace(cfg.ModelName) == "" {
		return "", errors.New("AI 配置不完整：请填写 BaseUrl 和模型名")
	}
	timeout := cfg.TimeOut
	if timeout <= 0 {
		timeout = 120
	}
	// 一次性提取/连接测试不需要等满 300 秒；同时避免前端 180 秒请求先超时。
	if timeout > 150 {
		timeout = 150
	}
	if maxTokens <= 0 {
		maxTokens = 600
	}

	client := resty.New().SetTimeout(time.Duration(timeout) * time.Second)
	if cfg.HttpProxyEnabled && strings.TrimSpace(cfg.HttpProxy) != "" {
		client.SetProxy(strings.TrimSpace(cfg.HttpProxy))
	}

	base, path := data.OpenAIChatEndpoint(cfg.BaseUrl)
	body := map[string]any{
		"model":       cfg.ModelName,
		"stream":      false,
		"temperature": 0,
		"max_tokens":  maxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}

	req := client.R().
		SetHeader("Authorization", "Bearer "+cfg.ApiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(body)
	if extra := buildExtraHeaders(cfg); len(extra) > 0 {
		req = req.SetHeaders(extra)
	}

	logger.SugaredLogger.Infof("mobile chatOnce: model=%s base=%s", cfg.ModelName, base)
	resp, err := req.Post(base + path)
	if err != nil {
		return "", fmt.Errorf("请求 AI 服务失败: %w", err)
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(resp.Body(), &parsed); err != nil {
		return "", fmt.Errorf("解析 AI 响应失败: %w", err)
	}
	if resp.StatusCode() != 200 {
		msg := strings.TrimSpace(resp.String())
		if parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		if len([]rune(msg)) > 300 {
			msg = string([]rune(msg)[:300])
		}
		return "", fmt.Errorf("AI 服务返回 %d: %s", resp.StatusCode(), msg)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("AI 返回为空")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", errors.New("AI 未返回文本内容")
	}
	return content, nil
}

func buildExtraHeaders(cfg data.AIConfig) map[string]string {
	raw := strings.TrimSpace(cfg.ExtraHeaders)
	if raw == "" {
		return nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		v = strings.ReplaceAll(v, "{{sessionId}}", cfg.SessionId)
		v = strings.ReplaceAll(v, "{{uuid}}", uuid.NewString())
		out[k] = v
	}
	return out
}
