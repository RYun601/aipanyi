package mobilecore

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"time"

	"aipanyi/backend/db"
	"aipanyi/backend/logger"
	"aipanyi/backend/marketcal"
	"aipanyi/mobile/alerts"
	"gorm.io/gorm"
)

type tradeSummary struct {
	Quantity     int64  `json:"quantity"`
	AveragePrice string `json:"averagePrice"`
	NetAmount    string `json:"netAmount"`
}

const tradeSummaryVersion = 1

type tradeStrategyDTO struct {
	SummaryVersion  int            `json:"summaryVersion"`
	Position        tradeResult    `json:"position"`
	Status          string         `json:"status"`
	TradingDate     string         `json:"tradingDate"`
	QuoteAt         *time.Time     `json:"quoteAt,omitempty"`
	EvaluatedAt     *time.Time     `json:"evaluatedAt,omitempty"`
	Bought          tradeSummary   `json:"bought"`
	Sold            tradeSummary   `json:"sold"`
	TodayRealized   string         `json:"todayRealized"`
	MatchedShares   int64          `json:"matchedShares"`
	UnmatchedShares int64          `json:"unmatchedShares"`
	MatchedSavings  string         `json:"matchedSavings"`
	TradeIDs        []int64        `json:"tradeIds"`
	RuleIDs         []uint         `json:"ruleIds,omitempty"`
	RuleVersions    map[uint]int64 `json:"ruleVersions,omitempty"`
}

type tradeTotals struct {
	shares int64
	gross  int64
	net    int64
}

func (t *tradeTotals) add(shares, gross, net int64) bool {
	t.shares += shares
	t.gross += gross
	t.net += net
	return t.shares <= maxTradeShares && checkedTradeMoney(t.gross) && checkedTradeMoney(t.net)
}

func (t tradeTotals) dto() tradeSummary {
	return tradeSummary{
		Quantity: t.shares, AveragePrice: formatTradeMoney(proportionalTradeMoney(t.gross, 1, t.shares)),
		NetAmount: formatTradeMoney(t.net),
	}
}

func tradeDay(code string, at time.Time) string {
	zone := alertQuoteZone
	if marketOf(code) == "US" {
		if ny, err := time.LoadLocation("America/New_York"); err == nil {
			zone = ny
		}
	}
	return at.In(zone).Format("2006-01-02")
}

type tradeSegment struct {
	baseline tradeEntry
	fills    []tradeEntry
}

func tradeSegments(tx *gorm.DB, code string, currentBaselineID int64) ([]tradeSegment, error) {
	var rows []tradeEntry
	if err := tx.Select("id", "code", "baseline_id", "kind", "target_id", "at", "side", "quantity", "price", "fee").
		Where("code = ?", code).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	var segments []tradeSegment
	fills := make(map[int64][]tradeEntry)
	found := false
	for _, row := range rows {
		if row.Kind == "baseline" {
			segments = append(segments, tradeSegment{baseline: row})
			found = found || row.ID == currentBaselineID
		} else {
			fills[row.BaselineID] = append(fills[row.BaselineID], row)
		}
	}
	if !found {
		return nil, errors.New("找不到当前持仓的期初记录")
	}
	for i := range segments {
		segments[i].fills = activeTrades(fills[segments[i].baseline.ID])
	}
	return segments, nil
}

func summarizeTrades(position tradePosition, segments []tradeSegment, now time.Time) (tradeStrategyDTO, error) {
	day := tradeDay(position.Code, now)
	out := tradeStrategyDTO{SummaryVersion: tradeSummaryVersion, Position: position.result(position.BaselineID),
		TradingDate: day, Status: "pending", TradeIDs: []int64{}}
	type soldLot struct{ shares, proceeds int64 }
	var openSales []soldLot
	var cycleIDs, todayIDs []int64
	var bought, sold tradeTotals
	var todayRealized, savings int64
	for _, segment := range segments {
		current := segment.baseline.ID == position.BaselineID
		state, err := replayTradePosition(segment.baseline, nil)
		if err != nil {
			return out, err
		}
		for _, fill := range segment.fills {
			wasFlat := state.Quantity == 0
			realized, err := state.applyFill(fill)
			if err != nil {
				return out, err
			}
			if current && wasFlat && fill.Side == "buy" {
				cycleIDs, openSales = nil, nil
				out.MatchedShares, savings = 0, 0
			}
			if current {
				cycleIDs = append(cycleIDs, fill.ID)
			}
			today := tradeDay(position.Code, fill.At) == day
			if today {
				todayIDs = append(todayIDs, fill.ID)
			}
			amount, _ := tradeProduct(fill.Price, fill.Quantity)
			if fill.Side == "sell" {
				net := amount - fill.Fee
				if today {
					todayRealized += realized
					if !sold.add(fill.Quantity, amount, net) || !checkedTradeMoney(todayRealized) {
						return out, errTradeOverflow
					}
				}
				if !current {
					continue
				}
				if state.Quantity == 0 {
					openSales = nil // 清仓后再次买入属于新一轮持仓。
				} else {
					openSales = append(openSales, soldLot{fill.Quantity, net})
				}
				continue
			}
			cost := amount + fill.Fee
			if today && !bought.add(fill.Quantity, amount, cost) {
				return out, errTradeOverflow
			}
			if !current {
				continue
			}
			for shares := fill.Quantity; shares > 0 && len(openSales) > 0; {
				lot := &openSales[0]
				matched := min(lot.shares, shares)
				allocatedProceeds := proportionalTradeMoney(lot.proceeds, matched, lot.shares)
				allocatedCost := proportionalTradeMoney(cost, matched, shares)
				difference := allocatedProceeds - allocatedCost
				if !checkedTradeMoney(difference) {
					return out, errTradeOverflow
				}
				savings += difference
				if !checkedTradeMoney(savings) {
					return out, errTradeOverflow
				}
				if out.MatchedShares > maxTradeShares-matched {
					return out, errTradeOverflow
				}
				out.MatchedShares += matched
				shares, cost = shares-matched, cost-allocatedCost
				lot.shares, lot.proceeds = lot.shares-matched, lot.proceeds-allocatedProceeds
				if lot.shares == 0 {
					openSales = openSales[1:]
				}
			}
		}
	}
	for _, lot := range openSales {
		out.UnmatchedShares += lot.shares
	}
	seen := make(map[int64]bool, len(cycleIDs)+len(todayIDs))
	for _, ids := range [][]int64{todayIDs, cycleIDs} {
		for _, id := range ids {
			if !seen[id] {
				out.TradeIDs = append(out.TradeIDs, id)
				seen[id] = true
			}
		}
	}
	out.Bought, out.Sold = bought.dto(), sold.dto()
	out.MatchedSavings, out.TodayRealized = formatTradeMoney(savings), formatTradeMoney(todayRealized)
	if position.Quantity == 0 {
		out.Status = "closed"
	}
	return out, nil
}

func tradeStrategyInputs(code string, now time.Time) (tradePosition, tradeStrategyDTO, []priceAlert, error) {
	var pos tradePosition
	var summary tradeStrategyDTO
	var rules []priceAlert
	err := db.Dao.Transaction(func(tx *gorm.DB) error {
		var err error
		pos, err = readTradePosition(tx, code)
		if err != nil {
			return err
		}
		if pos.Revision == 0 {
			summary.Status = "no_baseline"
			return nil
		}
		segments, err := tradeSegments(tx, code, pos.BaselineID)
		if err != nil {
			return err
		}
		summary, err = summarizeTrades(pos, segments, now)
		if err != nil {
			return err
		}
		rules, err = activeTradeRules(tx, code, pos.Revision)
		return err
	})
	return pos, summary, rules, err
}

func activeTradeRules(tx *gorm.DB, code string, revision int64) ([]priceAlert, error) {
	var rules []priceAlert
	err := tx.Where("stock_code = ? AND mode = ? AND enabled = ? AND needs_review = ?", code, "strategy", true, false).
		Where("position_revision IS NULL OR position_revision = ?", revision).Order("id").Find(&rules).Error
	return rules, err
}

func tradeRuleVersions(rules []priceAlert) map[uint]int64 {
	versions := make(map[uint]int64, len(rules))
	for _, rule := range rules {
		versions[rule.ID] = rule.Revision
	}
	return versions
}

func confirmedTradeRule(rules []priceAlert) bool {
	for _, rule := range rules {
		if rule.strategyState().Status == "confirmed" {
			return true
		}
	}
	return false
}

func getTradeStrategy(code string, now time.Time) (tradeStrategyDTO, bool, error) {
	position, err := readTradePosition(db.Dao, code)
	if err != nil {
		return tradeStrategyDTO{}, false, err
	}
	if position.Revision == 0 {
		return tradeStrategyDTO{Status: "no_baseline"}, false, nil
	}
	var cached tradeStrategyDTO
	if !position.Pending && json.Unmarshal([]byte(position.StrategyJSON), &cached) == nil &&
		cached.SummaryVersion == tradeSummaryVersion && cached.Position.Revision == position.Revision && cached.TradingDate == tradeDay(code, now) {
		maxAge := 2 * monitorPollInterval()
		needsRefresh := cached.EvaluatedAt == nil || now.Sub(*cached.EvaluatedAt) > maxAge || cached.EvaluatedAt.After(now)
		if !needsRefresh {
			rules, err := activeTradeRules(db.Dao, code, position.Revision)
			if err != nil {
				return tradeStrategyDTO{}, false, err
			}
			needsRefresh = !maps.Equal(cached.RuleVersions, tradeRuleVersions(rules))
			if cached.Status == "conditions_met" {
				session := chinaQuoteSession(now)
				needsRefresh = needsRefresh || cached.QuoteAt == nil || cached.QuoteAt.After(now) ||
					now.Sub(*cached.QuoteAt) > maxAge || session == "" || session != chinaQuoteSession(*cached.QuoteAt) ||
					!confirmedTradeRule(rules)
			}
			if !needsRefresh {
				return cached, false, nil
			}
		}
	}
	_, current, _, err := tradeStrategyInputs(code, now)
	return current, true, err
}

type tradeQuoteSource func(context.Context, string, time.Time) alerts.QuoteSnapshot

func fetchTradeQuote(ctx context.Context, code string, _ time.Time) alerts.QuoteSnapshot {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	quotes, err := getQuotesContext(ctx, []string{code})
	if err != nil || ctx.Err() != nil {
		return alerts.QuoteSnapshot{}
	}
	return strategySnapshot(code, quotes[code], time.Now())
}

func refreshTradeStrategy(ctx context.Context, code string, now time.Time, fetch tradeQuoteSource) (tradeStrategyDTO, error) {
	pos, out, rules, err := tradeStrategyInputs(code, now)
	if err != nil || pos.Revision == 0 {
		return out, err
	}
	if out.Status != "closed" {
		requestStarted := time.Now()
		quote := fetch(ctx, code, now)
		if !quote.At.IsZero() {
			out.QuoteAt = &quote.At
		}
		out.Status = tradeStrategyStatus(ctx, &out, rules, quote, now.Add(time.Since(requestStarted)))
	}
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	out.EvaluatedAt = &now
	out.RuleVersions = tradeRuleVersions(rules)
	payload, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	portfolioWriteMu.Lock()
	currentRules, ruleErr := activeTradeRules(db.Dao, code, pos.Revision)
	if ruleErr != nil || !maps.Equal(out.RuleVersions, tradeRuleVersions(currentRules)) ||
		(out.Status == "conditions_met" && !confirmedTradeRule(currentRules)) {
		portfolioWriteMu.Unlock()
		if ruleErr != nil {
			return out, ruleErr
		}
		latest, _, err := getTradeStrategy(code, now)
		return latest, err
	}
	updated := db.Dao.Model(&tradePosition{}).
		Where("code = ? AND revision = ? AND COALESCE(evaluated_at, 0) < ?", code, pos.Revision, now.UnixNano()).
		Updates(map[string]any{"pending": false, "strategy_json": string(payload), "evaluated_at": now.UnixNano()})
	if updated.Error == nil && updated.RowsAffected == 1 {
		emitMobile("mobileTradeStrategy", map[string]any{"code": code, "revision": pos.Revision})
	}
	portfolioWriteMu.Unlock()
	if updated.Error != nil {
		return out, updated.Error
	}
	if updated.RowsAffected != 1 {
		latest, _, err := getTradeStrategy(code, now)
		return latest, err
	}
	return out, nil
}

func tradeStrategyStatus(ctx context.Context, out *tradeStrategyDTO, rules []priceAlert, quote alerts.QuoteSnapshot, now time.Time) string {
	maxAge := 2 * monitorPollInterval()
	if quote.Price <= 0 || quote.At.IsZero() || quote.At.After(now) || now.Sub(quote.At) > maxAge {
		return "quote_pending"
	}
	if len(rules) == 0 {
		return "rules_pending"
	}
	if !marketcal.IsMonitorTradingTime(ctx, quote.Market, now) || quote.Session == "" {
		return "outside_session"
	}
	status := "observing"
	if out.UnmatchedShares > 0 {
		status = "waiting_rebuy"
	}
	for _, rule := range rules {
		out.RuleIDs = append(out.RuleIDs, rule.ID)
		// Recheck the actual current quote even if the rule has seen its timestamp already.
		state := rule.strategyState()
		if !quote.At.After(state.SampleAt) {
			state.SampleAt = time.Time{}
			state.Consecutive = 0
		}
		result := alerts.EvaluateStrategy(rule.strategyRule(maxAge), quote, state, now)
		if result.State.Status == "confirmed" && rule.strategyState().Status == "confirmed" {
			status = "conditions_met"
		}
	}
	return status
}

func (a *mobileApp) enqueueTradeStrategy(code string) {
	a.tradeMu.Lock()
	if _, running := a.tradeJobs[code]; running {
		a.tradeJobs[code] = true
		a.tradeMu.Unlock()
		return
	}
	if a.tradeJobs == nil {
		a.tradeJobs = make(map[string]bool)
	}
	a.tradeJobs[code] = false
	a.tradeMu.Unlock()
	go func() {
		for {
			if a.ctx.Err() == nil {
				_, err := refreshTradeStrategy(a.ctx, code, time.Now(), fetchTradeQuote)
				if err != nil && !errors.Is(err, context.Canceled) {
					logger.SugaredLogger.Warn("mobile trade: local strategy refresh failed")
				}
			}
			a.tradeMu.Lock()
			again := a.tradeJobs[code] && a.ctx.Err() == nil
			if again {
				a.tradeJobs[code] = false
			} else {
				delete(a.tradeJobs, code)
			}
			a.tradeMu.Unlock()
			if !again {
				return
			}
		}
	}()
}

func (a *mobileApp) resumeTradeStrategies() {
	var codes []string
	if err := db.Dao.Model(&tradePosition{}).Where("pending = ?", true).
		Where("EXISTS (SELECT 1 FROM followed_stock WHERE followed_stock.stock_code = mobile_trade_position.code AND is_del = 0)").
		Pluck("code", &codes).Error; err != nil {
		return
	}
	for _, code := range codes {
		a.enqueueTradeStrategy(code)
	}
}
