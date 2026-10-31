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

const defaultAnalysisSystemPromptNeutral = `你是一位严谨的 A 股分析师，擅长基于行情和 K 线给出可执行的交易策略。
要求：
1. 只基于给定数据分析，不编造行情、财务或消息数据。
2. 本次未提供用户的成本、持仓或账户信息；不要假设用户的成本、仓位或现金。
3. 给出趋势判断、关键支撑/压力位、操作建议（买入/持有/减仓/止损）和风险提示。
4. 涉及价格时给出明确数字或区间。
5. 用中文 Markdown 输出，结构清晰。`

const defaultAnalysisSystemPromptPosition = `你是一位严谨的 A 股分析师，擅长把用户已配置的持仓成本、数量、组合权重和实时价格结合起来给出可执行的交易策略。
要求：
1. 只基于给定数据分析，不编造行情、财务或消息数据。
2. 本次只带入了用户勾选的持仓；不要假设未勾选股票、现金或总资产。
3. 以当前分析股票为主，结合组合权重/集中度给出持有/加仓/减仓/止损建议。
4. 涉及价格时给出明确数字或区间。
5. 用中文 Markdown 输出，结构清晰。`

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

func (a *mobileApp) startAnalysis(code string, withPosition bool, positionCodes []string) (string, error) {
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

	var selected []string
	if withPosition {
		selected = validPositionCodes(positionCodes)
		if len(selected) == 0 {
			return "", errors.New("请选择至少一只已配置成本/持仓的股票，或关闭持仓开关")
		}
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
	go a.runAnalysis(ctx, runID, norm, name, aiID, settings, withPosition, selected)
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

func (a *mobileApp) runAnalysis(ctx context.Context, runID, code, name string, aiConfigID int64, settings mobileSettings, withPosition bool, positionCodes []string) {
	logger.SugaredLogger.Infof("runAnalysis entered: run=%s code=%s name=%s ai=%d withPosition=%v positions=%d", runID, code, name, aiConfigID, withPosition, len(positionCodes))
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
	currentIncluded := withPosition && containsCode(positionCodes, code)
	kline := quickKLine(code)
	logger.SugaredLogger.Infof("mobile analysis: kline len=%d", len(kline))

	portfolioBlock := ""
	portfolioCount := 0
	if withPosition {
		portfolioBlock, portfolioCount = buildPortfolioPrompt(code, price, positionCodes)
	}
	logger.SugaredLogger.Infof("mobile analysis: portfolio positions=%d currentIncluded=%v", portfolioCount, currentIncluded)

	repl := analysisTemplateReplacer(name, code, price, cost, volume, currentIncluded)
	systemPrompt := buildAnalysisSystemPrompt(settings, repl, withPosition)
	dataBlock := buildAnalysisDataBlock(name, code, price, cost, volume, currentIncluded)
	userPrompt := buildAnalysisUserPrompt(repl, dataBlock, kline, withPosition, portfolioBlock)
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
// includePosition 为 false 时，成本和持仓占位符填“未提供”，避免模板把旧值/空值误当成持仓。
func analysisTemplateReplacer(name, code string, price, cost float64, volume int64, includePosition bool) *strings.Replacer {
	costText := "未提供"
	volText := "未提供"
	if includePosition {
		costText = "未设置"
		if cost > 0 {
			costText = fmt.Sprintf("%.2f", cost)
		}
		volText = "未设置"
		if volume > 0 {
			volText = fmt.Sprintf("%d", volume)
		}
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

func buildAnalysisSystemPrompt(settings mobileSettings, repl *strings.Replacer, withPosition bool) string {
	if settings.DefaultPromptID > 0 {
		if p := data.NewPromptTemplateApi().GetPromptTemplateByID(int(settings.DefaultPromptID)); strings.TrimSpace(p) != "" {
			return repl.Replace(p)
		}
	}
	if p := strings.TrimSpace(data.GetSettingConfig().Prompt); p != "" {
		return repl.Replace(p)
	}
	if withPosition {
		return defaultAnalysisSystemPromptPosition
	}
	return defaultAnalysisSystemPromptNeutral
}

func buildAnalysisUserPrompt(repl *strings.Replacer, dataBlock, kline string, withPosition bool, portfolioBlock string) string {
	custom := strings.TrimSpace(data.GetSettingConfig().QuestionTemplate)
	question := custom
	if question == "" {
		if withPosition {
			question = "请以当前股票 {{stockName}}（{{stockCode}}）为主，结合我的成本/持仓和已勾选的组合持仓，给出这只股票的操盘策略：\n" +
				"1. 当前趋势与关键支撑/压力位；\n" +
				"2. 该股在我已勾选持仓中的权重、集中度与组合风险；\n" +
				"3. 持有/加仓/减仓/止损，明确触发价格和数量；\n" +
				"4. 风险提示。\n" +
				"只分析当前股票，不要展开分析其他持仓；不要假设现金或未勾选股票。"
		} else {
			question = "请基于当前价和近 30 日 K 线分析 {{stockName}}（{{stockCode}}）：\n" +
				"1. 当前趋势与关键支撑/压力位；\n" +
				"2. 给出中性的操作建议（买入/持有/减仓/止损）及触发价格；\n" +
				"3. 风险提示。\n" +
				"本次未提供我的成本/持仓信息，请不要假设我的成本、仓位或现金。"
		}
	}
	question = repl.Replace(question)
	out := dataBlock + "\n\n"
	if withPosition && strings.TrimSpace(portfolioBlock) != "" {
		out += portfolioBlock + "\n\n"
	}
	out += question
	if !withPosition && custom != "" {
		out += "\n\n注意：本次未提供用户的成本、持仓或账户信息；不要假设我的成本、仓位或现金。"
	}
	if strings.TrimSpace(kline) != "" {
		out += "\n\n近 30 日 K 线：\n" + kline
	}
	return out
}

func buildAnalysisDataBlock(name, code string, price, cost float64, volume int64, includePosition bool) string {
	priceText := "未知"
	if price > 0 {
		priceText = fmt.Sprintf("%.2f", price)
	}
	out := fmt.Sprintf("当前数据：\n- 股票：%s（%s）\n- 当前价：%s", name, code, priceText)
	if !includePosition {
		return out
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
	return out + fmt.Sprintf("\n- 成本价：%s\n- 持仓数量：%s\n- 持仓盈亏：%s", costText, volText, plText)
}

// validPositionCodes 校验前端勾选的股票：必须在自选里且已配置成本/持仓；按请求顺序去重返回。
func validPositionCodes(requested []string) []string {
	follows := data.NewStockDataApi().GetFollowList(0)
	if follows == nil || len(*follows) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(*follows))
	for _, f := range *follows {
		n := normalizeCode(f.StockCode)
		if n != "" && f.CostPrice > 0 && f.Volume > 0 {
			allowed[n] = struct{}{}
		}
	}
	out := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, raw := range requested {
		n := normalizeCode(raw)
		if n == "" {
			continue
		}
		if _, ok := allowed[n]; !ok {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func containsCode(codes []string, code string) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

type portfolioRow struct {
	Code         string
	Name         string
	CostPrice    float64
	Volume       int64
	Price        float64
	MarketValue  float64
	CostAmount   float64
	ProfitAmount float64
	Weight       float64
	Fallback     bool
	NoQuote      bool
	Current      bool
}

// buildPortfolioPrompt 只统计用户本次勾选、且已配置成本/持仓的股票。
// 权重按勾选股票的总市值计算，不代表总资产仓位；行情缺失时用自选里上一次价格。
func buildPortfolioPrompt(currentCode string, currentPrice float64, codes []string) (string, int) {
	if len(codes) == 0 {
		return "", 0
	}
	follows := data.NewStockDataApi().GetFollowList(0)
	if follows == nil || len(*follows) == 0 {
		return "", 0
	}
	byCode := make(map[string]data.FollowedStock, len(*follows))
	for _, f := range *follows {
		byCode[normalizeCode(f.StockCode)] = f
	}
	quotes := quickQuotes(codes)

	rows := make([]portfolioRow, 0, len(codes))
	var totalMV, totalCost float64
	for _, code := range codes {
		f, ok := byCode[code]
		if !ok || f.CostPrice <= 0 || f.Volume <= 0 {
			continue
		}
		row := portfolioRow{
			Code:      code,
			Name:      f.Name,
			CostPrice: f.CostPrice,
			Volume:    f.Volume,
			Current:   code == currentCode,
		}
		if row.Name == "" {
			row.Name = stockDisplayName(code)
		}
		price := parsePrice(quotes[code].Price)
		if price <= 0 {
			price = f.Price
			row.Fallback = price > 0
		}
		if code == currentCode && currentPrice > 0 {
			price = currentPrice
			row.Fallback = false
		}
		row.Price = price
		if price <= 0 {
			row.NoQuote = true
		} else {
			row.MarketValue = price * float64(f.Volume)
			row.CostAmount = f.CostPrice * float64(f.Volume)
			row.ProfitAmount = row.MarketValue - row.CostAmount
			totalMV += row.MarketValue
			totalCost += row.CostAmount
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return "", 0
	}

	currentIncluded := false
	currentWeight := 0.0
	for i := range rows {
		if totalMV > 0 {
			rows[i].Weight = rows[i].MarketValue / totalMV * 100
		}
		if rows[i].Current {
			currentIncluded = true
			currentWeight = rows[i].Weight
		}
	}

	var sb strings.Builder
	sb.WriteString("已勾选的持仓（本次仅这些；占比按这些持仓的市值合计计算）：\n")
	sb.WriteString("|股票|代码|成本价|持仓|现价|市值|持仓成本|盈亏|占组合|\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	hasFallback := false
	hasNoQuote := false
	for _, r := range rows {
		name := r.Name
		if r.Current {
			name += "（当前分析）"
		}
		priceText := "—"
		mvText := "—"
		profitText := "—"
		weightText := "—"
		if !r.NoQuote {
			priceText = fmt.Sprintf("%.2f", r.Price)
			if r.Fallback {
				priceText += "*"
				hasFallback = true
			}
			mvText = fmt.Sprintf("%.2f", r.MarketValue)
			profitText = fmt.Sprintf("%+.2f", r.ProfitAmount)
			weightText = fmt.Sprintf("%.2f%%", r.Weight)
		} else {
			hasNoQuote = true
		}
		sb.WriteString(fmt.Sprintf("|%s|%s|%.2f|%d|%s|%s|%.2f|%s|%s|\n",
			name, r.Code, r.CostPrice, r.Volume, priceText, mvText, r.CostAmount, profitText, weightText))
	}
	sb.WriteString(fmt.Sprintf("\n合计：市值 %.2f 元，持仓成本 %.2f 元，盈亏 %+.2f 元。\n", totalMV, totalCost, totalMV-totalCost))
	if currentIncluded {
		sb.WriteString(fmt.Sprintf("当前股票在本次勾选组合中占 %.2f%%。\n", currentWeight))
	} else {
		sb.WriteString("当前股票不在本次勾选的持仓组合中。\n")
	}
	if hasFallback {
		sb.WriteString("注：带 * 的现价使用了上次行情。\n")
	}
	if hasNoQuote {
		sb.WriteString("注：— 表示暂无行情，相关市值/盈亏/占比未计入合计。\n")
	}
	sb.WriteString("备注：仅包含本次勾选的、已配置成本/持仓的自选股；不含现金、未勾选股票、未配置成本/持仓的股票；占比不代表总资产仓位。")
	return sb.String(), len(rows)
}

func quickQuotes(codes []string) map[string]data.StockInfo {
	ch := make(chan map[string]data.StockInfo, 1)
	go func() {
		q, _ := getQuotes(codes)
		ch <- q
	}()
	select {
	case q := <-ch:
		if q == nil {
			return map[string]data.StockInfo{}
		}
		return q
	case <-time.After(6 * time.Second):
		logger.SugaredLogger.Warnf("mobile: portfolio quote timeout, use stored prices")
		return map[string]data.StockInfo{}
	}
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
