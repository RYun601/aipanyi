package mobilecore

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/mobile/alerts"
)

func setupAlertTestDB(t *testing.T) {
	t.Helper()
	db.InitWithOptions(filepath.Join(t.TempDir(), "alerts.db"), db.MobileOptions())
	conn, err := db.Dao.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Dao.AutoMigrate(&data.FollowedStock{}, &priceAlert{}, &priceAlertEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Create(&data.FollowedStock{StockCode: "sh600000", Name: "测试"}).Error; err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 49; i++ {
		if err := db.Dao.Create(&priceAlert{StockCode: "sh600000", Direction: "above", Price: float64(i), Enabled: true}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestSaveAlertSuggestionsReplacesStockRules(t *testing.T) {
	setupAlertTestDB(t)
	oldAI := priceAlert{StockCode: "sh600000", Direction: "below", Price: 15, Source: "ai"}
	otherStock := priceAlert{StockCode: "sz000001", Direction: "above", Price: 10, Enabled: true}
	for _, rule := range []*priceAlert{&oldAI, &otherStock} {
		if err := db.Dao.Create(rule).Error; err != nil {
			t.Fatal(err)
		}
	}
	event := priceAlertEvent{AlertID: oldAI.ID, StockCode: oldAI.StockCode, Price: 15}
	if err := db.Dao.Create(&event).Error; err != nil {
		t.Fatal(err)
	}
	items := []alertDTO{
		{Code: "sh600000", Direction: "above", Price: 1, Enabled: true, Mode: "price", Source: "ai"},
		{Code: "sh600000", Direction: "below", Price: 20, Phase: "close", Mode: "strategy", Source: "ai"},
	}
	for range 2 {
		for i, result := range writeAlerts(items, nil) {
			if result.Index != i || result.Status != "saved" {
				t.Fatalf("replacement result %d = %+v", i, result)
			}
		}
		rules := listAlerts("sh600000")
		if len(rules) != 2 || rules[0].Price != 1 || rules[1].Price != 20 || !rules[1].NeedsReview {
			t.Fatalf("replacement rules = %+v", rules)
		}
	}
	if rules := listAlerts("sz000001"); len(rules) != 1 || rules[0].ID != int64(otherStock.ID) || !rules[0].Enabled {
		t.Fatalf("other stock changed: %+v", rules)
	}
	if err := db.Dao.First(&priceAlertEvent{}, event.ID).Error; err != nil {
		t.Fatalf("old trigger history was removed: %v", err)
	}
}

func TestSaveAlertSuggestionsRollsBackFailedBatch(t *testing.T) {
	setupAlertTestDB(t)
	results := writeAlerts([]alertDTO{
		{Code: "sh600000", Direction: "above", Price: 50, Enabled: true, Mode: "price", Source: "ai"},
		{Code: "sh600000", Direction: "above", Price: -1, Enabled: true, Mode: "price", Source: "ai"},
	}, nil)
	if results[0].Status != "rolled_back" || results[0].ID != 0 || results[1].Status != "invalid" {
		t.Fatalf("replacement failure results = %+v", results)
	}
	if rules := listAlerts("sh600000"); len(rules) != 49 || lenAlertsEnabled() != 49 {
		t.Fatalf("rollback left %d rules, %d enabled", len(rules), lenAlertsEnabled())
	}
}

func TestSaveAlertSuggestionsSavesStrategyWhenQuoteUnavailable(t *testing.T) {
	setupAlertTestDB(t)
	results := writeAlerts([]alertDTO{
		{Code: "sh600000", Direction: "above", Price: 50, Enabled: true, Mode: "price", Source: "ai"},
		{Code: "sh600000", Direction: "below", Price: 40, Enabled: true, Mode: "strategy", Phase: "intraday",
			MinVolumeRatio: 1.5, BookSide: "sell", MinBookBias: 0.15, Source: "ai"},
	}, func(string) alerts.QuoteSnapshot { return alerts.QuoteSnapshot{} })
	if results[0].Status != "saved" || results[1].Status != "saved" || results[1].Message == "" {
		t.Fatalf("strategy should be saved disabled with a message: %+v", results)
	}
	rules := listAlerts("sh600000")
	if len(rules) != 2 || !rules[0].Enabled || rules[1].Enabled || rules[1].NeedsReview ||
		rules[1].BookSide != "sell" || rules[1].MinVolumeRatio != 1.5 || rules[1].MinBookBias != 0.15 {
		t.Fatalf("replacement lost enabled price rule or strategy conditions: %+v", rules)
	}
}

func TestSaveAlertSuggestionsEnablesStrategiesFromOneQuote(t *testing.T) {
	setupAlertTestDB(t)
	ratio, bias := 1.5, 0.2
	calls := 0
	items := []alertDTO{
		{Code: "sh600000", Direction: "above", Price: 50, Enabled: true, Mode: "strategy", Source: "ai",
			MinVolumeRatio: 1.5, BookSide: "buy", MinBookBias: 0.15},
		{Code: "sh600000", Direction: "below", Price: 40, Enabled: true, Mode: "strategy", Source: "ai",
			MinVolumeRatio: 1.5, BookSide: "sell", MinBookBias: 0.15},
	}
	results := writeAlerts(items, func(code string) alerts.QuoteSnapshot {
		calls++
		if code != "sh600000" {
			t.Fatalf("quote requested for %q", code)
		}
		return alerts.QuoteSnapshot{At: time.Now(), Price: 45, VolumeRatio: &ratio, BookBias: &bias}
	})
	if calls != 1 || results[0].Status != "saved" || results[1].Status != "saved" || lenAlertsEnabled() != 2 {
		t.Fatalf("quote fetched %d times, results = %+v, enabled = %d", calls, results, lenAlertsEnabled())
	}
}

func TestAlertEnableLimitConcurrent(t *testing.T) {
	setupAlertTestDB(t)
	rows := []priceAlert{
		{StockCode: "sh600000", Direction: "above", Price: 50},
		{StockCode: "sh600000", Direction: "above", Price: 51},
	}
	if err := db.Dao.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, len(rows))
	for _, row := range rows {
		wg.Go(func() { results <- writeAlertEnabled(int64(row.ID), true) })
	}
	wg.Wait()
	close(results)
	var succeeded int
	for err := range results {
		if err == nil {
			succeeded++
		} else if err != errAlertLimit {
			t.Fatalf("unexpected enable error: %v", err)
		}
	}
	if succeeded != 1 || lenAlertsEnabled() != 50 {
		t.Fatalf("successful enables = %d, enabled = %d", succeeded, lenAlertsEnabled())
	}
}

func TestSaveAlertSuggestionsIgnoresEmptyLedgerRevision(t *testing.T) {
	setupAlertTestDB(t)
	zero := int64(0)
	results := writeAlerts([]alertDTO{
		{Code: "sh600000", Direction: "above", Price: 50, Enabled: true, Mode: "strategy", Phase: "intraday",
			MinVolumeRatio: 1.5, BookSide: "buy", MinBookBias: 0.15,
			UsePosition: true, PositionRevision: &zero, Source: "ai"},
	}, func(string) alerts.QuoteSnapshot { return alerts.QuoteSnapshot{} })
	if results[0].Status != "saved" {
		t.Fatalf("suggestion saved before ledger exists = %+v", results)
	}
	rules := listAlerts("sh600000")
	if len(rules) != 1 || rules[0].UsePosition || rules[0].PositionRevision != nil {
		t.Fatalf("empty ledger revision should not bind position: %+v", rules)
	}
}

func TestSaveAlertSuggestionsDropsVanishedLedgerRevision(t *testing.T) {
	setupAlertTestDB(t)
	if err := db.Dao.AutoMigrate(&tradePosition{}); err != nil {
		t.Fatal(err)
	}
	revision := int64(2)
	results := writeAlerts([]alertDTO{
		{Code: "sh600000", Direction: "above", Price: 50, Enabled: true, Mode: "strategy", Phase: "intraday",
			MinVolumeRatio: 1.5, BookSide: "buy", MinBookBias: 0.15,
			UsePosition: true, PositionRevision: &revision, Source: "ai"},
	}, func(string) alerts.QuoteSnapshot { return alerts.QuoteSnapshot{} })
	if results[0].Status != "saved" {
		t.Fatalf("stale analysis basis without ledger = %+v", results)
	}
	rules := listAlerts("sh600000")
	if len(rules) != 1 || rules[0].UsePosition || rules[0].PositionRevision != nil {
		t.Fatalf("vanished ledger revision should not bind position: %+v", rules)
	}
}

func TestSaveAlertSuggestionsBindsCurrentLedgerRevision(t *testing.T) {
	setupAlertTestDB(t)
	if err := db.Dao.AutoMigrate(&tradePosition{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Create(&tradePosition{Code: "sh600000", Revision: 2, Quantity: 100}).Error; err != nil {
		t.Fatal(err)
	}
	revision := int64(2)
	ratio, bias := 1.2, 0.15
	results := writeAlerts([]alertDTO{
		{Code: "sh600000", Direction: "below", Price: 40, Enabled: true, Mode: "strategy", Phase: "intraday",
			MinVolumeRatio: 1.2, BookSide: "sell", MinBookBias: 0.15,
			UsePosition: true, PositionRevision: &revision, Source: "ai"},
	}, func(string) alerts.QuoteSnapshot {
		return alerts.QuoteSnapshot{At: time.Now(), Price: 45, VolumeRatio: &ratio, BookBias: &bias}
	})
	if results[0].Status != "saved" {
		t.Fatalf("suggestion with current ledger revision = %+v", results)
	}
	rules := listAlerts("sh600000")
	if len(rules) != 1 || !rules[0].UsePosition || rules[0].PositionRevision == nil || *rules[0].PositionRevision != 2 {
		t.Fatalf("current ledger revision should bind position: %+v", rules)
	}
}

func TestAlertModeMigrationKeepsExistingHistory(t *testing.T) {
	setupAlertTestDB(t)
	legacy := priceAlert{StockCode: "sh600000", Source: "ai", Price: 9.05, Note: "收盘站稳", Enabled: true, TriggerCount: 2}
	if err := db.Dao.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Create(&priceAlertEvent{AlertID: legacy.ID, StockCode: legacy.StockCode, Price: 9.06}).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := migrateAlertModes(); err != nil {
			t.Fatal(err)
		}
	}
	var stored priceAlert
	if err := db.Dao.First(&stored, legacy.ID).Error; err != nil || stored.Enabled || !stored.NeedsReview ||
		stored.Mode != "strategy" || stored.TriggerCount != 2 || stored.Note != legacy.Note {
		t.Fatalf("legacy AI rule: %+v, %v", stored, err)
	}
	var count int64
	if err := db.Dao.Model(&priceAlertEvent{}).Where("alert_id = ?", legacy.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("history count = %d, %v", count, err)
	}
	if lenAlertsEnabled() != 49 {
		t.Fatalf("enabled manual rules = %d", lenAlertsEnabled())
	}
}
