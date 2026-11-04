package mobilecore

import (
	"context"
	"testing"
	"time"

	"aipanyi/backend/db"
	"aipanyi/mobile/alerts"
)

func TestTradeStrategyRefresh(t *testing.T) {
	setupTradeTestDB(t)
	at := baselineTrade(t)
	now := at.Add(10 * time.Minute)
	sell, err := recordTrade(tradeAt(at.Add(time.Minute), "sell", "sell", 40, "12"))
	if err != nil {
		t.Fatal(err)
	}
	noQuote := func(context.Context, string, time.Time) alerts.QuoteSnapshot { return alerts.QuoteSnapshot{} }
	first, err := refreshTradeStrategy(context.Background(), "sh600000", now, noQuote)
	if err != nil || first.Status != "quote_pending" || first.Sold.Quantity != 40 || first.UnmatchedShares != 40 ||
		len(first.TradeIDs) != 1 || first.TradeIDs[0] != sell.ID {
		t.Fatalf("after sell: %+v, %v", first, err)
	}
	buy, err := recordTrade(tradeAt(at.Add(2*time.Minute), "buy", "buy", 40, "9"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := refreshTradeStrategy(context.Background(), "sh600000", now.Add(time.Second), noQuote)
	if err != nil || second.Position.Revision != buy.Revision || second.Bought.Quantity != 40 || second.MatchedShares != 40 ||
		second.UnmatchedShares != 0 || second.MatchedSavings != "112.0000" || second.Position.NetCost != "8.8800" {
		t.Fatalf("after buy: %+v, %v", second, err)
	}
}

func TestTradeStrategyRejectsStaleVersion(t *testing.T) {
	setupTradeTestDB(t)
	at := baselineTrade(t)
	now := at.Add(10 * time.Minute)
	fetch := func(context.Context, string, time.Time) alerts.QuoteSnapshot {
		if _, err := recordTrade(tradeAt(at.Add(time.Minute), "sell", "sell", 40, "12")); err != nil {
			t.Fatal(err)
		}
		return alerts.QuoteSnapshot{}
	}
	result, err := refreshTradeStrategy(context.Background(), "sh600000", now, fetch)
	if err != nil || result.Position.Revision != 2 || result.Position.Quantity != 60 {
		t.Fatalf("latest strategy: %+v %v", result, err)
	}
	stored, err := readTradePosition(db.Dao, "sh600000")
	if err != nil || stored.StrategyJSON != "" || !stored.Pending {
		t.Fatalf("stale evaluation was persisted: %+v %v", stored, err)
	}
}

func TestTradeStrategyPairsAcrossDays(t *testing.T) {
	setupTradeTestDB(t)
	at := baselineTrade(t)
	sell, err := recordTrade(tradeAt(at.Add(time.Minute), "sell", "sell", 40, "12"))
	if err != nil {
		t.Fatal(err)
	}
	buy, err := recordTrade(tradeAt(at.Add(24*time.Hour), "buy", "buy", 20, "9"))
	if err != nil {
		t.Fatal(err)
	}
	_, out, _, err := tradeStrategyInputs("sh600000", at.Add(25*time.Hour))
	if err != nil || out.Sold.Quantity != 0 || out.Bought.Quantity != 20 ||
		out.MatchedShares != 20 || out.UnmatchedShares != 20 || out.MatchedSavings != "54.0000" ||
		len(out.TradeIDs) != 2 || out.TradeIDs[0] != buy.ID || out.TradeIDs[1] != sell.ID {
		t.Fatalf("cross-day strategy: %+v, %v", out, err)
	}
}

func TestTradeInvalidatesPositionRules(t *testing.T) {
	setupTradeTestDB(t)
	at := baselineTrade(t)
	revision := int64(1)
	rules := []priceAlert{
		{StockCode: "sh600000", Mode: "strategy", PositionRevision: &revision, Enabled: true},
		{StockCode: "sh600000", Mode: "price", Enabled: true},
	}
	if err := db.Dao.Create(&rules).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := recordTrade(tradeAt(at.Add(time.Minute), "buy", "buy", 10, "9")); err != nil {
		t.Fatal(err)
	}
	var stale, price priceAlert
	if err := db.Dao.First(&stale, rules[0].ID).Error; err != nil || stale.Enabled || stale.strategyState().Status != "position_changed" {
		t.Fatalf("position rule: %+v, %v", stale, err)
	}
	if err := db.Dao.First(&price, rules[1].ID).Error; err != nil || !price.Enabled {
		t.Fatalf("price rule: %+v, %v", price, err)
	}
}
