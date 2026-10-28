package mobilecore

import (
	"context"
	"errors"
	"strings"
	"sync"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/backend/models"
)

// mobileApp 是 /api/call 的反射目标。方法必须导出（reflect.MethodByName 只能取导出方法），
// 类型本身保持非导出，避免 gomobile 生成不必要的 Java 绑定。
type mobileApp struct {
	ctx context.Context

	analysisMu     sync.Mutex
	analysisCancel context.CancelFunc
	analysisRunID  string
}

func newMobileApp(ctx context.Context) *mobileApp {
	return &mobileApp{ctx: ctx}
}

// ---------------- 自选 ----------------

func (a *mobileApp) GetWatchlist() ([]watchItem, error) {
	return loadWatchlist()
}

func (a *mobileApp) SearchStocks(keyword string) []stockBasicDTO {
	return searchStocks(keyword)
}

func (a *mobileApp) AddStock(code string) (string, error) {
	return addStock(code)
}

func (a *mobileApp) RemoveStock(code string) (string, error) {
	return removeStock(code)
}

func (a *mobileApp) SetPosition(code string, costPrice float64, volume int64) (string, error) {
	return setPosition(code, costPrice, volume)
}

func (a *mobileApp) GetStockDetail(code string) (*stockDetailDTO, error) {
	return stockDetail(code)
}

// ---------------- AI 分析 ----------------

func (a *mobileApp) StartAnalysis(code string) (string, error) {
	return a.startAnalysis(code)
}

func (a *mobileApp) CancelAnalysis() {
	a.cancelAnalysis()
}

func (a *mobileApp) GetAnalysis(code string) *analysisDTO {
	return getAnalysis(code)
}

func (a *mobileApp) ExtractAlerts(code string) ([]alertSuggestionDTO, error) {
	return extractAlerts(code)
}

// ---------------- AI 配置 ----------------

func (a *mobileApp) ListAIConfigs() []aiConfigDTO {
	configs := data.GetSettingConfig().AiConfigs
	out := make([]aiConfigDTO, 0, len(configs))
	for _, c := range configs {
		out = append(out, aiConfigDTOFromModel(c))
	}
	return out
}

func aiConfigDTOFromModel(c *data.AIConfig) aiConfigDTO {
	return aiConfigDTO{
		ID:               int64(c.ID),
		Name:             c.Name,
		BaseURL:          c.BaseUrl,
		APIKey:           c.ApiKey,
		ModelName:        c.ModelName,
		ModelType:        c.ModelType,
		MaxTokens:        c.MaxTokens,
		ContextWindow:    c.ContextWindow,
		Temperature:      c.Temperature,
		TimeOut:          c.TimeOut,
		HTTPProxy:        c.HttpProxy,
		HTTPProxyEnabled: c.HttpProxyEnabled,
		ExtraHeaders:     c.ExtraHeaders,
		Thinking:         c.Thinking,
		SupportVision:    c.SupportVision,
	}
}

func aiConfigFromDTO(c aiConfigDTO) *data.AIConfig {
	modelType := strings.TrimSpace(c.ModelType)
	if modelType == "" {
		modelType = "chat"
	}
	return &data.AIConfig{
		ID:               uint(c.ID),
		Name:             strings.TrimSpace(c.Name),
		BaseUrl:          strings.TrimSpace(c.BaseURL),
		ApiKey:           strings.TrimSpace(c.APIKey),
		ModelName:        strings.TrimSpace(c.ModelName),
		ModelType:        modelType,
		MaxTokens:        c.MaxTokens,
		ContextWindow:    c.ContextWindow,
		Temperature:      c.Temperature,
		TimeOut:          c.TimeOut,
		HttpProxy:        strings.TrimSpace(c.HTTPProxy),
		HttpProxyEnabled: c.HTTPProxyEnabled,
		ExtraHeaders:     c.ExtraHeaders,
		Thinking:         c.Thinking,
		SupportVision:    c.SupportVision,
	}
}

func (a *mobileApp) SaveAIConfig(cfg aiConfigDTO) (int64, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		return 0, errors.New("配置名称不能为空")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return 0, errors.New("BaseUrl 不能为空")
	}
	if strings.TrimSpace(cfg.ModelName) == "" {
		return 0, errors.New("模型名不能为空")
	}
	configs := data.GetSettingConfig().AiConfigs
	found := false
	for _, c := range configs {
		if int64(c.ID) != cfg.ID || cfg.ID <= 0 {
			continue
		}
		found = true
		c.Name = strings.TrimSpace(cfg.Name)
		c.BaseUrl = strings.TrimSpace(cfg.BaseURL)
		if strings.TrimSpace(cfg.APIKey) != "" {
			c.ApiKey = strings.TrimSpace(cfg.APIKey)
		}
		c.ModelName = strings.TrimSpace(cfg.ModelName)
		if strings.TrimSpace(cfg.ModelType) != "" {
			c.ModelType = strings.TrimSpace(cfg.ModelType)
		}
		c.MaxTokens = cfg.MaxTokens
		c.ContextWindow = cfg.ContextWindow
		c.Temperature = cfg.Temperature
		c.TimeOut = cfg.TimeOut
		c.HttpProxy = strings.TrimSpace(cfg.HTTPProxy)
		c.HttpProxyEnabled = cfg.HTTPProxyEnabled
		c.ExtraHeaders = cfg.ExtraHeaders
		c.Thinking = cfg.Thinking
		c.SupportVision = cfg.SupportVision
		break
	}
	if !found {
		configs = append(configs, aiConfigFromDTO(cfg))
	}
	if msg := data.UpdateAiConfigsOnly(configs); strings.Contains(msg, "失败") {
		return 0, errors.New(msg)
	}
	// 重新读取，找到刚保存的配置 ID（新配置由数据库自增）。
	saved := data.GetSettingConfig().AiConfigs
	for _, c := range saved {
		if c.Name == strings.TrimSpace(cfg.Name) &&
			c.BaseUrl == strings.TrimSpace(cfg.BaseURL) &&
			c.ModelName == strings.TrimSpace(cfg.ModelName) {
			s := loadMobileSettings()
			if s.DefaultAIConfigID == 0 {
				s.DefaultAIConfigID = int64(c.ID)
				_ = saveMobileSettings(s)
			}
			return int64(c.ID), nil
		}
	}
	return 0, nil
}

func (a *mobileApp) DeleteAIConfig(id int64) (string, error) {
	if id <= 0 {
		return "", errors.New("无效的配置 ID")
	}
	configs := data.GetSettingConfig().AiConfigs
	out := make([]*data.AIConfig, 0, len(configs))
	for _, c := range configs {
		if int64(c.ID) == id {
			continue
		}
		out = append(out, c)
	}
	if len(out) == len(configs) {
		return "", errors.New("配置不存在")
	}
	if msg := data.UpdateAiConfigsOnly(out); strings.Contains(msg, "失败") {
		return "", errors.New(msg)
	}
	s := loadMobileSettings()
	if s.DefaultAIConfigID == id {
		s.DefaultAIConfigID = 0
		_ = saveMobileSettings(s)
	}
	return "删除成功", nil
}

func (a *mobileApp) TestAIConfig(cfg aiConfigDTO) (string, error) {
	model := aiConfigFromDTO(cfg)
	if model.ApiKey == "" && cfg.ID > 0 {
		if stored, ok := findAIConfig(cfg.ID); ok {
			if model.ApiKey == "" {
				model.ApiKey = stored.ApiKey
			}
			if model.BaseUrl == "" {
				model.BaseUrl = stored.BaseUrl
			}
			if model.ModelName == "" {
				model.ModelName = stored.ModelName
			}
		}
	}
	if model.BaseUrl == "" || model.ModelName == "" {
		return "", errors.New("请先填写 BaseUrl 和模型名")
	}
	content, err := chatOnce(*model, "你是连通性测试助手。", "只回复 OK", 16)
	if err != nil {
		return "", err
	}
	return "连接成功：" + strings.TrimSpace(content), nil
}

// ---------------- 系统提示词 ----------------

func (a *mobileApp) ListPrompts() []promptDTO {
	res := data.NewPromptTemplateApi().GetPromptTemplates("", "")
	if res == nil {
		return []promptDTO{}
	}
	out := make([]promptDTO, 0, len(*res))
	for _, p := range *res {
		out = append(out, promptDTO{ID: int64(p.ID), Name: p.Name, Content: p.Content, Type: p.Type})
	}
	return out
}

func (a *mobileApp) SavePrompt(p promptDTO) (int64, error) {
	if strings.TrimSpace(p.Name) == "" {
		return 0, errors.New("模板名称不能为空")
	}
	if strings.TrimSpace(p.Content) == "" {
		return 0, errors.New("提示词内容不能为空")
	}
	typ := strings.TrimSpace(p.Type)
	if typ == "" {
		typ = "stock"
	}
	msg := data.NewPromptTemplateApi().AddPrompt(models.PromptTemplate{
		ID:      int(p.ID),
		Name:    strings.TrimSpace(p.Name),
		Content: p.Content,
		Type:    typ,
	})
	if strings.Contains(msg, "失败") {
		return 0, errors.New(msg)
	}
	// 回读 ID：AddPrompt 只返回中文提示，不返回主键。
	var t models.PromptTemplate
	if err := db.Dao.Where("name = ?", strings.TrimSpace(p.Name)).Order("id desc").First(&t).Error; err == nil {
		return int64(t.ID), nil
	}
	return 0, nil
}

func (a *mobileApp) DeletePrompt(id int64) (string, error) {
	if id <= 0 {
		return "", errors.New("无效的模板 ID")
	}
	msg := data.NewPromptTemplateApi().DelPrompt(uint(id))
	if strings.Contains(msg, "失败") {
		return "", errors.New(msg)
	}
	s := loadMobileSettings()
	if s.DefaultPromptID == id {
		s.DefaultPromptID = 0
		_ = saveMobileSettings(s)
	}
	return msg, nil
}

// ---------------- 价格提醒 ----------------

func (a *mobileApp) ListAlerts(code string) []alertDTO {
	return listAlerts(code)
}

func (a *mobileApp) SaveAlert(alert alertDTO) (int64, error) {
	return saveAlert(alert)
}

func (a *mobileApp) DeleteAlert(id int64) (string, error) {
	return deleteAlert(id)
}

func (a *mobileApp) SetAlertEnabled(id int64, enabled bool) (string, error) {
	return setAlertEnabled(id, enabled)
}

func (a *mobileApp) ListAlertEvents(limit int) []alertEventDTO {
	return listAlertEvents(limit)
}

// ---------------- 设置 ----------------

func (a *mobileApp) GetMobileSettings() mobileSettingsDTO {
	return settingsDTO(loadMobileSettings())
}

func (a *mobileApp) SaveMobileSettings(s mobileSettingsDTO) (string, error) {
	cur := loadMobileSettings()
	cur.DefaultAIConfigID = s.DefaultAIConfigID
	cur.DefaultPromptID = s.DefaultPromptID
	cur.AutoExtractAlerts = s.AutoExtractAlerts
	cur.PollIntervalSec = s.PollIntervalSec
	cur.EnableTools = s.EnableTools
	cur.Thinking = s.Thinking
	cur.PushEnabled = s.PushEnabled
	if err := saveMobileSettings(cur); err != nil {
		return "", err
	}
	return "保存成功", nil
}

func (a *mobileApp) GetVersionInfo() versionDTO {
	return versionDTO{Version: buildVersion}
}

// 便于排查：返回当前启用的提醒数量。
func (a *mobileApp) GetMonitorStatus() int {
	return lenAlertsEnabled()
}

// 供前端在“关于”页展示数据库路径（不暴露敏感内容）。
func (a *mobileApp) GetDataSummary() map[string]any {
	var follows, alerts int64
	db.Dao.Model(&data.FollowedStock{}).Count(&follows)
	db.Dao.Model(&priceAlert{}).Count(&alerts)
	return map[string]any{
		"followed": follows,
		"alerts":   alerts,
		"version":  buildVersion,
	}
}
