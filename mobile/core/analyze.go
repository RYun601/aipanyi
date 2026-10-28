package mobilecore

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"aipanyi/backend/data"
	"aipanyi/backend/events"
	"aipanyi/backend/logger"
)

func getAnalysis(code string) *analysisDTO {
	norm := normalizeCode(code)
	if norm == "" {
		return nil
	}
	res := data.NewDeepSeekOpenAi(context.Background(), 0).GetAIResponseResult(norm)
	if res == nil || strings.TrimSpace(res.Content) == "" {
		return nil
	}
	name := res.StockName
	if name == "" {
		name = stockDisplayName(norm)
	}
	return &analysisDTO{
		Code:      norm,
		Name:      name,
		Content:   res.Content,
		Question:  res.Question,
		ModelName: res.ModelName,
		ChatID:    res.ChatId,
		CreatedAt: res.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (a *mobileApp) startAnalysis(code string) (string, error) {
	norm := normalizeCode(code)
	if norm == "" {
		return "", errors.New("股票代码为空")
	}
	f := data.NewStockDataApi().GetFollowedStockByStockCode(norm)
	if f.StockCode == "" {
		return "", errors.New("请先关注该股票")
	}
	settings := loadMobileSettings()
	aiID := effectiveAIConfigID(settings)
	if aiID == 0 {
		return "", errors.New("请先在设置里添加 AI 配置")
	}

	a.analysisMu.Lock()
	if a.analysisCancel != nil {
		a.analysisMu.Unlock()
		return "", errors.New("已有分析在进行中，请等待完成或先取消")
	}
	ctx, cancel := context.WithCancel(context.Background())
	runID := uuid.NewString()
	a.analysisCancel = cancel
	a.analysisRunID = runID
	a.analysisMu.Unlock()

	go a.runAnalysis(ctx, runID, norm, f.Name, aiID, settings)
	return runID, nil
}

func (a *mobileApp) cancelAnalysis() {
	a.analysisMu.Lock()
	cancel := a.analysisCancel
	a.analysisMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *mobileApp) finishAnalysis(runID string) {
	a.analysisMu.Lock()
	if a.analysisRunID == runID {
		a.analysisRunID = ""
		a.analysisCancel = nil
	}
	a.analysisMu.Unlock()
}

func (a *mobileApp) runAnalysis(ctx context.Context, runID, code, name string, aiConfigID int64, settings mobileSettings) {
	defer a.finishAnalysis(runID)
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("mobile analysis panic: %v", r)
			emitMobile("mobileAnalysis", map[string]any{
				"runId": runID, "code": code, "done": true,
				"error": "分析异常，请重试",
			})
		}
	}()

	o := data.NewDeepSeekOpenAi(ctx, int(aiConfigID))
	tools := []data.Tool{}
	if settings.EnableTools {
		tools = data.Tools(nil)
	}
	var sysPtr *int
	if id := effectivePromptID(settings); id > 0 {
		sysPtr = &id
	}

	msgs := o.NewChatStream(name, code, "", sysPtr, tools, settings.Thinking)
	var content, reasoning strings.Builder
	var chatID, question, errText string

	for msg := range msgs {
		if ctx.Err() != nil {
			break
		}
		if isErrorMsg(msg) {
			if v, ok := msg["content"].(string); ok && v != "" {
				errText = v
			}
			continue
		}
		if v, ok := msg["content"].(string); ok && v != "" {
			content.WriteString(v)
			emitMobile("mobileAnalysis", map[string]any{
				"runId": runID, "code": code, "delta": v,
			})
		}
		if v, ok := msg["reasoning_content"].(string); ok && v != "" {
			reasoning.WriteString(v)
			emitMobile("mobileAnalysis", map[string]any{
				"runId": runID, "code": code, "reasoning": v,
			})
		}
		if v, ok := msg["extraContent"].(string); ok && v != "" {
			content.WriteString(v)
			emitMobile("mobileAnalysis", map[string]any{
				"runId": runID, "code": code, "delta": v,
			})
		}
		if v, ok := msg["chatId"].(string); ok && v != "" {
			chatID = v
		}
		if v, ok := msg["question"].(string); ok && v != "" {
			question = v
		}
	}

	if ctx.Err() != nil {
		emitMobile("mobileAnalysis", map[string]any{
			"runId": runID, "code": code, "done": true, "canceled": true,
		})
		return
	}

	final := strings.TrimSpace(content.String())
	if final == "" {
		msg := "AI 未返回内容，请检查 AI 配置或网络"
		if errText != "" {
			msg = errText
		}
		emitMobile("mobileAnalysis", map[string]any{
			"runId": runID, "code": code, "done": true,
			"error": msg,
		})
		return
	}

	o.SaveAIResponseResult(code, name, final, chatID, question)
	emitMobile("mobileAnalysis", map[string]any{
		"runId": runID, "code": code, "done": true, "saved": true,
	})
	// 分析本体已完成，先释放“进行中”锁；后面的策略提取不阻塞下一次分析。
	a.finishAnalysis(runID)

	if !settings.AutoExtractAlerts {
		return
	}
	cfg, ok := findAIConfig(aiConfigID)
	if !ok {
		return
	}
	suggestions, err := extractAlertsWithConfig(code, name, final, mustPrice(code), cfg)
	if err != nil {
		logger.SugaredLogger.Warnf("mobile: extract alerts failed: %v", err)
		emitMobile("mobileAlertSuggestions", map[string]any{
			"code": code, "name": name, "error": err.Error(),
		})
		return
	}
	emitMobile("mobileAlertSuggestions", map[string]any{
		"code": code, "name": name, "suggestions": suggestions,
	})
}

func emitMobile(event string, payload map[string]any) {
	defer func() { _ = recover() }()
	events.Emit(context.Background(), event, payload)
}

// isErrorMsg 判断 NewChatStream 推送的是错误帧（code=0），而不是正文。
func isErrorMsg(msg map[string]any) bool {
	switch v := msg["code"].(type) {
	case int:
		return v == 0
	case int64:
		return v == 0
	case float64:
		return v == 0
	default:
		return false
	}
}
