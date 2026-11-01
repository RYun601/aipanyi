package mobilecore

import (
	"sync"
	"time"

	"aipanyi/backend/db"
	"aipanyi/backend/logger"
	"aipanyi/backend/marketcal"
)

var (
	scheduleMu      sync.Mutex
	schedulePending bool
)

func scheduleMonitorRefresh() {
	if db.Dao == nil {
		return
	}
	scheduleMu.Lock()
	if schedulePending {
		scheduleMu.Unlock()
		return
	}
	schedulePending = true
	scheduleMu.Unlock()

	go func() {
		defer func() {
			scheduleMu.Lock()
			schedulePending = false
			scheduleMu.Unlock()
		}()
		refreshMonitorSchedule()
	}()
}

func enabledAlertMarkets() ([]string, error) {
	var markets []string
	err := db.Dao.Model(&priceAlert{}).
		Where("enabled = ?", true).
		Select("DISTINCT market").
		Pluck("market", &markets).Error
	return markets, err
}

func refreshMonitorSchedule() {
	markets, err := enabledAlertMarkets()
	if err != nil {
		logger.SugaredLogger.Errorf("mobile: list enabled alert markets failed: %v", err)
		return
	}
	if len(markets) == 0 {
		sendMonitorSchedule(0, 0, 0)
		logger.SugaredLogger.Info("mobile: monitor schedule cleared (no enabled alerts)")
		return
	}

	now := time.Now()
	current, ok := marketcal.NextMonitorWindow(markets, now)
	if !ok {
		sendMonitorSchedule(0, 0, 0)
		logger.SugaredLogger.Infof("mobile: monitor schedule cleared (no window) markets=%v", markets)
		return
	}

	var nextStart int64
	if next, ok := marketcal.NextMonitorWindowAfter(markets, current.End); ok {
		nextStart = next.Start.UnixMilli()
	}

	sendMonitorSchedule(current.Start.UnixMilli(), current.End.UnixMilli(), nextStart)
	logger.SugaredLogger.Infof(
		"mobile: monitor schedule markets=%v start=%s end=%s next=%s",
		markets,
		formatScheduleTime(current.Start.UnixMilli()),
		formatScheduleTime(current.End.UnixMilli()),
		formatScheduleTime(nextStart),
	)
}

func formatScheduleTime(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.UnixMilli(ms).In(time.Local).Format("2006-01-02 15:04:05")
}
