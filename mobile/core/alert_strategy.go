package mobilecore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/backend/logger"
	"aipanyi/backend/marketcal"
	"aipanyi/mobile/alerts"
	"gorm.io/gorm"
)

var (
	errAlertMode          = errors.New("请明确选择提醒方式")
	errAlertConditions    = errors.New("请填写量比、买卖盘方向及偏向阈值")
	errAlertCoverage      = errors.New("该股票暂缺可确认的量比、五档买卖盘或行情时间，可先保存停用")
	errAlertReview        = errors.New("请编辑规则并完成复核")
	errAlertStalePosition = errors.New("此建议依据旧持仓，请按最新账本重新评估并编辑规则")
	alertQuoteZone        = time.FixedZone("UTC+8", 8*3600)
)

func migrateAlertModes() error {
	return db.Dao.Transaction(func(tx *gorm.DB) error {
		unmigrated := "mode IS NULL OR mode = ''"
		if err := tx.Model(&priceAlert{}).Where("source = ?", "ai").Where("(" + unmigrated + ")").
			Updates(map[string]any{"mode": "strategy", "phase": "intraday", "needs_review": true, "enabled": false}).Error; err != nil {
			return err
		}
		return tx.Model(&priceAlert{}).Where(unmigrated).
			Updates(map[string]any{"mode": "price", "phase": "intraday"}).Error
	})
}

func configureAlertMode(tx *gorm.DB, a *priceAlert, in alertDTO) error {
	if a.Mode == "" && a.Source != "ai" {
		a.Mode = "price"
	}
	switch a.Mode {
	case "price":
		a.Phase, a.BookSide, a.MinVolumeRatio, a.MinBookBias = "intraday", "", 0, 0
	case "strategy":
		if a.Phase == "" {
			a.Phase = "intraday"
		}
		if a.Phase != "intraday" && a.Phase != "close" {
			return errAlertMode
		}
		if math.IsNaN(a.MinVolumeRatio) || math.IsInf(a.MinVolumeRatio, 0) || math.IsNaN(a.MinBookBias) || math.IsInf(a.MinBookBias, 0) {
			return errAlertConditions
		}
		if in.UsePosition {
			position, err := readTradePosition(tx, a.StockCode)
			if err != nil {
				return err
			}
			if position.Revision == 0 {
				return errTradeBaseline
			}
			if in.PositionRevision != nil && *in.PositionRevision != position.Revision {
				return errAlertStalePosition
			}
			a.PositionRevision = &position.Revision
		}
		a.NeedsReview = a.Phase == "close" || !a.strategyRule(time.Minute).Valid()
		if a.Phase == "close" {
			a.Enabled = false
		}
		if a.Enabled {
			return canEnableStrategy(tx, *a)
		}
	default:
		return errAlertMode
	}
	return nil
}

func canEnableStrategy(tx *gorm.DB, a priceAlert) error {
	if a.Mode != "strategy" {
		return nil
	}
	if a.Phase != "intraday" || a.NeedsReview {
		return errAlertReview
	}
	if !a.strategyRule(time.Minute).Valid() {
		return errAlertConditions
	}
	if a.PositionRevision != nil {
		position, err := readTradePosition(tx, a.StockCode)
		if err != nil {
			return err
		}
		if position.Revision != *a.PositionRevision {
			return errAlertReview
		}
	}
	var quote data.StockInfo
	err := tx.Where("code = ?", a.StockCode).Order("updated_at desc, id desc").First(&quote).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	q := strategySnapshot(a.StockCode, quote, time.Now())
	if q.At.IsZero() || q.VolumeRatio == nil || q.BookBias == nil || q.Price <= 0 {
		return errAlertCoverage
	}
	return nil
}

func (a priceAlert) strategyRule(maxAge time.Duration) alerts.StrategyRule {
	return alerts.StrategyRule{
		Direction: a.Direction, Price: a.Price, Phase: a.Phase, MinVolumeRatio: a.MinVolumeRatio,
		BookSide: a.BookSide, MinBookBias: a.MinBookBias, MaxAge: maxAge,
	}
}

func (a priceAlert) strategyState() alerts.StrategyState {
	var state alerts.StrategyState
	_ = json.Unmarshal([]byte(a.StrategyState), &state)
	return state
}

func supportedQuoteTime(code string, q data.StockInfo) time.Time {
	if !strings.HasPrefix(code, "sh") && !strings.HasPrefix(code, "sz") && !strings.HasPrefix(code, "hk") {
		return time.Time{}
	}
	at, _ := time.ParseInLocation("2006-01-02 15:04:05", quoteTime(q), alertQuoteZone)
	return at
}

func strategySnapshot(code string, q data.StockInfo, now time.Time) alerts.QuoteSnapshot {
	p := parsePrice(q.Price)
	if math.IsNaN(p) || math.IsInf(p, 0) {
		p = 0
	}
	snapshot := alerts.QuoteSnapshot{Market: marketOf(code), Price: p}
	// 腾讯沪深 A 股是现有行情源里同一响应提供量比、五档盘口与源时间的市场。
	if !strings.HasPrefix(code, "sh") && !strings.HasPrefix(code, "sz") {
		return snapshot
	}
	snapshot.At = supportedQuoteTime(code, q)
	if a := chinaQuoteSession(snapshot.At); a != "" && a == chinaQuoteSession(now) {
		snapshot.Session = a
	}
	if r := parsePrice(q.VolumeRatio); r > 0 && !math.IsInf(r, 0) {
		snapshot.VolumeRatio = &r
	}
	buy := bookVolume([5]string{q.B1P, q.B2P, q.B3P, q.B4P, q.B5P}, [5]string{q.B1V, q.B2V, q.B3V, q.B4V, q.B5V})
	sell := bookVolume([5]string{q.A1P, q.A2P, q.A3P, q.A4P, q.A5P}, [5]string{q.A1V, q.A2V, q.A3V, q.A4V, q.A5V})
	if buy > 0 && sell > 0 && !math.IsInf(buy+sell, 0) {
		bias := (buy - sell) / (buy + sell)
		snapshot.BookBias = &bias
	}
	return snapshot
}

func chinaQuoteSession(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	local := t.In(alertQuoteZone)
	hour, minute, second := local.Clock()
	elapsed := hour*3600 + minute*60 + second
	switch {
	case elapsed >= 9*3600+15*60 && elapsed <= 11*3600+30*60:
		return local.Format("2006-01-02") + "/AM"
	case elapsed >= 13*3600 && elapsed < 15*3600:
		return local.Format("2006-01-02") + "/PM"
	default:
		return ""
	}
}

func bookVolume(prices, sizes [5]string) float64 {
	var total float64
	for i, raw := range sizes {
		price, size := parsePrice(prices[i]), parsePrice(raw)
		if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) ||
			size <= 0 || math.IsNaN(size) || math.IsInf(size, 0) {
			return 0
		}
		total += size
	}
	return total
}

type alertEvidence struct {
	QuoteAt     time.Time `json:"quoteAt"`
	VolumeRatio *float64  `json:"volumeRatio"`
	BookBias    *float64  `json:"bookBias"`
}

func (e priceAlertEvent) evidence() *alertEvidence {
	if e.Evidence == "" {
		return nil
	}
	var result alertEvidence
	if json.Unmarshal([]byte(e.Evidence), &result) != nil {
		return nil
	}
	return &result
}

func persistStrategyDecision(a priceAlert, decision alerts.Decision, now time.Time, batch string) (*priceAlertEvent, error) {
	state, err := json.Marshal(decision.State)
	if err != nil {
		return nil, err
	}
	if a.StrategyState == string(state) {
		return nil, nil
	}
	updates := map[string]any{"strategy_state": string(state)}
	if !decision.Confirmed {
		return persistAlertEvaluation(a, updates, nil)
	}
	evidence, err := json.Marshal(alertEvidence{
		QuoteAt: decision.Evidence.At, VolumeRatio: decision.Evidence.VolumeRatio, BookBias: decision.Evidence.BookBias,
	})
	if err != nil {
		return nil, err
	}
	event := newAlertEvent(a, decision.Evidence.Price, now)
	event.Batch, event.Evidence = batch, string(evidence)
	return persistAlertEvaluation(a, updates, &event)
}

func newAlertEvent(a priceAlert, price float64, now time.Time) priceAlertEvent {
	return priceAlertEvent{AlertID: a.ID, StockCode: a.StockCode, StockName: a.StockName, Price: price,
		Threshold: a.Price, Direction: a.Direction, TriggeredAt: now.UTC(), Mode: a.Mode, RuleRevision: a.Revision}
}

func persistAlertEvaluation(a priceAlert, updates map[string]any, event *priceAlertEvent) (*priceAlertEvent, error) {
	portfolioWriteMu.Lock()
	defer portfolioWriteMu.Unlock()
	applied := false
	err := db.Dao.Transaction(func(tx *gorm.DB) error {
		if a.PositionRevision != nil {
			position, err := readTradePosition(tx, a.StockCode)
			if err != nil || position.Revision != *a.PositionRevision {
				return err
			}
		}
		if event != nil {
			updates["last_triggered_at"] = event.TriggeredAt
			updates["trigger_count"] = gorm.Expr("trigger_count + 1")
		}
		result := tx.Model(&priceAlert{}).Where("id = ? AND revision = ? AND enabled = ?", a.ID, a.Revision, true).
			Where("trigger_count = ? AND last_state = ?", a.TriggerCount, a.LastState).
			Where("COALESCE(strategy_state, '') = ?", a.StrategyState).Updates(updates)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		applied = true
		if event != nil {
			return tx.Create(event).Error
		}
		return nil
	})
	if err != nil || !applied {
		return nil, err
	}
	return event, nil
}

func publishAlertEvents(events []priceAlertEvent, title, body string) string {
	last := events[len(events)-1]
	for _, e := range events {
		emitMobile("mobileAlertTriggered", map[string]any{"id": e.ID, "alertId": e.AlertID, "code": e.StockCode})
	}
	if !loadMobileSettings().PushEnabled {
		return "push_disabled"
	}
	return notifyDevice(title, body, last.StockCode+"|"+strconv.FormatUint(uint64(last.ID), 10))
}

func (e *alertEngine) publishStrategyBatch(events []priceAlertEvent) string {
	last := events[len(events)-1]
	body := fmt.Sprintf("现价 %.2f，已确认 %d 条策略", last.Price, len(events))
	if q := last.evidence(); q != nil && q.VolumeRatio != nil && q.BookBias != nil {
		body += fmt.Sprintf("\n量比 %.2f · 买卖盘偏向 %+.1f%% · %s", *q.VolumeRatio, *q.BookBias*100, q.QuoteAt.In(alertQuoteZone).Format("15:04:05"))
	}
	return publishAlertEvents(events, "【策略提醒】"+last.StockName, body)
}

type alertContextDTO struct {
	Events []alertEventDTO `json:"events"`
	Status string          `json:"status"`
}

func getAlertContext(code string, id int64) (alertContextDTO, error) {
	out := alertContextDTO{Status: "waiting"}
	var event priceAlertEvent
	norm := normalizeCode(code)
	if err := db.Dao.Where("id = ? AND stock_code = ?", id, norm).First(&event).Error; err != nil {
		return out, errors.New("该股票的提醒记录不存在")
	}
	rows := []priceAlertEvent{event}
	if event.Batch != "" {
		if err := db.Dao.Where("batch = ? AND stock_code = ?", event.Batch, norm).Order("id").Find(&rows).Error; err != nil {
			return out, err
		}
	}
	rules, err := rulesForEvents(rows)
	if err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	quotes, _ := getQuotesContext(ctx, []string{norm})
	now := time.Now()
	q := strategySnapshot(norm, quotes[norm], now)
	maxAge := 2 * monitorPollInterval()
	for _, row := range rows {
		item := alertEventDTOFromModel(row)
		rule := rules[row.AlertID]
		item.CurrentStatus = currentAlertEventStatus(row, rule, now, maxAge)
		if item.CurrentStatus != "invalidated" && row.Mode == "strategy" && marketcal.IsMonitorTradingTime(ctx, rule.Market, now) {
			state := alerts.StrategyState{Latched: true}
			result := alerts.EvaluateStrategy(rule.strategyRule(maxAge), q, state, now)
			item.CurrentStatus = result.State.Status
		} else if row.Mode == "strategy" && item.CurrentStatus != "invalidated" {
			item.CurrentStatus = "waiting"
		}
		out.Events = append(out.Events, item)
	}
	if len(out.Events) == 1 {
		out.Status = out.Events[0].CurrentStatus
	} else {
		out.Status = "batch"
	}
	return out, nil
}

func rulesForEvents(rows []priceAlertEvent) (map[uint]priceAlert, error) {
	out := make(map[uint]priceAlert)
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.AlertID)
	}
	var rules []priceAlert
	if err := db.Dao.Where("id IN ?", ids).Find(&rules).Error; err != nil {
		return nil, err
	}
	for _, rule := range rules {
		out[rule.ID] = rule
	}
	return out, nil
}

func currentAlertEventStatus(row priceAlertEvent, rule priceAlert, now time.Time, maxAge time.Duration) string {
	if row.Mode != "strategy" {
		return "touched"
	}
	if !rule.Enabled || rule.Revision != row.RuleRevision ||
		(rule.LastTriggeredAt != nil && rule.LastTriggeredAt.After(row.TriggeredAt)) {
		return "invalidated"
	}
	state := rule.strategyState()
	if state.Status == "invalidated" {
		return "invalidated"
	}
	if state.SampleAt.IsZero() || state.SampleAt.After(now) || now.Sub(state.SampleAt) > maxAge ||
		chinaQuoteSession(state.SampleAt) != chinaQuoteSession(now) {
		return "waiting"
	}
	return state.Status
}

func logAlertWrite(err error) {
	logger.SugaredLogger.Warnf("mobile alert: persist decision failed: %v", err)
}
