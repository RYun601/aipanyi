package mobilecore

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/backend/logger"
	"aipanyi/backend/marketcal"
)

const maxEnabledAlerts = 50

func alertDTOFromModel(a priceAlert) alertDTO {
	dto := alertDTO{
		ID:           int64(a.ID),
		Code:         a.StockCode,
		Name:         a.StockName,
		Market:       a.Market,
		Direction:    a.Direction,
		Price:        a.Price,
		Note:         a.Note,
		Source:       a.Source,
		Enabled:      a.Enabled,
		LastState:    a.LastState,
		TriggerCount: int64(a.TriggerCount),
		CreatedAt:    a.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if a.LastTriggeredAt != nil {
		dto.LastTriggeredAt = a.LastTriggeredAt.Format("2006-01-02 15:04:05")
	}
	return dto
}

func alertEventDTOFromModel(e priceAlertEvent) alertEventDTO {
	return alertEventDTO{
		ID:          int64(e.ID),
		AlertID:     int64(e.AlertID),
		Code:        e.StockCode,
		Name:        e.StockName,
		Price:       e.Price,
		Threshold:   e.Threshold,
		Direction:   e.Direction,
		TriggeredAt: e.TriggeredAt.Format("2006-01-02 15:04:05"),
	}
}

func listAlerts(code string) []alertDTO {
	query := db.Dao.Model(&priceAlert{})
	if strings.TrimSpace(code) != "" {
		query = query.Where("stock_code = ?", normalizeCode(code))
	}
	var rows []priceAlert
	if err := query.Order("enabled desc, created_at desc").Find(&rows).Error; err != nil {
		return []alertDTO{}
	}
	out := make([]alertDTO, 0, len(rows))
	for _, a := range rows {
		out = append(out, alertDTOFromModel(a))
	}
	return out
}

func lenAlertsEnabled() int {
	var count int64
	db.Dao.Model(&priceAlert{}).Where("enabled = ?", true).Count(&count)
	return int(count)
}

func saveAlert(in alertDTO) (int64, error) {
	norm := normalizeCode(in.Code)
	if norm == "" {
		return 0, errors.New("股票代码为空")
	}
	direction := strings.ToLower(strings.TrimSpace(in.Direction))
	if direction != "above" && direction != "below" {
		return 0, errors.New("提醒方向只能是 above 或 below")
	}
	if in.Price <= 0 {
		return 0, errors.New("提醒价格必须大于 0")
	}
	f := data.NewStockDataApi().GetFollowedStockByStockCode(norm)
	if f.StockCode == "" {
		return 0, errors.New("请先关注该股票")
	}
	name := f.Name
	if name == "" {
		name = stockDisplayName(norm)
	}
	note := strings.TrimSpace(in.Note)
	if len([]rune(note)) > 100 {
		note = string([]rune(note)[:100])
	}
	source := strings.TrimSpace(in.Source)
	if source != "ai" {
		source = "manual"
	}
	market := marketOf(norm)

	// 更新已有规则
	if in.ID > 0 {
		var existing priceAlert
		if err := db.Dao.First(&existing, uint(in.ID)).Error; err != nil {
			return 0, errors.New("提醒规则不存在")
		}
		if in.Enabled && !existing.Enabled && lenAlertsEnabled() >= maxEnabledAlerts {
			return 0, fmt.Errorf("最多同时启用 %d 条价格提醒", maxEnabledAlerts)
		}
		updates := map[string]any{
			"stock_code": norm,
			"stock_name": name,
			"market":     market,
			"direction":  direction,
			"price":      in.Price,
			"note":       note,
			"source":     source,
			"enabled":    in.Enabled,
			// 改了价格/方向后重新武装，避免拿旧状态误触发。
			"last_state": "unknown",
		}
		if err := db.Dao.Model(&priceAlert{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return 0, err
		}
		updateMonitorStatus(lenAlertsEnabled())
		return int64(existing.ID), nil
	}

	// 新建
	if in.Enabled && lenAlertsEnabled() >= maxEnabledAlerts {
		return 0, fmt.Errorf("最多同时启用 %d 条价格提醒", maxEnabledAlerts)
	}
	var dup int64
	db.Dao.Model(&priceAlert{}).
		Where("stock_code = ? AND direction = ? AND price = ? AND enabled = ?", norm, direction, in.Price, true).
		Count(&dup)
	if dup > 0 {
		return 0, errors.New("已存在相同的价格提醒")
	}

	a := priceAlert{
		StockCode: norm,
		StockName: name,
		Market:    market,
		Direction: direction,
		Price:     in.Price,
		Note:      note,
		Source:    source,
		Enabled:   in.Enabled,
		LastState: "unknown",
	}
	if err := db.Dao.Create(&a).Error; err != nil {
		return 0, err
	}

	// 创建时若已在触发侧，盘中提醒一次。行情请求放到 goroutine 里，
	// 避免行情源慢/超时把“保存提醒”的 HTTP 请求拖住几分钟。
	if a.Enabled {
		go applyInitialAlertState(a)
	}
	updateMonitorStatus(lenAlertsEnabled())
	return int64(a.ID), nil
}

// applyInitialAlertState 新建规则时初始化 last_state；若当前价已在触发侧，
// 交易时段内立即提醒一次。非交易时段只置位，等下一次穿越。
func applyInitialAlertState(a priceAlert) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("mobile: init alert state panic: %v", r)
		}
	}()
	now := time.Now()
	if !marketcal.IsTradingTimeForMarket(a.Market, now) {
		return
	}
	p := mustPrice(a.StockCode)
	if p <= 0 {
		return
	}
	if (a.Direction == "above" && p >= a.Price) || (a.Direction == "below" && p <= a.Price) {
		state := "above"
		if a.Direction == "below" {
			state = "below"
		}
		triggerPriceAlert(a, p, state, now)
		return
	}
	state := "above"
	if a.Direction == "above" {
		state = "below"
	}
	_ = setAlertState(a.ID, state)
}

func deleteAlert(id int64) (string, error) {
	if id <= 0 {
		return "", errors.New("无效的提醒 ID")
	}
	if err := db.Dao.Delete(&priceAlert{}, uint(id)).Error; err != nil {
		return "", err
	}
	updateMonitorStatus(lenAlertsEnabled())
	return "删除成功", nil
}

func setAlertEnabled(id int64, enabled bool) (string, error) {
	if id <= 0 {
		return "", errors.New("无效的提醒 ID")
	}
	updates := map[string]any{"enabled": enabled}
	if enabled {
		// 重新启用时从 unknown 开始，下一次拿到行情只初始化状态，不立即误报。
		updates["last_state"] = "unknown"
	}
	if err := db.Dao.Model(&priceAlert{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return "", err
	}
	updateMonitorStatus(lenAlertsEnabled())
	return "设置成功", nil
}

func listAlertEvents(limit int) []alertEventDTO {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	var rows []priceAlertEvent
	if err := db.Dao.Order("triggered_at desc").Limit(limit).Find(&rows).Error; err != nil {
		return []alertEventDTO{}
	}
	out := make([]alertEventDTO, 0, len(rows))
	for _, e := range rows {
		out = append(out, alertEventDTOFromModel(e))
	}
	return out
}

func setAlertState(id uint, state string) error {
	return db.Dao.Model(&priceAlert{}).Where("id = ?", id).Update("last_state", state).Error
}

func markAlertTriggered(a priceAlert, state string, at time.Time) error {
	return db.Dao.Model(&priceAlert{}).Where("id = ?", a.ID).Updates(map[string]any{
		"last_state":        state,
		"last_triggered_at": at,
		"trigger_count":     a.TriggerCount + 1,
	}).Error
}

func addAlertEvent(a priceAlert, p float64, at time.Time) error {
	return db.Dao.Create(&priceAlertEvent{
		AlertID:     a.ID,
		StockCode:   a.StockCode,
		StockName:   a.StockName,
		Price:       p,
		Threshold:   a.Price,
		Direction:   a.Direction,
		TriggeredAt: at,
	}).Error
}

func triggerPriceAlert(a priceAlert, p float64, state string, at time.Time) {
	_ = markAlertTriggered(a, state, at)
	_ = addAlertEvent(a, p, at)

	emitMobile("mobileAlertTriggered", map[string]any{
		"alertId":   a.ID,
		"code":      a.StockCode,
		"name":      a.StockName,
		"price":     p,
		"threshold": a.Price,
		"direction": a.Direction,
		"time":      at.Format("2006-01-02 15:04:05"),
	})

	settings := loadMobileSettings()
	if !settings.PushEnabled {
		return
	}
	title, body := alertNotificationText(a, p, at)
	notifyDevice(title, body, a.StockCode)
}

func alertNotificationText(a priceAlert, p float64, at time.Time) (string, string) {
	verb := "涨到"
	if a.Direction == "below" {
		verb = "跌破"
	}
	title := fmt.Sprintf("【价格提醒】%s", a.StockName)
	body := fmt.Sprintf("现价 %.2f，已%s %.2f", p, verb, a.Price)
	if a.Note != "" {
		body += "\n" + a.Note
	}
	body += "\n" + at.Format("01-02 15:04")
	return title, body
}
