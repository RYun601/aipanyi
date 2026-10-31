package mobilecore

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"aipanyi/backend/data"
	"aipanyi/backend/logger"
)

type chatStreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// chatStream 是移动端“轻量分析”使用的一次性流式对话：
// 不注入桌面端的 8 路行情/资讯上下文，也不带工具调用，避免手机网络下长时间卡住。
func chatStream(ctx context.Context, cfg data.AIConfig, systemPrompt, userPrompt string, maxTokens int, onDelta, onReasoning func(string)) (string, error) {
	if strings.TrimSpace(cfg.BaseUrl) == "" || strings.TrimSpace(cfg.ModelName) == "" {
		return "", errors.New("AI 配置不完整：请填写 BaseUrl 和模型名")
	}
	timeout := cfg.TimeOut
	if timeout <= 0 || timeout > 180 {
		timeout = 180
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	if maxTokens > 4096 {
		maxTokens = 4096
	}

	client := resty.New().SetTimeout(time.Duration(timeout) * time.Second)
	if cfg.HttpProxyEnabled && strings.TrimSpace(cfg.HttpProxy) != "" {
		client.SetProxy(strings.TrimSpace(cfg.HttpProxy))
	}
	base, path := data.OpenAIChatEndpoint(cfg.BaseUrl)
	body := map[string]any{
		"model":       cfg.ModelName,
		"stream":      true,
		"temperature": cfg.Temperature,
		"max_tokens":  maxTokens,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}
	req := client.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		SetHeader("Authorization", "Bearer "+cfg.ApiKey).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "text/event-stream").
		SetBody(body)
	if extra := buildExtraHeaders(cfg); len(extra) > 0 {
		req = req.SetHeaders(extra)
	}
	resp, err := req.Post(base + path)
	if err != nil {
		logger.SugaredLogger.Errorf("chatStream: post %s failed: %v", base+path, err)
		return "", fmt.Errorf("请求 AI 服务失败: %w", err)
	}
	logger.SugaredLogger.Infof("chatStream: post %s status=%d", base+path, resp.StatusCode())
	defer func() { _ = resp.RawBody().Close() }()

	if resp.StatusCode() != 200 {
		raw, _ := io.ReadAll(resp.RawBody())
		msg := strings.TrimSpace(string(raw))
		var parsed chatCompletionResponse
		if json.Unmarshal(raw, &parsed) == nil && parsed.Error != nil && parsed.Error.Message != "" {
			msg = parsed.Error.Message
		}
		if len([]rune(msg)) > 300 {
			msg = string([]rune(msg)[:300])
		}
		return "", fmt.Errorf("AI 服务返回 %d: %s", resp.StatusCode(), msg)
	}

	var content, reasoning strings.Builder
	scanner := bufio.NewScanner(resp.RawBody())
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			break
		}
		var chunk chatStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			// 个别兼容服务会混入非 JSON 行；跳过，不中断整段分析。
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return content.String(), errors.New(chunk.Error.Message)
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				if onDelta != nil {
					onDelta(choice.Delta.Content)
				}
			}
			if choice.Delta.ReasoningContent != "" {
				reasoning.WriteString(choice.Delta.ReasoningContent)
				if onReasoning != nil {
					onReasoning(choice.Delta.ReasoningContent)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return content.String(), ctx.Err()
		}
		return content.String(), fmt.Errorf("读取 AI 流失败: %w", err)
	}
	if strings.TrimSpace(content.String()) == "" {
		return "", errors.New("AI 未返回文本内容")
	}
	return content.String(), nil
}
