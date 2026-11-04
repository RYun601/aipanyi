package mobilecore

import (
	"path/filepath"
	"sync"
	"testing"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
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

func TestSaveAlertSuggestionsPartialAndLimit(t *testing.T) {
	setupAlertTestDB(t)
	items := []alertDTO{
		{Code: "sh600000", Direction: "above", Price: 1, Enabled: true},
		{Code: "sh600000", Direction: "above", Price: 50, Enabled: true},
		{Code: "sh600000", Direction: "above", Price: 51, Enabled: true},
		{Code: "sh600000", Direction: "above", Price: -1, Enabled: true},
	}
	results := writeAlerts(items)
	for i, want := range []string{"duplicate", "saved", "limit", "invalid"} {
		if results[i].Index != i || results[i].Status != want {
			t.Fatalf("result %d = %+v, want %s", i, results[i], want)
		}
	}
	retry := writeAlerts(items[1:2])[0]
	if retry.Status != "duplicate" || retry.ID != results[1].ID || lenAlertsEnabled() != 50 {
		t.Fatalf("retry = %+v, first save = %+v, enabled = %d", retry, results[1], lenAlertsEnabled())
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
