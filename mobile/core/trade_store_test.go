package mobilecore

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
)

func setupTradeTestDB(t *testing.T) {
	t.Helper()
	db.InitWithOptions(filepath.Join(t.TempDir(), "trades.db"), db.MobileOptions())
	conn, err := db.Dao.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Dao.AutoMigrate(&data.FollowedStock{}, &priceAlert{}, &priceAlertEvent{}, &tradeEntry{}, &tradePosition{}, &tradeDraft{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Create(&data.FollowedStock{StockCode: "sh600000", Name: "测试", Volume: 100, CostPrice: 10}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestTradeFeeRateSetting(t *testing.T) {
	setupTradeTestDB(t)
	if err := db.Dao.AutoMigrate(&mobileSettings{}); err != nil {
		t.Fatal(err)
	}
	stale := mobileSettings{ID: settingsRowID, DefaultAIConfigID: 7, PollIntervalSec: 30}
	if err := db.Dao.Create(&stale).Error; err != nil {
		t.Fatal(err)
	}
	app := newMobileApp(t.Context())
	if got := app.GetMobileSettings().TradeFeeRate; got != "" {
		t.Fatalf("old settings have fee rate %q, want blank", got)
	}
	if got, err := app.SetTradeFeeRate(" 0.0300 "); err != nil || got != "0.03" {
		t.Fatalf("save rate = %q, %v", got, err)
	}
	if err := saveMobileSettings(stale); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"-1", "100.0001", "0.00001", "1%"} {
		if _, err := app.SetTradeFeeRate(invalid); err == nil {
			t.Fatalf("accepted invalid fee rate %q", invalid)
		}
	}
	if s := app.GetMobileSettings(); s.TradeFeeRate != "0.03" || s.DefaultAIConfigID != 7 {
		t.Fatalf("settings after unrelated save = %+v", s)
	}
	if got, err := app.SetTradeFeeRate("0"); err != nil || got != "0" {
		t.Fatalf("zero rate = %q, %v", got, err)
	}
	if _, err := app.SetTradeFeeRate(""); err != nil || app.GetMobileSettings().TradeFeeRate != "" {
		t.Fatalf("clear rate error = %v", err)
	}
}

func TestTradeFeeRulesKeepBuyAndSellSeparate(t *testing.T) {
	setupTradeTestDB(t)
	if err := db.Dao.AutoMigrate(&mobileSettings{}); err != nil {
		t.Fatal(err)
	}
	stale := mobileSettings{ID: settingsRowID, PollIntervalSec: 30, TradeFeeRate: "0.03"}
	if err := db.Dao.Create(&stale).Error; err != nil {
		t.Fatal(err)
	}
	app := newMobileApp(t.Context())
	if got := app.GetMobileSettings().TradeFeeRules; got.Buy.Rate != "0.03" || got.Sell.Rate != "0.03" {
		t.Fatalf("legacy rates = %+v", got)
	}
	saved, err := app.SetTradeFeeRules(tradeFeeRules{
		Buy:  tradeFeeRule{Mode: "fixed", Rate: "0.01"},
		Sell: tradeFeeRule{Mode: "aShare", Commission: " 0.850 ", Minimum: "0.50"},
	})
	if err != nil || saved.Buy.Rate != "0.01" || saved.Sell.Commission != "0.85" || saved.Sell.Minimum != "0.5" {
		t.Fatalf("saved rules = %+v, %v", saved, err)
	}
	invalid := saved
	invalid.Sell.Minimum = "0.505"
	if _, err := app.SetTradeFeeRules(invalid); err == nil {
		t.Fatal("accepted a minimum commission with a fractional cent")
	}
	if err := saveMobileSettings(stale); err != nil {
		t.Fatal(err)
	}
	if got := app.GetMobileSettings().TradeFeeRules; got != saved {
		t.Fatalf("persisted rules = %+v, want %+v", got, saved)
	}
}

func TestTradeDraftSurvivesCommitBeforeAcknowledgment(t *testing.T) {
	setupTradeTestDB(t)
	at := baselineTrade(t)
	fill := tradeAt(at.Add(time.Minute), "interrupted", "sell", 40, "12")
	payload, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	draft := tradeDraftDTO{Kind: "fill", Input: payload}
	if err := saveTradeDraft(draft); err != nil {
		t.Fatal(err)
	}
	first, err := recordTrade(fill)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := getTradeDraft(fill.Code)
	if err != nil || restored == nil || restored.Kind != "fill" {
		t.Fatalf("restore = %+v, %v", restored, err)
	}
	var retry tradeInput
	if err := json.Unmarshal(restored.Input, &retry); err != nil {
		t.Fatal(err)
	}
	receipt, err := recordTrade(retry)
	if err != nil || receipt != first || receipt.Quantity != 60 {
		t.Fatalf("receipt = %+v, %v", receipt, err)
	}
	if err := acknowledgeTradeDraft(fill.Code, fill.RequestID); err != nil {
		t.Fatal(err)
	}
	if got, err := getTradeDraft(fill.Code); got != nil || err != nil {
		t.Fatalf("acknowledgement left %+v, %v", got, err)
	}
}

func baselineTrade(t *testing.T) time.Time {
	t.Helper()
	at := time.Date(2026, 10, 30, 1, 30, 0, 0, time.UTC)
	result, err := setTradeBaseline(tradeBaselineInput{Code: "sh600000", RequestID: "baseline", At: at, Quantity: 100, UnitCost: "10"})
	if err != nil || result.Quantity != 100 || result.AverageCost != "10.0000" {
		t.Fatalf("baseline: %+v %v", result, err)
	}
	return at
}

func tradeAt(at time.Time, id, side string, shares int64, price string) tradeInput {
	return tradeInput{Code: "sh600000", RequestID: id, Side: side, Quantity: shares, Price: price, Fee: "4", At: at}
}

func TestTradeLedgerReplay(t *testing.T) {
	setupTradeTestDB(t)
	at := baselineTrade(t)
	sell := tradeAt(at.Add(time.Minute), "sell", "sell", 40, "12")
	first, err := recordTrade(sell)
	if err != nil || first.Quantity != 60 || first.AverageCost != "10.0000" || first.NetCost != "8.7333" {
		t.Fatalf("sell: %+v %v", first, err)
	}
	buy := tradeAt(at.Add(3*time.Minute), "buy", "buy", 40, "9")
	second, err := recordTrade(buy)
	if err != nil || second.Quantity != 100 || second.AverageCost != "9.6400" || second.NetCost != "8.8800" {
		t.Fatalf("buy: %+v %v", second, err)
	}
	replacement := tradeAt(at.Add(2*time.Minute), "", "buy", 20, "8")
	corrected, err := correctTrade(tradeCorrection{Code: buy.Code, RequestID: "correct", TargetID: second.ID, Replacement: &replacement})
	if err != nil || corrected.Quantity != 80 || corrected.AverageCost != "9.5500" || corrected.NetCost != "8.6000" {
		t.Fatalf("correction: %+v %v", corrected, err)
	}
	closed, err := recordTrade(tradeAt(at.Add(4*time.Minute), "close", "sell", 80, "9"))
	if err != nil || closed.Quantity != 0 || closed.AverageCost != "0.0000" || closed.NetCost != "0.0000" {
		t.Fatalf("close: %+v %v", closed, err)
	}
	reopen, err := recordTrade(tradeAt(at.Add(5*time.Minute), "reopen", "buy", 10, "8"))
	if err != nil || reopen.Quantity != 10 || reopen.AverageCost != "8.4000" || reopen.NetCost != "8.4000" {
		t.Fatalf("reopen: %+v %v", reopen, err)
	}
}

func TestTradeCanMoveEmptyBaselineBeforeFirstRecordedFill(t *testing.T) {
	setupTradeTestDB(t)
	at := baselineTrade(t)
	_, err := setTradeBaseline(tradeBaselineInput{
		Code: "sh600000", RequestID: "adjust-empty", At: at.Add(-30 * time.Minute), Quantity: 100, UnitCost: "10",
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := recordTrade(tradeAt(at.Add(-10*time.Minute), "first-sell", "sell", 40, "12"))
	if err != nil || receipt.Quantity != 60 {
		t.Fatalf("first sell = %+v, %v", receipt, err)
	}
	_, strategy, _, err := tradeStrategyInputs("sh600000", at)
	if err != nil || strategy.Sold.Quantity != 40 || strategy.Sold.NetAmount != "476.0000" {
		t.Fatalf("today's sell = %+v, %v", strategy.Sold, err)
	}
}

func TestTradeCanModifyEarlierSegment(t *testing.T) {
	setupTradeTestDB(t)
	at := baselineTrade(t)
	prior, err := recordTrade(tradeAt(at.Add(time.Minute), "old-sale", "sell", 40, "12"))
	if err != nil {
		t.Fatal(err)
	}
	baseline := tradeBaselineInput{
		Code: "sh600000", RequestID: "new-baseline", At: at.Add(2 * time.Minute), Quantity: 80, UnitCost: "9",
	}
	newPosition, err := setTradeBaseline(baseline)
	if err != nil {
		t.Fatal(err)
	}
	rule := priceAlert{StockCode: baseline.Code, Enabled: true, PositionRevision: &newPosition.Revision}
	if err := db.Dao.Create(&rule).Error; err != nil {
		t.Fatal(err)
	}
	history, err := listTrades(baseline.Code, 0, 30)
	if err != nil || len(history) != 3 || history[1].ID != prior.ID || !history[1].CanCorrect {
		t.Fatalf("earlier trade not editable: %+v, %v", history, err)
	}
	replacement := tradeAt(at.Add(time.Minute), "", "sell", 30, "11")
	change := tradeCorrection{Code: baseline.Code, RequestID: "modify-old", TargetID: prior.ID, Replacement: &replacement}
	result, err := correctTrade(change)
	if err != nil || result.Quantity != 80 || result.AverageCost != "9.0000" {
		t.Fatalf("old-segment correction = %+v, %v", result, err)
	}
	var updated priceAlert
	if err := db.Dao.First(&updated, rule.ID).Error; err != nil || !updated.Enabled ||
		updated.PositionRevision == nil || *updated.PositionRevision != result.Revision {
		t.Fatalf("current alert after archive correction = %+v, %v", updated, err)
	}
	if _, err := recordTrade(tradeAt(at.Add(3*time.Minute), "new-sale", "sell", 10, "12")); err != nil {
		t.Fatal(err)
	}
	_, strategy, _, err := tradeStrategyInputs(baseline.Code, at.Add(5*time.Minute))
	if err != nil || strategy.Sold.Quantity != 40 || strategy.TodayRealized != "52.0000" || strategy.UnmatchedShares != 10 {
		t.Fatalf("summary after editing an earlier segment = %+v, %v", strategy, err)
	}
	change.RequestID, change.TargetID = "cross-baseline", result.ID
	replacement.At = baseline.At
	if _, err := correctTrade(change); err == nil {
		t.Fatal("accepted a correction across a later baseline")
	}
}

func TestTradeCorrectionKeepsSameTimeOrder(t *testing.T) {
	setupTradeTestDB(t)
	at := time.Date(2026, 10, 30, 1, 30, 0, 0, time.UTC)
	baseline := tradeBaselineInput{Code: "sh600000", RequestID: "flat", At: at, Quantity: 0, UnitCost: "0"}
	if _, err := setTradeBaseline(baseline); err != nil {
		t.Fatal(err)
	}
	filledAt := at.Add(time.Minute)
	buy, err := recordTrade(tradeAt(filledAt, "buy-first", "buy", 100, "9"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recordTrade(tradeAt(filledAt, "sell-next", "sell", 100, "10")); err != nil {
		t.Fatal(err)
	}
	baseline.At, baseline.RequestID = filledAt, "after-same-time-fills"
	if _, err := setTradeBaseline(baseline); err != nil {
		t.Fatal(err)
	}
	replacement := tradeAt(filledAt, "", "buy", 100, "8")
	change := tradeCorrection{Code: baseline.Code, RequestID: "correct-buy-price", TargetID: buy.ID, Replacement: &replacement}
	if _, err := correctTrade(change); err != nil {
		t.Fatalf("same-time correction reordered a sell before its buy: %v", err)
	}
}

func TestTradeAtomicRetry(t *testing.T) {
	setupTradeTestDB(t)
	at := baselineTrade(t)
	in := tradeAt(at.Add(time.Minute), "buy", "buy", 10, "9")
	if err := db.Dao.Exec("CREATE TRIGGER reject_position BEFORE UPDATE OF volume ON followed_stock BEGIN SELECT RAISE(ABORT, 'test'); END").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := recordTrade(in); err == nil {
		t.Fatal("write failure did not roll back")
	}
	if err := db.Dao.Exec("DROP TRIGGER reject_position").Error; err != nil {
		t.Fatal(err)
	}
	result, err := recordTrade(in)
	if err != nil || result.Quantity != 110 {
		t.Fatalf("retry: %+v %v", result, err)
	}
	again, err := recordTrade(in)
	if err != nil || again != result {
		t.Fatalf("duplicate result: %+v %v", again, err)
	}
	in.Quantity++
	if _, err := recordTrade(in); err == nil {
		t.Fatal("same key accepted a changed request")
	}
}

func TestTradeConcurrentSell(t *testing.T) {
	setupTradeTestDB(t)
	at := baselineTrade(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, id := range []string{"sell1", "sell2"} {
		wg.Go(func() {
			_, err := recordTrade(tradeAt(at.Add(time.Minute), id, "sell", 60, "10"))
			results <- err
		})
	}
	wg.Wait()
	close(results)
	var saved int
	for err := range results {
		if err == nil {
			saved++
		}
	}
	var followed data.FollowedStock
	if err := db.Dao.First(&followed).Error; err != nil || saved != 1 || followed.Volume != 40 {
		t.Fatalf("saved = %d, position = %+v, error = %v", saved, followed, err)
	}
}
