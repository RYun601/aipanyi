package mobilecore

import (
	"path/filepath"
	"testing"

	"aipanyi/backend/db"
)

func TestBackfillAlertMarketsSetsMarketFromCode(t *testing.T) {
	dir := t.TempDir()
	db.InitWithOptions(filepath.Join(dir, "monitor.db"), db.MobileOptions())
	if sqlDB, err := db.Dao.DB(); err == nil {
		defer sqlDB.Close()
	}
	if err := db.Dao.AutoMigrate(&priceAlert{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Create(&priceAlert{StockCode: "hk00700", Market: "", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}

	if err := backfillAlertMarkets(); err != nil {
		t.Fatalf("backfillAlertMarkets() error = %v", err)
	}

	var got priceAlert
	if err := db.Dao.First(&got, "stock_code = ?", "hk00700").Error; err != nil {
		t.Fatal(err)
	}
	if got.Market != "HK" {
		t.Fatalf("market = %q, want HK", got.Market)
	}
}
