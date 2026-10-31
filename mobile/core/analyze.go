package mobilecore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"aipanyi/backend/data"
	"aipanyi/backend/events"
	"aipanyi/backend/logger"
)

const defaultAnalysisSystemPrompt = `你是一位严谨的 A 股分析师，擅长把持仓成本、仓位和实时价格结合起来给出可执行的交易策略。
要求：
1. 只基于给定数据分析，不编造行情、财务或消息数据。
2. 给出趋势判断、关键支撑/压力位、操作建议（买入/持有/减仓/止损）和风险提示。
3. 涉及价格时给出明确数字或区间。
4. 用中文 Markdown 输出，结构清晰。`

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

	name := f.Name
	if strings.TrimSpace(name) == "" {
		name = stockDisplayName(norm)
	}
	go a.runAnalysis(ctx, runID, norm, name, aiID, settings)
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
	logger.SugaredLogger.Infof("runAnalysis entered: run=%s code=%s name=%s ai=%d", runID, code, name, aiConfigID)
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

	cfg, ok := findAIConfig(aiConfigID)
	if !ok {
		emitMobile("mobileAnalysis", map[string]any{
			"runId": runID, "code": code, "done": true,
			"error": "AI 配置不存在",
		})
		return
	}
	logger.SugaredLogger.Infof("mobile analysis start: run=%s code=%s name=%s ai=%d model=%s", runID, code, name, aiConfigID, cfg.ModelName)
	follow := data.NewStockDataApi().GetFollowedStockByStockCode(code)
	logger.SugaredLogger.Infof("mobile analysis: follow cost=%.2f volume=%d storedPrice=%.2f", follow.CostPrice, follow.Volume, follow.Price)
	price := quickPrice(code, follow.Price)
	logger.SugaredLogger.Infof("mobile analysis: price=%.2f", price)
	cost := follow.CostPrice
	volume := follow.Volume
	kline := quickKLine(code)
	logger.SugaredLogger.Infof("mobile analysis: kline len=%d", len(kline))
	repl := analysisTemplateReplacer(name, code, price, cost, volume)
	systemPrompt := buildAnalysisSystemPrompt(settings, repl)
	userPrompt := buildAnalysisUserPrompt(repl, buildAnalysisDataBlock(name, code, price, cost, volume), kline)
	chatID := uuid.NewString()

	logger.SugaredLogger.Infof("mobile analysis: calling AI, prompt len=%d", len(userPrompt))
	final, err := chatStream(ctx, cfg, systemPrompt, userPrompt, cfg.MaxTokens,
		func(delta string) {
			emitMobile("mobileAnalysis", map[string]any{
				"runId": runID, "code": code, "delta": delta,
			})
		},
		func(reasoning string) {
			emitMobile("mobileAnalysis", map[string]any{
				"runId": runID, "code": code, "reasoning": reasoning,
			})
		})

	if ctx.Err() != nil {
		emitMobile("mobileAnalysis", map[string]any{
			"runId": runID, "code": code, "done": true, "canceled": true,
		})
		return
	}
	logger.SugaredLogger.Infof("mobile analysis: AI done len=%d err=%v", len(final), err)
	if err != nil {
		logger.SugaredLogger.Errorf("mobile analysis failed: %v", err)
		emitMobile("mobileAnalysis", map[string]any{
			"runId": runID, "code": code, "done": true, "error": err.Error(),
		})
		return
	}
	final = strings.TrimSpace(final)
	if final == "" {
		emitMobile("mobileAnalysis", map[string]any{
			"runId": runID, "code": code, "done": true,
			"error": "AI 未返回内容，请检查 AI 配置或网络",
		})
		return
	}

	data.NewDeepSeekOpenAi(ctx, int(aiConfigID)).SaveAIResponseResult(code, name, final, chatID, userPrompt)
	emitMobile("mobileAnalysis", map[string]any{
		"runId": runID, "code": code, "done": true, "saved": true,
	})
	// 分析本体已完成，先释放“进行中”锁；后面的策略提取不阻塞下一次分析。
	a.finishAnalysis(runID)

	if !settings.AutoExtractAlerts {
		return
	}
	suggestions, err := extractAlertsWithConfig(code, name, final, price, cfg)
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

// analysisTemplateReplacer 复用桌面端模板占位符的三种写法。
func analysisTemplateReplacer(name, code string, price, cost float64, volume int64) *strings.Replacer {
	costText := "未设置"
	if cost > 0 {
		costText = fmt.Sprintf("%.2f", cost)
	}
	volText := "未设置"
	if volume > 0 {
		volText = fmt.Sprintf("%d", volume)
	}
	priceText := "未知"
	if price > 0 {
		priceText = fmt.Sprintf("%.2f", price)
	}
	return strings.NewReplacer(
		"{{stockName}}", name, "{stockName}", name, "stockName", name,
		"{{stockCode}}", code, "{stockCode}", code, "stockCode", code,
		"{{costPrice}}", costText, "{costPrice}", costText, "costPrice", costText,
		"{{costVolume}}", volText, "{costVolume}", volText, "costVolume", volText,
		"{{price}}", priceText, "{price}", priceText,
	)
}

func buildAnalysisSystemPrompt(settings mobileSettings, repl *strings.Replacer) string {
	if settings.DefaultPromptID > 0 {
		if p := data.NewPromptTemplateApi().GetPromptTemplateByID(int(settings.DefaultPromptID)); strings.TrimSpace(p) != "" {
			return repl.Replace(p)
		}
	}
	if p := strings.TrimSpace(data.GetSettingConfig().Prompt); p != "" {
		return repl.Replace(p)
	}
	return defaultAnalysisSystemPrompt
}

func buildAnalysisUserPrompt(repl *strings.Replacer, dataBlock, kline string) string {
	question := strings.TrimSpace(data.GetSettingConfig().QuestionTemplate)
	if question == "" {
		question = "请结合当前价、成本价和持仓数量分析 {{stockName}}（{{stockCode}}）：\n" +
			"1. 当前趋势与关键支撑/压力位；\n" +
			"2. 结合我的成本和仓位，给出继续持有/加仓/减仓/止损的建议；\n" +
			"3. 如果给出操盘策略，请明确触发价格和仓位。"
	}
	question = repl.Replace(question)
	out := dataBlock + "\n\n" + question
	if strings.TrimSpace(kline) != "" {
		out += "\n\n近 30 日 K 线：\n" + kline
	}
	return out
}

func buildAnalysisDataBlock(name, code string, price, cost float64, volume int64) string {
	priceText := "未知"
	if price > 0 {
		priceText = fmt.Sprintf("%.2f", price)
	}
	costText := "未设置"
	if cost > 0 {
		costText = fmt.Sprintf("%.2f", cost)
	}
	volText := "未设置"
	if volume > 0 {
		volText = fmt.Sprintf("%d", volume)
	}
	plText := "未设置"
	if cost > 0 && volume > 0 && price > 0 {
		amount := (price - cost) * float64(volume)
		pct := (price/cost - 1) * 100
		plText = fmt.Sprintf("%+.2f 元（%+.2f%%）", amount, pct)
	}
	return fmt.Sprintf("当前数据：\n- 股票：%s（%s）\n- 当前价：%s\n- 成本价：%s\n- 持仓数量：%s\n- 持仓盈亏：%s",
		name, code, priceText, costText, volText, plText)
}

func quickPrice(code string, fallback float64) float64 {
	ch := make(chan float64, 1)
	go func() { ch <- mustPrice(code) }()
	select {
	case p := <-ch:
		if p > 0 {
			return p
		}
	case <-time.After(6 * time.Second):
		logger.SugaredLogger.Warnf("mobile: quote timeout for %s, use stored price", code)
	}
	return fallback
}

func quickKLine(code string) string {
	ch := make(chan string, 1)
	go func() {
		var rows *[]data.KLineData
		if strings.HasPrefix(code, "hk") || strings.HasPrefix(code, "gb_") || strings.HasPrefix(code, "us") {
			rows = data.NewStockDataApi().GetHK_KLineData(code, "day", 60)
		} else {
			rows = data.NewStockDataApi().GetKLineData(code, "240", 60)
		}
		if rows == nil || len(*rows) == 0 {
			ch <- ""
			return
		}
		list := *rows
		start := 0
		if len(list) > 30 {
			start = len(list) - 30
		}
		var sb strings.Builder
		sb.WriteString("|日期|开盘|最高|最低|收盘|成交量|\n|---|---|---|---|---|---|\n")
		for _, k := range list[start:] {
			sb.WriteString(fmt.Sprintf("|%s|%s|%s|%s|%s|%s|\n", k.Day, k.Open, k.High, k.Low, k.Close, k.Volume))
		}
		ch <- sb.String()
	}()
	select {
	case s := <-ch:
		return s
	case <-time.After(8 * time.Second):
		logger.SugaredLogger.Warnf("mobile: kline timeout for %s, continue without it", code)
		return ""
	}
}
