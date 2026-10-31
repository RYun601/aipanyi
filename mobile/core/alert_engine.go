package mobilecore

import (
	"context"
	"sync"
	"time"

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
	setState     func(id uint, state string) error
	trigger      func(a priceAlert, price float64, state string, at time.Time)
	quotes       func(codes []string) (map[string]float64, error)
	isTrading    func(market string, at time.Time) bool
	now          func() time.Time
	interval     func() time.Duration
	enabledCount func() int

	stop     chan struct{}
	stopOnce sync.Once
}

func newAlertEngine() *alertEngine {
	e := &alertEngine{
		listEnabled: func() ([]priceAlert, error) {
			var rows []priceAlert
			err := db.Dao.Where("enabled = ?", true).Find(&rows).Error
			return rows, err
		},
		setState:  setAlertState,
		trigger:   triggerPriceAlert,
		quotes:    quotePrices,
		isTrading: marketcal.IsTradingTimeForMarket,
		now:       time.Now,
		interval: func() time.Duration {
			sec := loadMobileSettings().PollIntervalSec
			if sec < 10 {
				sec = 10
			}
			return time.Duration(sec) * time.Second
		},
		enabledCount: lenAlertsEnabled,
		stop:         make(chan struct{}),
	}
	return e
}

func quotePrices(codes []string) (map[string]float64, error) {
	quotes, err := getQuotes(codes)
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(quotes))
	for code, q := range quotes {
		out[code] = parsePrice(q.Price)
	}
	return out, nil
}

func (e *alertEngine) run(ctx context.Context) {
	// 启动先跑一次：恢复上次运行期间可能已经穿越的规则。
	e.safeTick()
	for {
		interval := e.interval()
		if interval < 10*time.Second {
			interval = 10 * time.Second
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-e.stop:
			timer.Stop()
			return
		case <-timer.C:
			e.safeTick()
		}
	}
}

func (e *alertEngine) shutdown() {
	e.stopOnce.Do(func() { close(e.stop) })
}

func (e *alertEngine) safeTick() {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("mobile alert tick panic: %v", r)
		}
	}()
	e.tick(e.now())
}

// tick 执行一轮检查。返回本轮触发的规则数，便于测试与日志。
func (e *alertEngine) tick(now time.Time) int {
	alerts, err := e.listEnabled()
	if err != nil {
		logger.SugaredLogger.Errorf("mobile alert: list enabled failed: %v", err)
		return 0
	}
	if e.enabledCount != nil {
		updateMonitorStatus(e.enabledCount())
	}
	if len(alerts) == 0 {
		return 0
	}

	// 只监控当前处于交易时段的市场，避免夜间无意义请求。
	filtered := make([]priceAlert, 0, len(alerts))
	for _, a := range alerts {
		if e.isTrading(a.Market, now) {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) == 0 {
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
	prices, err := e.quotes(codes)
	if err != nil {
		logger.SugaredLogger.Errorf("mobile alert: quotes failed: %v", err)
		return 0
	}

	triggered := 0
	for _, a := range filtered {
		p, ok := prices[a.StockCode]
		if !ok || p <= 0 {
			// 行情缺失/停牌：不动状态，避免误报。
			continue
		}
		if e.evaluate(a, p, now) {
			triggered++
		}
	}
	return triggered
}

// evaluate 单条规则状态机。返回是否触发通知。
func (e *alertEngine) evaluate(a priceAlert, price float64, now time.Time) bool {
	cur := alerts.NormalizeState(a.LastState)
	next, trigger := alerts.Evaluate(a.Direction, a.Price, a.LastState, price)
	if trigger {
		if e.trigger != nil {
			e.trigger(a, price, next, now)
		} else {
			_ = e.setState(a.ID, next)
		}
		return true
	}
	if next != cur {
		_ = e.setState(a.ID, next)
	}
	return false
}
