package mobilecore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/mobile/alerts"
	"gorm.io/gorm"
)

const maxEnabledAlerts = 50

func alertDTOFromModel(a priceAlert) alertDTO {
	dto := alertDTO{
		ID:        int64(a.ID),
		Code:      a.StockCode,
		Name:      a.StockName,
		Market:    a.Market,
		Direction: a.Direction,
		Price:     a.Price,
		Note:      a.Note,
		Source:    a.Source,
		Enabled:   a.Enabled,
		Mode:      a.Mode, Phase: a.Phase, NeedsReview: a.NeedsReview,
		MinVolumeRatio: a.MinVolumeRatio, BookSide: a.BookSide, MinBookBias: a.MinBookBias,
		PositionRevision: a.PositionRevision, UsePosition: a.PositionRevision != nil,
		Status: a.strategyState().Status, Reason: a.strategyState().Reason,
		LastState:    a.LastState,
		TriggerCount: int64(a.TriggerCount),
		CreatedAt:    a.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if a.LastTriggeredAt != nil {
		dto.LastTriggeredAt = a.LastTriggeredAt.In(alertQuoteZone).Format("2006-01-02 15:04:05")
	}
	return dto
}

func alertEventDTOFromModel(e priceAlertEvent) alertEventDTO {
	return alertEventDTO{
		ID:        int64(e.ID),
		AlertID:   int64(e.AlertID),
		Code:      e.StockCode,
		Name:      e.StockName,
		Price:     e.Price,
		Threshold: e.Threshold,
		Direction: e.Direction,
		Mode:      e.Mode, Evidence: e.evidence(),
		TriggeredAt: e.TriggeredAt.In(alertQuoteZone).Format("2006-01-02 15:04:05"),
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
	portfolioWriteMu      sync.Mutex
	errAlertInvalid       = errors.New("股票代码、提醒方向或价格无效")
	errAlertNotFollowed   = errors.New("请先关注该股票")
	errAlertDuplicate     = errors.New("已存在相同的价格提醒")
	errAlertLimit         = fmt.Errorf("最多同时启用 %d 条价格提醒", maxEnabledAlerts)
	errAlertMissing       = errors.New("提醒规则不存在")
	errAlertBatchRejected = errors.New("本批提醒未替换，旧提醒已保留")
)

func refreshAlertMonitor() {
	updateMonitorStatus(lenAlertsEnabled())
	scheduleMonitorRefresh()
	wakeAlertEngine()
}

func saveAlert(in alertDTO) (int64, error) {
	alert, err := writeAlert(in)
	if err != nil {
		return 0, err
	}
	refreshAlertMonitor()
	return int64(alert.ID), nil
}

func saveAlertSuggestions(items []alertDTO) []alertSaveResult {
	results := writeAlerts(items, fetchAlertQuote)
	for _, result := range results {
		if result.Status == "saved" {
			refreshAlertMonitor()
			break
		}
	}
	return results
}

func writeAlerts(items []alertDTO, fetchQuote func(string) alerts.QuoteSnapshot) []alertSaveResult {
	results := make([]alertSaveResult, len(items))
	if len(items) == 0 {
		return results
	}
	code := normalizeCode(items[0].Code)
	for _, item := range items {
		if code == "" || normalizeCode(item.Code) != code {
			for i := range results {
				results[i] = alertSaveResult{Index: i, Status: "invalid", Message: "每次只能替换同一只股票的提醒"}
			}
			return results
		}
	}

	var quote alerts.QuoteSnapshot
	for _, item := range items {
		if wantsStrategyQuote(item) {
			quote = fetchQuote(code)
			break
		}
	}
	portfolioWriteMu.Lock()
	defer portfolioWriteMu.Unlock()
	err := db.Dao.Transaction(func(tx *gorm.DB) error {
		// 旧规则先在事务内腾出配额，同价位也能重新保存。触发历史独立保留。
		if err := tx.Where("stock_code = ?", code).Delete(&priceAlert{}).Error; err != nil {
			return err
		}
		rejected := false
		for index, item := range items {
			item.ID = 0
			alert, err := writeAlertTx(tx, item, quote)
			if err == errAlertCoverage {
				item.Enabled = false
				alert, err = writeAlertTx(tx, item, quote)
				if err == nil {
					results[index] = alertSaveResult{Index: index, ID: int64(alert.ID), Status: "saved", Message: "行情暂不完整，已停用保存"}
					continue
				}
			}
			results[index] = suggestionSaveResult(index, alert, err)
			rejected = rejected || (err != nil && err != errAlertDuplicate)
		}
		if rejected {
			return errAlertBatchRejected
		}
		return nil
	})
	if err != nil {
		for i, result := range results {
			switch result.Status {
			case "", "saved", "duplicate":
				results[i] = alertSaveResult{Index: i, Status: "rolled_back", Message: errAlertBatchRejected.Error()}
			}
		}
	}
	return results
}

func suggestionSaveResult(index int, alert priceAlert, err error) alertSaveResult {
	result := alertSaveResult{Index: index, ID: int64(alert.ID), Status: "saved"}
	if err == nil {
		return result
	}
	result.Message = err.Error()
	switch err {
	case errAlertDuplicate:
		result.Status = "duplicate"
		return result
	case errAlertInvalid, errAlertConditions, errAlertCoverage, errAlertMode, errAlertReview, errTradeBaseline, errAlertStalePosition:
		result.Status = "invalid"
	case errAlertNotFollowed:
		result.Status = "not_followed"
	case errAlertLimit:
		result.Status = "limit"
	default:
		result.Status = "failed"
		result.Message = "写入提醒失败，请重试"
	}
	result.ID = 0
	return result
}

func wantsStrategyQuote(in alertDTO) bool {
	return in.Enabled && in.Mode == "strategy" && in.Phase != "close"
}

func fetchAlertQuote(code string) alerts.QuoteSnapshot {
	return fetchStrategyQuote(context.Background(), code)
}

func writeAlert(in alertDTO) (a priceAlert, err error) {
	var quote alerts.QuoteSnapshot
	if wantsStrategyQuote(in) {
		quote = fetchAlertQuote(normalizeCode(in.Code))
	}
	portfolioWriteMu.Lock()
	defer portfolioWriteMu.Unlock()
	err = db.Dao.Transaction(func(tx *gorm.DB) error {
		a, err = writeAlertTx(tx, in, quote)
		return err
	})
	if err != nil && err != errAlertDuplicate {
		a.ID = 0
	}
	return
}

func writeAlertTx(tx *gorm.DB, in alertDTO, quote alerts.QuoteSnapshot) (a priceAlert, err error) {
	norm := normalizeCode(in.Code)
	direction := strings.ToLower(strings.TrimSpace(in.Direction))
	if norm == "" || (direction != "above" && direction != "below") ||
		in.Price <= 0 || math.IsNaN(in.Price) || math.IsInf(in.Price, 0) {
		return a, errAlertInvalid
	}
	var followed data.FollowedStock
	if err := tx.Where("stock_code = ?", norm).First(&followed).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return a, errAlertNotFollowed
		}
		return a, err
	}
	created := in.ID == 0
	var existing priceAlert
	if !created {
		if err := tx.First(&existing, in.ID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return a, errAlertMissing
			}
			return a, err
		}
	}
	a = priceAlert{
		StockCode: norm, StockName: followed.Name, Market: marketOf(norm),
		Direction: direction, Price: in.Price, Note: strings.TrimSpace(in.Note),
		Source: "manual", Enabled: in.Enabled, LastState: "unknown", Mode: in.Mode, Phase: in.Phase,
		MinVolumeRatio: in.MinVolumeRatio, BookSide: in.BookSide, MinBookBias: in.MinBookBias,
		InitialNotify: created && in.Enabled && (in.Mode == "price" || in.Mode == ""),
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
	if err := configureAlertMode(tx, &a, in, quote); err != nil {
		return a, err
	}
	if created || a.Enabled {
		dup, err := duplicateAlert(tx, a, existing.ID, !created)
		if err != nil {
			return a, err
		}
		if dup.ID != 0 {
			return dup, errAlertDuplicate
		}
	}
	if a.Enabled && !existing.Enabled {
		if err := checkAlertLimit(tx); err != nil {
			return a, err
		}
	}
	if created {
		err := tx.Create(&a).Error
		return a, err
	}
	a.ID = existing.ID
	return a, tx.Model(&existing).Updates(map[string]any{
		"stock_code": a.StockCode, "stock_name": a.StockName, "market": a.Market,
		"direction": a.Direction, "price": a.Price, "note": a.Note,
		"source": a.Source, "enabled": a.Enabled, "last_state": a.LastState,
		"mode": a.Mode, "phase": a.Phase, "min_volume_ratio": a.MinVolumeRatio,
		"book_side": a.BookSide, "min_book_bias": a.MinBookBias, "needs_review": a.NeedsReview,
		"strategy_state": "", "initial_notify": false, "revision": existing.Revision + 1,
		"position_revision": a.PositionRevision,
	}).Error
}

func duplicateAlert(tx *gorm.DB, a priceAlert, exclude uint, onlyEnabled bool) (priceAlert, error) {
	var dup priceAlert
	query := tx.Where("stock_code = ? AND direction = ? AND price = ? AND id <> ?", a.StockCode, a.Direction, a.Price, exclude)
	if a.Mode == "strategy" {
		query = query.Where("mode = ? AND phase = ? AND min_volume_ratio = ? AND book_side = ? AND min_book_bias = ?",
			a.Mode, a.Phase, a.MinVolumeRatio, a.BookSide, a.MinBookBias)
	} else {
		query = query.Where("(mode = ? OR mode = '' OR mode IS NULL)", "price")
	}
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
	var quote alerts.QuoteSnapshot
	var quoteCode string
	if enabled {
		// 在事务之外获取行情，避免网络请求占用 SQLite 写锁。
		var current priceAlert
		if err := db.Dao.First(&current, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errAlertMissing
			}
			return err
		}
		if current.Mode == "strategy" && current.Phase == "intraday" && !current.NeedsReview {
			quoteCode = current.StockCode
			quote = fetchAlertQuote(quoteCode)
		}
	}
	portfolioWriteMu.Lock()
	defer portfolioWriteMu.Unlock()
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
			if a.StockCode != quoteCode {
				quote = alerts.QuoteSnapshot{}
			}
			if err := canEnableStrategy(tx, a, quote); err != nil {
				return err
			}
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
		return tx.Model(&a).Updates(map[string]any{"enabled": enabled, "last_state": "unknown", "revision": a.Revision + 1, "strategy_state": "", "initial_notify": false}).Error
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
	rules, _ := rulesForEvents(rows)
	now, maxAge := time.Now(), 2*monitorPollInterval()
	out := make([]alertEventDTO, 0, len(rows))
	for _, e := range rows {
		item := alertEventDTOFromModel(e)
		if rules != nil {
			item.CurrentStatus = currentAlertEventStatus(e, rules[e.AlertID], now, maxAge)
		}
		out = append(out, item)
	}
	return out
}

func alertNotificationText(a priceAlert, p float64, at time.Time) (string, string) {
	verb := "涨到"
	if a.Direction == "below" {
		verb = "跌破"
	}
	title := fmt.Sprintf("【价格提醒】%s", a.StockName)
	body := fmt.Sprintf("现价 %.2f，已%s %.2f", p, verb, a.Price)
	if a.Note != "" {
		body += "\n备注：" + a.Note
	}
	body += "\n" + at.Format("01-02 15:04")
	return title, body
}
