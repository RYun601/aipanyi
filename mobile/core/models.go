package mobilecore

import "time"

// 说明：本包只把 Start/Stop/Notifier 暴露给 gomobile。
// 其余类型与 MobileApp 方法都保持非导出，避免 gomobile 为 time.Time 等
// 不支持的字段生成 Java 绑定；这些方法只由 webserver 通过反射调用。

// ---------------- 表模型（内部使用） ----------------

type priceAlert struct {
	ID              uint `gorm:"primarykey"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	StockCode       string     `gorm:"size:20;index" json:"stockCode"`
	StockName       string     `gorm:"size:50" json:"stockName"`
	Market          string     `gorm:"size:10" json:"market"`
	Direction       string     `gorm:"size:10" json:"direction"`
	Price           float64    `json:"price"`
	Note            string     `gorm:"size:200" json:"note"`
	Source          string     `gorm:"size:10" json:"source"`
	Enabled         bool       `gorm:"index" json:"enabled"`
	LastState       string     `gorm:"size:10;default:unknown" json:"lastState"`
	LastTriggeredAt *time.Time `json:"lastTriggeredAt"`
	TriggerCount    int        `gorm:"default:0" json:"triggerCount"`
}

func (priceAlert) TableName() string { return "mobile_price_alert" }

type priceAlertEvent struct {
	ID          uint `gorm:"primarykey"`
	CreatedAt   time.Time
	AlertID     uint      `gorm:"index" json:"alertId"`
	StockCode   string    `gorm:"size:20;index" json:"stockCode"`
	StockName   string    `gorm:"size:50" json:"stockName"`
	Price       float64   `json:"price"`
	Threshold   float64   `json:"threshold"`
	Direction   string    `gorm:"size:10" json:"direction"`
	TriggeredAt time.Time `gorm:"index" json:"triggeredAt"`
}

func (priceAlertEvent) TableName() string { return "mobile_price_alert_event" }

type mobileSettings struct {
	ID                uint `gorm:"primarykey"`
	DefaultAIConfigID int64
	DefaultPromptID   int64
	AutoExtractAlerts bool
	PollIntervalSec   int
	EnableTools       bool
	Thinking          bool
	PushEnabled       bool
}

func (mobileSettings) TableName() string { return "mobile_settings" }

// ---------------- 前端 DTO ----------------

type watchItem struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Market        string  `json:"market"`
	Price         float64 `json:"price"`
	ChangeAmount  float64 `json:"changeAmount"`
	ChangePercent float64 `json:"changePercent"`
	CostPrice     float64 `json:"costPrice"`
	Volume        int64   `json:"volume"`
	ProfitAmount  float64 `json:"profitAmount"`
	ProfitPercent float64 `json:"profitPercent"`
	HasQuote      bool    `json:"hasQuote"`
	Followed      bool    `json:"followed"`
	QuoteTime     string  `json:"quoteTime"`
}

type stockBasicDTO struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	Market string `json:"market"`
}

type alertDTO struct {
	ID              int64   `json:"id"`
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	Market          string  `json:"market"`
	Direction       string  `json:"direction"`
	Price           float64 `json:"price"`
	Note            string  `json:"note"`
	Source          string  `json:"source"`
	Enabled         bool    `json:"enabled"`
	LastState       string  `json:"lastState"`
	LastTriggeredAt string  `json:"lastTriggeredAt"`
	TriggerCount    int64   `json:"triggerCount"`
	CreatedAt       string  `json:"createdAt"`
}

type alertEventDTO struct {
	ID          int64   `json:"id"`
	AlertID     int64   `json:"alertId"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Threshold   float64 `json:"threshold"`
	Direction   string  `json:"direction"`
	TriggeredAt string  `json:"triggeredAt"`
}

type alertSuggestionDTO struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Direction string  `json:"direction"`
	Price     float64 `json:"price"`
	Note      string  `json:"note"`
	Valid     bool    `json:"valid"`
	Reason    string  `json:"reason"`
}

type analysisDTO struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Question  string `json:"question"`
	ModelName string `json:"modelName"`
	ChatID    string `json:"chatId"`
	CreatedAt string `json:"createdAt"`
}

type stockDetailDTO struct {
	Item     watchItem    `json:"item"`
	Alerts   []alertDTO   `json:"alerts"`
	Analysis *analysisDTO `json:"analysis,omitempty"`
}

type aiConfigDTO struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	BaseURL          string  `json:"baseUrl"`
	APIKey           string  `json:"apiKey"`
	ModelName        string  `json:"modelName"`
	ModelType        string  `json:"modelType"`
	MaxTokens        int     `json:"maxTokens"`
	ContextWindow    int     `json:"contextWindow"`
	Temperature      float64 `json:"temperature"`
	TimeOut          int     `json:"timeOut"`
	HTTPProxy        string  `json:"httpProxy"`
	HTTPProxyEnabled bool    `json:"httpProxyEnabled"`
	ExtraHeaders     string  `json:"extraHeaders"`
	Thinking         bool    `json:"thinking"`
	SupportVision    bool    `json:"supportVision"`
}

type promptDTO struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Type    string `json:"type"`
}

type mobileSettingsDTO struct {
	DefaultAIConfigID int64 `json:"defaultAiConfigId"`
	DefaultPromptID   int64 `json:"defaultPromptId"`
	AutoExtractAlerts bool  `json:"autoExtractAlerts"`
	PollIntervalSec   int   `json:"pollIntervalSec"`
	EnableTools       bool  `json:"enableTools"`
	Thinking          bool  `json:"thinking"`
	PushEnabled       bool  `json:"pushEnabled"`
}

type versionDTO struct {
	Version string `json:"version"`
	Build   string `json:"build"`
}
