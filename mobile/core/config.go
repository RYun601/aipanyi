package mobilecore

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
)

const settingsRowID = 1

func defaultPollInterval() int { return 30 }

func loadMobileSettings() mobileSettings {
	s := mobileSettings{}
	err := db.Dao.Where("id = ?", settingsRowID).First(&s).Error
	if err == nil {
		return normalizeSettings(s)
	}
	s = mobileSettings{
		ID:                settingsRowID,
		DefaultAIConfigID: 0,
		DefaultPromptID:   0,
		AutoExtractAlerts: true,
		PollIntervalSec:   defaultPollInterval(),
		EnableTools:       true,
		Thinking:          false,
		PushEnabled:       true,
	}
	if configs := data.GetSettingConfig().AiConfigs; len(configs) > 0 {
		s.DefaultAIConfigID = int64(configs[0].ID)
	}
	_ = db.Dao.Create(&s).Error
	return s
}

func normalizeSettings(s mobileSettings) mobileSettings {
	if s.PollIntervalSec < 10 {
		s.PollIntervalSec = 10
	}
	if s.PollIntervalSec > 300 {
		s.PollIntervalSec = 300
	}
	return s
}

func saveMobileSettings(s mobileSettings) error {
	s.ID = settingsRowID
	s = normalizeSettings(s)
	if s.PollIntervalSec < 10 {
		return errors.New("轮询间隔不能小于 10 秒")
	}
	// The trade fee is saved separately, so a stale settings form cannot overwrite it.
	return db.Dao.Omit("TradeFeeRate", "TradeFeeRulesJSON").Save(&s).Error
}

func normalizeFixedFeeRate(raw string) (string, error) {
	rate := strings.TrimSpace(raw)
	if rate != "" {
		fixed, err := parseTradeMoney(rate)
		if err != nil || fixed > 100*tradeScale {
			return "", errors.New("交易费率须为 0 到 100 的百分数，最多 4 位小数")
		}
		rate = compactTradeDecimal(fixed)
	}
	return rate, nil
}

func saveTradeFeeRate(raw string) (string, error) {
	rate, err := normalizeFixedFeeRate(raw)
	if err != nil {
		return "", err
	}
	loadMobileSettings()
	result := db.Dao.Model(&mobileSettings{}).Where("id = ?", settingsRowID).UpdateColumn("trade_fee_rate", rate)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected != 1 {
		return "", errors.New("无法保存交易费率")
	}
	return rate, nil
}

func compactTradeDecimal(fixed int64) string {
	return strings.TrimRight(strings.TrimRight(formatTradeMoney(fixed), "0"), ".")
}

func normalizeTradeFeeRule(rule tradeFeeRule) (tradeFeeRule, error) {
	switch rule.Mode {
	case "fixed":
		rate, err := normalizeFixedFeeRate(rule.Rate)
		return tradeFeeRule{Mode: "fixed", Rate: rate}, err
	case "aShare":
		commission, err := parseTradeMoney(rule.Commission)
		if err != nil || commission > 10_000*tradeScale {
			return tradeFeeRule{}, errors.New("请填写有效的佣金万分比，最多 4 位小数")
		}
		minimum, err := parseTradeMoney(rule.Minimum)
		if err != nil || minimum%100 != 0 {
			return tradeFeeRule{}, errors.New("请填写非负的最低佣金（元），精确到分")
		}
		return tradeFeeRule{Mode: "aShare", Commission: compactTradeDecimal(commission), Minimum: compactTradeDecimal(minimum)}, nil
	default:
		return tradeFeeRule{}, errors.New("请选择费用计算方式")
	}
}

func readTradeFeeRules(s mobileSettings) tradeFeeRules {
	var rules tradeFeeRules
	if s.TradeFeeRulesJSON != "" && json.Unmarshal([]byte(s.TradeFeeRulesJSON), &rules) == nil {
		return rules
	}
	legacy := tradeFeeRule{Mode: "fixed", Rate: s.TradeFeeRate}
	return tradeFeeRules{Buy: legacy, Sell: legacy}
}

func saveTradeFeeRules(input tradeFeeRules) (tradeFeeRules, error) {
	buy, err := normalizeTradeFeeRule(input.Buy)
	if err != nil {
		return tradeFeeRules{}, fmt.Errorf("买入费用：%w", err)
	}
	sell, err := normalizeTradeFeeRule(input.Sell)
	if err != nil {
		return tradeFeeRules{}, fmt.Errorf("卖出费用：%w", err)
	}
	rules := tradeFeeRules{Buy: buy, Sell: sell}
	payload, err := json.Marshal(rules)
	if err != nil {
		return tradeFeeRules{}, err
	}
	loadMobileSettings()
	result := db.Dao.Model(&mobileSettings{}).Where("id = ?", settingsRowID).
		UpdateColumn("trade_fee_rules_json", string(payload))
	if result.Error != nil {
		return tradeFeeRules{}, result.Error
	}
	if result.RowsAffected != 1 {
		return tradeFeeRules{}, errors.New("无法保存买卖费用")
	}
	return rules, nil
}

func settingsDTO(s mobileSettings) mobileSettingsDTO {
	return mobileSettingsDTO{
		DefaultAIConfigID: s.DefaultAIConfigID,
		DefaultPromptID:   s.DefaultPromptID,
		AutoExtractAlerts: s.AutoExtractAlerts,
		PollIntervalSec:   s.PollIntervalSec,
		EnableTools:       s.EnableTools,
		Thinking:          s.Thinking,
		PushEnabled:       s.PushEnabled,
		TradeFeeRate:      s.TradeFeeRate,
		TradeFeeRules:     readTradeFeeRules(s),
	}
}

// effectiveAIConfigID 返回当前应使用的 AI 配置：默认配置存在则用它，否则用第一条。
func effectiveAIConfigID(s mobileSettings) int64 {
	configs := data.GetSettingConfig().AiConfigs
	if len(configs) == 0 {
		return 0
	}
	if s.DefaultAIConfigID > 0 {
		for _, c := range configs {
			if int64(c.ID) == s.DefaultAIConfigID {
				return s.DefaultAIConfigID
			}
		}
	}
	return int64(configs[0].ID)
}

func effectivePromptID(s mobileSettings) int {
	if s.DefaultPromptID <= 0 {
		return 0
	}
	// 模板被删除后回退到内置提示词，避免分析直接失败。
	var count int64
	db.Dao.Table("prompt_templates").Where("id = ?", s.DefaultPromptID).Count(&count)
	if count == 0 {
		return 0
	}
	return int(s.DefaultPromptID)
}
