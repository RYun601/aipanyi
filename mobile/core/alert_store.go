package mobilecore

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/backend/logger"
	"aipanyi/backend/marketcal"
	"gorm.io/gorm"
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

// backfillAlertMarkets 给早期版本缺失 market 字段的提醒规则补齐市场。
// AutoMigrate 只会加列，不会回填历史行；不补的话 HK/US 老提醒会按 A 股窗口调度。
func backfillAlertMarkets() error {
	if db.Dao == nil {
		return errors.New("数据库未初始化")
	}
	var rows []priceAlert
	if err := db.Dao.Where("market = '' OR market IS NULL").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		market := marketOf(row.StockCode)
		if market == "" {
			continue
		}
		if err := db.Dao.Model(&priceAlert{}).Where("id = ?", row.ID).Update("market", market).Error; err != nil {
			return err
		}
	}
	return nil
}

var (
	alertWriteMu        sync.Mutex
	errAlertInvalid     = errors.New("股票代码、提醒方向或价格无效")
	errAlertNotFollowed = errors.New("请先关注该股票")
	errAlertDuplicate   = errors.New("已存在相同的价格提醒")
	errAlertLimit       = fmt.Errorf("最多同时启用 %d 条价格提醒", maxEnabledAlerts)
	errAlertMissing     = errors.New("提醒规则不存在")
)

func refreshAlertMonitor() {
	updateMonitorStatus(lenAlertsEnabled())
	scheduleMonitorRefresh()
}

func saveAlert(in alertDTO) (int64, error) {
	alert, created, err := writeAlert(in)
	if err != nil {
		return 0, err
	}
	if created && alert.Enabled {
		go applyInitialAlertState(alert)
	}
	refreshAlertMonitor()
	return int64(alert.ID), nil
}

func saveAlertSuggestions(items []alertDTO) []alertSaveResult {
	results := writeAlerts(items)
	changed := false
	for i, result := range results {
		if result.Status != "saved" {
			continue
		}
		changed = true
		if items[i].Enabled {
			var a priceAlert
			if err := db.Dao.First(&a, result.ID).Error; err == nil && a.Enabled {
				go applyInitialAlertState(a)
			}
		}
	}
	if changed {
		refreshAlertMonitor()
	}
	return results
}

func writeAlerts(items []alertDTO) []alertSaveResult {
	results := make([]alertSaveResult, 0, len(items))
	for index, item := range items {
		item.ID = 0 // 批量添加不会修改已存在的规则。
		alert, _, err := writeAlert(item)
		result := alertSaveResult{Index: index, ID: int64(alert.ID), Status: "saved"}
		if err != nil {
			result.Message = err.Error()
			switch err {
			case errAlertInvalid:
				result.Status = "invalid"
			case errAlertNotFollowed:
				result.Status = "not_followed"
			case errAlertDuplicate:
				result.Status = "duplicate"
			case errAlertLimit:
				result.Status = "limit"
			default:
				result.Status = "failed"
				result.Message = "写入提醒失败，请重试"
			}
		}
		results = append(results, result)
	}
	return results
}

func writeAlert(in alertDTO) (a priceAlert, created bool, err error) {
	norm := normalizeCode(in.Code)
	direction := strings.ToLower(strings.TrimSpace(in.Direction))
	if norm == "" || (direction != "above" && direction != "below") ||
		in.Price <= 0 || math.IsNaN(in.Price) || math.IsInf(in.Price, 0) {
		return a, false, errAlertInvalid
	}
	alertWriteMu.Lock()
	defer alertWriteMu.Unlock()
	err = db.Dao.Transaction(func(tx *gorm.DB) error {
		var followed data.FollowedStock
		if err := tx.Where("stock_code = ?", norm).First(&followed).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errAlertNotFollowed
			}
			return err
		}
		created = in.ID == 0
		var existing priceAlert
		if !created {
			if err := tx.First(&existing, in.ID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errAlertMissing
				}
				return err
			}
		}
		a = priceAlert{
			StockCode: norm, StockName: followed.Name, Market: marketOf(norm),
			Direction: direction, Price: in.Price, Note: strings.TrimSpace(in.Note),
			Source: "manual", Enabled: in.Enabled, LastState: "unknown",
		}
		if a.StockName == "" {
			a.StockName = norm
		}
		if strings.TrimSpace(in.Source) == "ai" {
			a.Source = "ai"
		}
		if note := []rune(a.Note); len(note) > 100 {
			a.Note = string(note[:100])
		}
		if created || a.Enabled {
			dup, err := duplicateAlert(tx, a, existing.ID, !created)
			if err != nil {
				return err
			}
			if dup.ID != 0 {
				a = dup
				return errAlertDuplicate
			}
		}
		if a.Enabled && !existing.Enabled {
			if err := checkAlertLimit(tx); err != nil {
				return err
			}
		}
		if created {
			return tx.Create(&a).Error
		}
		a.ID = existing.ID
		return tx.Model(&existing).Updates(map[string]any{
			"stock_code": a.StockCode, "stock_name": a.StockName, "market": a.Market,
			"direction": a.Direction, "price": a.Price, "note": a.Note,
			"source": a.Source, "enabled": a.Enabled, "last_state": a.LastState,
		}).Error
	})
	if err != nil && err != errAlertDuplicate {
		a.ID = 0
	}
	return
}

func duplicateAlert(tx *gorm.DB, a priceAlert, exclude uint, onlyEnabled bool) (priceAlert, error) {
	var dup priceAlert
	query := tx.Where("stock_code = ? AND direction = ? AND price = ? AND id <> ?", a.StockCode, a.Direction, a.Price, exclude)
	if onlyEnabled {
		query = query.Where("enabled = ?", true)
	}
	err := query.Order("enabled desc, id asc").First(&dup).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return dup, err
}

func checkAlertLimit(tx *gorm.DB) error {
	var count int64
	if err := tx.Model(&priceAlert{}).Where("enabled = ?", true).Count(&count).Error; err != nil {
		return err
	}
	if count >= maxEnabledAlerts {
		return errAlertLimit
	}
	return nil
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
	scheduleMonitorRefresh()
	return "删除成功", nil
}

func setAlertEnabled(id int64, enabled bool) (string, error) {
	if err := writeAlertEnabled(id, enabled); err != nil {
		return "", err
	}
	refreshAlertMonitor()
	return "设置成功", nil
}

func writeAlertEnabled(id int64, enabled bool) error {
	if id <= 0 {
		return errAlertMissing
	}
	alertWriteMu.Lock()
	defer alertWriteMu.Unlock()
	return db.Dao.Transaction(func(tx *gorm.DB) error {
		var a priceAlert
		if err := tx.First(&a, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errAlertMissing
			}
			return err
		}
		if a.Enabled == enabled {
			return nil
		}
		if enabled {
			dup, err := duplicateAlert(tx, a, a.ID, true)
			if err != nil {
				return err
			}
			if dup.ID != 0 {
				return errAlertDuplicate
			}
			if err := checkAlertLimit(tx); err != nil {
				return err
			}
		}
		return tx.Model(&a).Updates(map[string]any{"enabled": enabled, "last_state": "unknown"}).Error
	})
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
