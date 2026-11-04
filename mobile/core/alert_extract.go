package mobilecore

import (
	"errors"
	"fmt"
	"strings"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/backend/logger"
	"aipanyi/mobile/alerts"
	"gorm.io/gorm"
)

const extractSystemPrompt = `你是一个交易策略结构化助手。你的任务是从一段股票分析中提取“价格提醒”条件。
只输出 JSON，不要输出 Markdown、解释或代码块。
如果没有明确价格，输出 {"alerts":[]}。`

func findAIConfig(id int64) (data.AIConfig, bool) {
	logger.SugaredLogger.Infof("findAIConfig: id=%d", id)
	configs := data.GetSettingConfig().AiConfigs
	logger.SugaredLogger.Infof("findAIConfig: configs=%d", len(configs))
	if len(configs) == 0 {
		return data.AIConfig{}, false
	}
	if id > 0 {
		for _, c := range configs {
			if int64(c.ID) == id {
				return *c, true
			}
		}
	}
	return *configs[0], true
}

func extractAlerts(code string) ([]alertSuggestionDTO, error) {
	norm := normalizeCode(code)
	if norm == "" {
		return nil, errors.New("股票代码为空")
	}
	analysis := getAnalysis(norm)
	if analysis == nil || strings.TrimSpace(analysis.Content) == "" {
		return nil, errors.New("还没有 AI 分析结果，请先分析该股票")
	}
	settings := loadMobileSettings()
	aiID := effectiveAIConfigID(settings)
	if aiID == 0 {
		return nil, errors.New("请先在设置里添加 AI 配置")
	}
	cfg, ok := findAIConfig(aiID)
	if !ok {
		return nil, errors.New("AI 配置不存在")
	}
	var basis analysisAlertBasis
	err := db.Dao.First(&basis, "chat_id = ?", analysis.ChatID).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("无法读取分析的持仓版本，请重试")
	}
	suggestions, err := extractAlertsWithConfig(norm, analysis.Name, analysis.Content, mustPrice(norm), cfg)
	if err == nil && basis.ChatID != "" {
		assignSuggestionBasis(suggestions, &basis.PositionRevision)
	}
	return suggestions, err
}

func extractAlertsWithConfig(code, name, analysis string, currentPrice float64, cfg data.AIConfig) ([]alertSuggestionDTO, error) {
	if strings.TrimSpace(analysis) == "" {
		return nil, errors.New("分析内容为空")
	}
	if len([]rune(analysis)) > 12000 {
		analysis = string([]rune(analysis)[:12000])
	}
	if strings.TrimSpace(name) == "" {
		name = stockDisplayName(code)
	}
	userPrompt := "股票：" + name + "（" + code + "）\n" +
		"当前价：" + fmtPriceOrUnknown(currentPrice) + "\n" +
		"分析内容：\n<analysis>\n" + analysis + "\n</analysis>\n\n" +
		"请提取所有明确的价格提醒。输出格式：\n" +
		`{"alerts":[{"direction":"above|below","price":123.45,"note":"简短说明","phase":"intraday|close","minVolumeRatio":null,"bookSide":"","minBookBias":null}]}` + "\n" +
		"规则：\n" +
		"1. 只提取分析中明确给出的价格，不要自己推测。\n" +
		"2. direction=above 表示价格涨到/突破该价；below 表示跌到/跌破该价。\n" +
		"3. price 必须是数字，A 股保留两位小数。\n" +
		"4. 保留收盘、放量、站稳等限定，收盘用 phase=close。只填写原文明示的量比、五档买卖盘方向（buy/sell）及偏向阈值（0 到 1）。缺失填 null 或空字符串。\n" +
		"5. 没有明确价格时输出 {\"alerts\":[]}。"

	// 0 表示按模型预算解析，不能用 600 这类小额度：思考模型的 reasoning
	// 会先耗尽额度，导致正文 JSON 为空。
	content, err := chatOnce(cfg, extractSystemPrompt, userPrompt, 0)
	if err != nil {
		return nil, err
	}
	return parseAlertSuggestions(content, code, name, currentPrice)
}

// parseAlertSuggestions 把模型返回的 JSON 解析成带校验结果的 DTO。
// 纯逻辑在 mobile/alerts，便于不依赖网络/数据库做单测。
func parseAlertSuggestions(content, code, name string, currentPrice float64) ([]alertSuggestionDTO, error) {
	sugs, err := alerts.ParseSuggestions(content, code, name, currentPrice)
	if err != nil {
		return nil, err
	}
	out := make([]alertSuggestionDTO, 0, len(sugs))
	for _, s := range sugs {
		out = append(out, alertSuggestionDTO{
			Code:      s.Code,
			Name:      s.Name,
			Direction: s.Direction,
			Price:     s.Price,
			Note:      s.Note,
			Valid:     s.Valid,
			Reason:    s.Reason,
			Mode:      s.Mode, Phase: s.Phase, MinVolumeRatio: s.MinVolumeRatio, BookSide: s.BookSide, MinBookBias: s.MinBookBias,
		})
	}
	return out, nil
}

func assignSuggestionBasis(suggestions []alertSuggestionDTO, revision *int64) {
	for i := range suggestions {
		suggestions[i].PositionRevision = revision
	}
}

func fmtPriceOrUnknown(v float64) string {
	if v <= 0 {
		return "未知"
	}
	return fmt.Sprintf("%.2f", v)
}
