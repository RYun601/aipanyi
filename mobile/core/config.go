package mobilecore

import (
	"errors"

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
	return db.Dao.Save(&s).Error
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
