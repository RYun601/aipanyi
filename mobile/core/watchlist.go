package mobilecore

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
)

func normalizeCode(code string) string {
	return data.NormalizeStockCode(strings.TrimSpace(code))
}

// marketOf 按归一化代码判断市场，用于交易时段判断与 UI 展示。
func marketOf(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	switch {
	case strings.HasPrefix(c, "hk"):
		return "HK"
	case strings.HasPrefix(c, "gb_"), strings.HasPrefix(c, "us"):
		return "US"
	default:
		return "A"
	}
}

func parsePrice(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func quoteTime(q data.StockInfo) string {
	d := strings.TrimSpace(q.Date)
	t := strings.TrimSpace(q.Time)
	switch {
	case d != "" && t != "":
		return d + " " + t
	case d != "":
		return d
	default:
		return t
	}
}

func getQuotes(codes []string) (map[string]data.StockInfo, error) {
	out := make(map[string]data.StockInfo, len(codes))
	if len(codes) == 0 {
		return out, nil
	}
	res, err := data.NewStockDataApi().GetStockCodeRealTimeData(codes...)
	if res != nil {
		for _, q := range *res {
			out[normalizeCode(q.Code)] = q
		}
	}
	// 行情源可能部分失败（例如 A 股成功、美股源超时）；有数据就先用，避免整批丢弃。
	if len(out) > 0 {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func buildWatchItem(f data.FollowedStock, q data.StockInfo, hasQuote bool) watchItem {
	price := 0.0
	changeAmount := 0.0
	changePercent := 0.0
	qt := ""
	if hasQuote {
		price = parsePrice(q.Price)
		base := parsePrice(q.PreClose)
		if base <= 0 {
			base = q.PrePrice
		}
		if price > 0 && base > 0 {
			changeAmount = price - base
			changePercent = (price/base - 1) * 100
		}
		qt = quoteTime(q)
	}
	if price <= 0 {
		price = f.Price
		changeAmount = f.PriceChange
		changePercent = f.ChangePercent
		hasQuote = false
	}

	item := watchItem{
		Code:          f.StockCode,
		Name:          f.Name,
		Market:        marketOf(f.StockCode),
		Price:         price,
		ChangeAmount:  changeAmount,
		ChangePercent: changePercent,
		CostPrice:     f.CostPrice,
		Volume:        f.Volume,
		HasQuote:      hasQuote,
		Followed:      true,
		QuoteTime:     qt,
	}
	if f.CostPrice > 0 && f.Volume > 0 && price > 0 {
		item.ProfitAmount = (price - f.CostPrice) * float64(f.Volume)
		item.ProfitPercent = (price/f.CostPrice - 1) * 100
	}
	return item
}

func loadWatchlist() ([]watchItem, error) {
	follows := data.NewStockDataApi().GetFollowList(0)
	if follows == nil || len(*follows) == 0 {
		return []watchItem{}, nil
	}
	codes := make([]string, 0, len(*follows))
	for _, f := range *follows {
		codes = append(codes, f.StockCode)
	}
	quotes, err := getQuotes(codes)
	if err != nil {
		// 行情失败时仍返回自选列表（使用库里上次价格），让用户能编辑成本/持仓。
		quotes = map[string]data.StockInfo{}
	}
	out := make([]watchItem, 0, len(*follows))
	for _, f := range *follows {
		q, ok := quotes[normalizeCode(f.StockCode)]
		out = append(out, buildWatchItem(f, q, ok))
	}
	return out, nil
}

func searchStocks(keyword string) []stockBasicDTO {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []stockBasicDTO{}
	}
	res := data.NewStockDataApi().GetStockList(keyword)
	out := make([]stockBasicDTO, 0, len(res))
	for _, s := range res {
		if s.TsCode == "" || s.Name == "" {
			continue
		}
		out = append(out, stockBasicDTO{
			Code:   s.TsCode,
			Name:   s.Name,
			Symbol: s.Symbol,
			Market: marketOf(s.TsCode),
		})
		if len(out) >= 30 {
			break
		}
	}
	return out
}

func addStock(code string) (string, error) {
	norm := normalizeCode(code)
	if norm == "" {
		return "", errors.New("股票代码为空")
	}
	msg := data.NewStockDataApi().Follow(norm)
	if msg == "关注失败" {
		return "", errors.New("关注失败：获取行情失败或代码不支持")
	}
	return msg, nil
}

func removeStock(code string) (string, error) {
	norm := normalizeCode(code)
	if norm == "" {
		return "", errors.New("股票代码为空")
	}
	msg := data.NewStockDataApi().UnFollow(norm)
	// 取消关注后同步关闭该股票的提醒，避免继续为不在自选里的股票发通知。
	_ = db.Dao.Model(&priceAlert{}).Where("stock_code = ?", norm).Update("enabled", false).Error
	updateMonitorStatus(lenAlertsEnabled())
	return msg, nil
}

func setPosition(code string, costPrice float64, volume int64) (string, error) {
	norm := normalizeCode(code)
	if norm == "" {
		return "", errors.New("股票代码为空")
	}
	if costPrice < 0 || volume < 0 {
		return "", errors.New("成本价和持仓数量不能为负数")
	}
	f := data.NewStockDataApi().GetFollowedStockByStockCode(norm)
	if f.StockCode == "" {
		return "", errors.New("请先关注该股票")
	}
	msg := data.NewStockDataApi().SetCostPriceAndVolume(costPrice, volume, norm)
	if strings.HasPrefix(msg, "设置失败") {
		return "", errors.New(msg)
	}
	return msg, nil
}

func stockDetail(code string) (*stockDetailDTO, error) {
	norm := normalizeCode(code)
	if norm == "" {
		return nil, errors.New("股票代码为空")
	}
	f := data.NewStockDataApi().GetFollowedStockByStockCode(norm)
	quotes, _ := getQuotes([]string{norm})
	q, hasQuote := quotes[norm]
	item := buildWatchItem(f, q, hasQuote)
	item.Code = norm
	item.Market = marketOf(norm)
	if f.StockCode == "" {
		item.Followed = false
		if hasQuote {
			item.Name = q.Name
		}
	}
	return &stockDetailDTO{
		Item:     item,
		Alerts:   listAlerts(norm),
		Analysis: getAnalysis(norm),
	}, nil
}

func mustPrice(code string) float64 {
	quotes, err := getQuotes([]string{normalizeCode(code)})
	if err != nil {
		return 0
	}
	q, ok := quotes[normalizeCode(code)]
	if !ok {
		return 0
	}
	return parsePrice(q.Price)
}

func stockDisplayName(code string) string {
	norm := normalizeCode(code)
	f := data.NewStockDataApi().GetFollowedStockByStockCode(norm)
	if f.Name != "" {
		return f.Name
	}
	var basic data.StockBasic
	if ts := tsCodeFromNormalized(norm); ts != "" {
		db.Dao.Where("ts_code = ?", ts).First(&basic)
		if basic.Name != "" {
			return basic.Name
		}
	}
	return norm
}

func tsCodeFromNormalized(norm string) string {
	switch {
	case strings.HasPrefix(norm, "sh"):
		return strings.ToUpper(strings.TrimPrefix(norm, "sh")) + ".SH"
	case strings.HasPrefix(norm, "sz"):
		return strings.ToUpper(strings.TrimPrefix(norm, "sz")) + ".SZ"
	case strings.HasPrefix(norm, "bj"):
		return strings.ToUpper(strings.TrimPrefix(norm, "bj")) + ".BJ"
	case strings.HasPrefix(norm, "hk"):
		return strings.ToUpper(strings.TrimPrefix(norm, "hk")) + ".HK"
	default:
		return ""
	}
}

func fmtPrice(v float64) string {
	return fmt.Sprintf("%.2f", v)
}
