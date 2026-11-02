package data

import (
	"fmt"
	"strings"
	"time"

	"aipanyi/backend/logger"

	"github.com/duke-git/lancet/v2/convertor"
)

// PositionContext 是桌面端股票 AI 分析时显式带入的持仓上下文。
//
// nil 表示旧行为（自动带入当前股票的成本/持仓，供定时任务等旧调用方使用）；
// Enabled=false 表示用户关闭了持仓带入；Enabled=true 时 Markdown 只包含本次
// 勾选、且已配置成本价和持仓数量的股票。
type PositionContext struct {
	Enabled         bool
	CurrentIncluded bool
	Count           int
	Markdown        string
}

// applyPositionTemplates 按持仓开关填充用户提示词里的成本/持仓占位符。
// nil 表示旧调用方（自动带入当前股票）；非 nil 时关闭开关或当前股票未勾选都填“未提供”。
func applyPositionTemplates(replaceTemplates map[string]string, followedStock FollowedStock, position *PositionContext) {
	includeCurrentPosition := position == nil
	if position != nil && position.Enabled && position.CurrentIncluded {
		includeCurrentPosition = true
	}
	unavailablePositionText := "未设置"
	if position != nil {
		unavailablePositionText = "未提供"
	}

	setPositionTemplate := func(name, value string) {
		replaceTemplates["{{"+name+"}}"] = value
		replaceTemplates["{"+name+"}"] = value
		replaceTemplates[name] = value
	}
	if includeCurrentPosition && followedStock.CostPrice > 0 {
		setPositionTemplate("costPrice", convertor.ToString(followedStock.CostPrice))
	} else {
		// 未设置成本时也替换占位符，避免模板里出现裸 {{costPrice}} 被模型当成真实数据。
		setPositionTemplate("costPrice", unavailablePositionText)
	}
	if includeCurrentPosition && followedStock.Volume > 0 {
		setPositionTemplate("costVolume", convertor.ToString(followedStock.Volume))
	} else {
		setPositionTemplate("costVolume", unavailablePositionText)
	}
}

type positionRow struct {
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

// BuildPositionContext 读取自选列表和行情，构造本次勾选股票的持仓上下文。
// 只接受已配置成本价和持仓数量（均大于 0）的自选股；行情最多等待 6 秒，
// 超时或缺失时回退到 followed_stock 中上一次保存的价格。
func BuildPositionContext(currentCode string, currentPrice float64, codes []string) *PositionContext {
	ctx := &PositionContext{Enabled: true}
	follows := NewStockDataApi().GetFollowList(0)
	if follows == nil || len(*follows) == 0 {
		return ctx
	}

	selected := normalizeSelectedPositionCodes(*follows, codes)
	if len(selected) == 0 {
		return ctx
	}

	quotes := fetchPositionQuotes(selected)
	return buildPositionContext(currentCode, currentPrice, *follows, quotes, selected)
}

// normalizeSelectedPositionCodes 校验并去重前端勾选的股票。
// 只保留自选里已配置成本价和持仓数量的股票，保持请求顺序。
func normalizeSelectedPositionCodes(follows []FollowedStock, codes []string) []string {
	if len(codes) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(follows))
	for _, f := range follows {
		n := normalizeStockCode(f.StockCode)
		if n != "" && f.CostPrice > 0 && f.Volume > 0 {
			allowed[n] = struct{}{}
		}
	}
	out := make([]string, 0, len(codes))
	seen := make(map[string]struct{}, len(codes))
	for _, raw := range codes {
		n := normalizeStockCode(strings.TrimSpace(raw))
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

func fetchPositionQuotes(codes []string) map[string]StockInfo {
	out := make(map[string]StockInfo, len(codes))
	if len(codes) == 0 {
		return out
	}

	done := make(chan map[string]StockInfo, 1)
	go func() {
		res, _ := NewStockDataApi().GetStockCodeRealTimeData(codes...)
		quotes := make(map[string]StockInfo, len(codes))
		if res != nil {
			for _, q := range *res {
				quotes[normalizeStockCode(q.Code)] = q
			}
		}
		done <- quotes
	}()

	select {
	case quotes := <-done:
		if quotes != nil {
			return quotes
		}
	case <-time.After(6 * time.Second):
		logger.SugaredLogger.Warnf("desktop: position quote timeout, use stored prices")
	}
	return out
}

func buildPositionContext(currentCode string, currentPrice float64, follows []FollowedStock, quotes map[string]StockInfo, codes []string) *PositionContext {
	ctx := &PositionContext{Enabled: true}
	selected := normalizeSelectedPositionCodes(follows, codes)
	if len(selected) == 0 {
		return ctx
	}

	byCode := make(map[string]FollowedStock, len(follows))
	for _, f := range follows {
		byCode[normalizeStockCode(f.StockCode)] = f
	}
	normCurrent := normalizeStockCode(currentCode)

	rows := make([]positionRow, 0, len(selected))
	var totalMV, totalCost float64
	for _, code := range selected {
		f, ok := byCode[code]
		if !ok || f.CostPrice <= 0 || f.Volume <= 0 {
			continue
		}

		row := positionRow{
			Code:       code,
			Name:       f.Name,
			CostPrice:  f.CostPrice,
			Volume:     f.Volume,
			CostAmount: f.CostPrice * float64(f.Volume),
			Current:    code == normCurrent,
		}
		if row.Name == "" {
			row.Name = code
		}

		price := 0.0
		if q, ok := quotes[code]; ok {
			price, _ = convertor.ToFloat(q.Price)
		}
		if price <= 0 {
			price = f.Price
			row.Fallback = price > 0
		}
		if code == normCurrent && currentPrice > 0 {
			price = currentPrice
			row.Fallback = false
		}
		row.Price = price
		if price <= 0 {
			row.NoQuote = true
		} else {
			row.MarketValue = price * float64(f.Volume)
			row.ProfitAmount = row.MarketValue - row.CostAmount
			totalMV += row.MarketValue
			totalCost += row.CostAmount
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return ctx
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

	ctx.Count = len(rows)
	ctx.CurrentIncluded = currentIncluded
	ctx.Markdown = sb.String()
	return ctx
}
