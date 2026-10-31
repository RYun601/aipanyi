package mobilecore

import (
	"errors"
	"fmt"
	"strings"

	"aipanyi/backend/data"
	"aipanyi/backend/logger"
	"aipanyi/mobile/alerts"
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
	return extractAlertsWithConfig(norm, analysis.Name, analysis.Content, mustPrice(norm), cfg)
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
		`{"alerts":[{"direction":"above|below","price":123.45,"note":"简短说明"}]}` + "\n" +
		"规则：\n" +
		"1. 只提取分析中明确给出的价格，不要自己推测。\n" +
		"2. direction=above 表示价格涨到/突破该价；below 表示跌到/跌破该价。\n" +
		"3. price 必须是数字，A 股保留两位小数。\n" +
		"4. 没有明确价格时输出 {\"alerts\":[]}。"

	content, err := chatOnce(cfg, extractSystemPrompt, userPrompt, 600)
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
		})
	}
	return out, nil
}

func fmtPriceOrUnknown(v float64) string {
	if v <= 0 {
		return "未知"
	}
	return fmt.Sprintf("%.2f", v)
}
