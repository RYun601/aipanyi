package mobilecore

import (
	"path/filepath"
	"sync"
	"testing"

	"aipanyi/backend/db"
)

type fakeNotifier struct {
	mu       sync.Mutex
	status   int32
	schedule [3]int64
}

func (f *fakeNotifier) Notify(title, body, payload string) {}

func (f *fakeNotifier) UpdateMonitorStatus(active int32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = active
}

func (f *fakeNotifier) UpdateMonitorSchedule(startMs, endMs, nextStartMs int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.schedule = [3]int64{startMs, endMs, nextStartMs}
}

func (f *fakeNotifier) lastSchedule() [3]int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.schedule
}

func TestRefreshMonitorScheduleClearsWhenNoEnabledAlerts(t *testing.T) {
	dir := t.TempDir()
	db.InitWithOptions(filepath.Join(dir, "monitor.db"), db.MobileOptions())
	if sqlDB, err := db.Dao.DB(); err == nil {
		defer sqlDB.Close()
	}
	if err := db.Dao.AutoMigrate(&priceAlert{}); err != nil {
		t.Fatal(err)
	}

	notifier := &fakeNotifier{}
	setNotifier(notifier)
	defer setNotifier(nil)

	refreshMonitorSchedule()

	if got := notifier.lastSchedule(); got != [3]int64{0, 0, 0} {
		t.Fatalf("schedule = %v, want [0 0 0]", got)
	}
}
