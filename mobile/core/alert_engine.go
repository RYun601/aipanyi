package mobilecore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/backend/logger"
	"aipanyi/backend/marketcal"
	"aipanyi/mobile/alerts"
)

// alertEngine 价格提醒引擎。
//
// 依赖全部用函数字段注入，tick 不直接访问网络/数据库/时钟，
// 便于用假行情、假时钟做状态机单测。
type alertEngine struct {
	listEnabled  func() ([]priceAlert, error)
	quotes       func(ctx context.Context, codes []string) (map[string]data.StockInfo, error)
	isTrading    func(ctx context.Context, market string, at time.Time) bool
	now          func() time.Time
	interval     func() time.Duration
	enabledCount func() int

	instance         uint64
	round            uint64
	lastPoll         time.Time
	stop             chan struct{}
	stopOnce         sync.Once
	wake             chan struct{}
	seenStrategy     map[uint]int64
	lastPriceSamples map[string]time.Time
}

var alertEngineID atomic.Uint64

func newAlertEngine() *alertEngine {
	e := &alertEngine{
		listEnabled: func() ([]priceAlert, error) {
			var rows []priceAlert
			err := db.Dao.Where("enabled = ?", true).Find(&rows).Error
			return rows, err
		},
		quotes: getQuotesContext, isTrading: marketcal.IsMonitorTradingTime,
		now: time.Now, interval: monitorPollInterval, enabledCount: lenAlertsEnabled,
		stop: make(chan struct{}), wake: make(chan struct{}, 1), seenStrategy: make(map[uint]int64),
		instance: alertEngineID.Add(1), lastPriceSamples: make(map[string]time.Time),
	}
	return e
}

func monitorPollInterval() time.Duration {
	return time.Duration(max(loadMobileSettings().PollIntervalSec, 10)) * time.Second
}

func wakeAlertEngine() {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	if engine == nil {
		return
	}
	select {
	case engine.wake <- struct{}{}:
	default:
	}
}

func (e *alertEngine) run(ctx context.Context) {
	for ctx.Err() == nil {
		interval := max(e.interval(), 10*time.Second)
		startedAt := time.Now()
		e.safeTick(ctx, interval)
		timer := time.NewTimer(nextPollDelay(interval, time.Since(startedAt)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-e.stop:
			timer.Stop()
			return
		case <-e.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

func nextPollDelay(interval, elapsed time.Duration) time.Duration {
	if elapsed > interval {
		return interval
	}
	return interval - elapsed
}

func (e *alertEngine) shutdown() {
	e.stopOnce.Do(func() { close(e.stop) })
}

func (e *alertEngine) safeTick(ctx context.Context, interval time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("mobile alert tick panic: %v", r)
		}
	}()
	e.tick(ctx, interval)
}

// tick 执行一轮检查。返回本轮触发的规则数，便于测试与日志。
func (e *alertEngine) tick(parent context.Context, interval time.Duration) (triggered int) {
	ctx, cancel := context.WithTimeout(parent, interval)
	defer cancel()
	now := e.now()
	e.round++
	gap := time.Duration(0)
	if !e.lastPoll.IsZero() {
		gap = now.Sub(e.lastPoll)
	}
	e.lastPoll = now
	var active int
	var quoteElapsed time.Duration
	submissions := make(map[string]int)
	markets := make(map[string]int)
	result := "no_rules"
	defer func() {
		logger.SugaredLogger.Infof("mobile alert poll instance=%d round=%d gapMs=%d enabled=%d markets=%v quoteMs=%d result=%s triggered=%d submissions=%v",
			e.instance, e.round, gap.Milliseconds(), active, markets, quoteElapsed.Milliseconds(), result, triggered, submissions)
	}()
	rules, err := e.listEnabled()
	active = len(rules)
	if err != nil {
		result = "list_failed"
		return 0
	}
	if e.enabledCount != nil {
		updateMonitorStatus(e.enabledCount())
	}
	if len(rules) == 0 {
		return 0
	}

	// 只监控当前处于交易时段的市场，避免夜间无意义请求。
	filtered := make([]priceAlert, 0, len(rules))
	marketOpen := make(map[string]bool)
	for _, a := range rules {
		open, ok := marketOpen[a.Market]
		if !ok {
			open = e.isTrading(ctx, a.Market, now)
			marketOpen[a.Market] = open
		}
		if open {
			filtered = append(filtered, a)
			markets[a.Market]++
		}
	}
	if ctx.Err() != nil {
		result = "calendar_cancelled"
		return 0
	}
	if len(filtered) == 0 {
		result = "outside_session"
		return 0
	}

	seen := make(map[string]struct{}, len(filtered))
	codes := make([]string, 0, len(filtered))
	for _, a := range filtered {
		if _, ok := seen[a.StockCode]; ok {
			continue
		}
		seen[a.StockCode] = struct{}{}
		codes = append(codes, a.StockCode)
	}
	requestedAt := time.Now()
	quotes, err := e.quotes(ctx, codes)
	quoteElapsed = time.Since(requestedAt)
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		result = "quote_timeout"
		return 0
	case ctx.Err() != nil:
		result = "cancelled"
		return 0
	case err != nil:
		result = "quote_failed"
		return 0
	case len(quotes) == 0:
		result = "quote_empty"
		return 0
	}
	result = "ok"
	now = e.now()
	batch := fmt.Sprintf("%d-%d-%d", now.UnixNano(), e.instance, e.round)
	groups := make(map[string][]priceAlertEvent)
	snapshots := make(map[string]alerts.QuoteSnapshot, len(quotes))
	clear(marketOpen)
	for _, a := range filtered {
		open, ok := marketOpen[a.Market]
		if !ok {
			open = e.isTrading(ctx, a.Market, now)
			marketOpen[a.Market] = open
		}
		if ctx.Err() != nil || !open {
			continue
		}
		q := quotes[a.StockCode]
		if a.Mode == "strategy" {
			snapshot, ok := snapshots[a.StockCode]
			if !ok {
				snapshot = strategySnapshot(a.StockCode, q, now)
				snapshots[a.StockCode] = snapshot
			}
			state := a.strategyState()
			if e.seenStrategy[a.ID] != a.Revision {
				state.Consecutive = 0
				e.seenStrategy[a.ID] = a.Revision
			}
			decision := alerts.EvaluateStrategy(a.strategyRule(2*interval), snapshot, state, now)
			event, err := persistStrategyDecision(a, decision, now, batch)
			if err != nil {
				logAlertWrite(err)
				continue
			}
			if event != nil {
				groups[a.StockCode] = append(groups[a.StockCode], *event)
				triggered++
			}
			continue
		}
		price := parsePrice(q.Price)
		at := supportedQuoteTime(a.StockCode, q)
		if !(price > 0) || !usablePriceQuote(a.StockCode, at, e.lastPriceSamples[a.StockCode], now, 2*interval) {
			continue
		}
		if !at.IsZero() && e.lastPriceSamples != nil {
			e.lastPriceSamples[a.StockCode] = at
		}
		event, err := evaluatePriceAlert(ctx, a, price, now, 2*interval)
		if err != nil {
			logAlertWrite(err)
			continue
		}
		if event != nil {
			title, body := alertNotificationText(a, price, now)
			submissions[publishAlertEvents([]priceAlertEvent{*event}, title, body)]++
			triggered++
		}
	}
	for _, events := range groups {
		submissions[e.publishStrategyBatch(events)]++
	}
	return triggered
}

func usablePriceQuote(code string, at, previous, now time.Time, maxAge time.Duration) bool {
	// Legacy price rules can still operate with sources that have no reliable clock.
	if at.IsZero() {
		return true
	}
	if at.Before(previous) || at.After(now) || now.Sub(at) > maxAge {
		return false
	}
	if marketOf(code) == "A" {
		return chinaQuoteSession(at) != "" && chinaQuoteSession(at) == chinaQuoteSession(now)
	}
	return at.In(alertQuoteZone).Format("2006-01-02") == now.In(alertQuoteZone).Format("2006-01-02")
}

func evaluatePriceAlert(ctx context.Context, a priceAlert, price float64, now time.Time, maxAge time.Duration) (*priceAlertEvent, error) {
	next, trigger := alerts.Evaluate(a.Direction, a.Price, a.LastState, price)
	if a.InitialNotify && next == a.Direction && now.Sub(a.CreatedAt) <= maxAge &&
		marketcal.IsMonitorTradingTime(ctx, a.Market, a.CreatedAt) {
		trigger = true
	}
	if ctx.Err() != nil || (next == a.LastState && !trigger) {
		return nil, nil
	}
	updates := map[string]any{"last_state": next, "initial_notify": false}
	if !trigger {
		return persistAlertEvaluation(a, updates, nil)
	}
	event := newAlertEvent(a, price, now)
	return persistAlertEvaluation(a, updates, &event)
}
