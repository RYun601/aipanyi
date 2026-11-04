package mobilecore

import (
	"context"
	"strings"
	"testing"
	"time"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/mobile/alerts"
)

type monitorTestNotifier struct {
	payloads []string
}

func (n *monitorTestNotifier) Notify(_, _, payload string)             { n.payloads = append(n.payloads, payload) }
func (*monitorTestNotifier) UpdateMonitorStatus(int32)                 {}
func (*monitorTestNotifier) UpdateMonitorSchedule(int64, int64, int64) {}

func TestAlertEngineGroupsStrategyNotifications(t *testing.T) {
	setupTradeTestDB(t)
	if err := db.Dao.AutoMigrate(&mobileSettings{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Dao.Create(&mobileSettings{ID: 1, PushEnabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	for _, threshold := range []float64{9.55, 9.61} {
		if err := db.Dao.Create(&priceAlert{StockCode: "sh600000", Mode: "strategy", Market: "A",
			Phase: "intraday", Enabled: true, Direction: "above", Price: threshold,
			MinVolumeRatio: 1.5, BookSide: "buy", MinBookBias: .15, LastState: "unknown"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	notifier := &monitorTestNotifier{}
	setNotifier(notifier)
	t.Cleanup(func() { setNotifier(nil) })
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, alertQuoteZone)
	sourceAt := now
	engine := newAlertEngine()
	engine.now = func() time.Time { return now }
	engine.isTrading = func(context.Context, string, time.Time) bool { return true }
	engine.quotes = func(context.Context, []string) (map[string]data.StockInfo, error) {
		fields := make([]string, 55)
		fields[1], fields[3], fields[4], fields[5] = "TEST", "9.63", "9.60", "9.55"
		for i := 9; i < 29; i += 2 {
			fields[i], fields[i+1] = "9.62", "100"
			if i < 19 {
				fields[i+1] = "150"
			}
		}
		fields[30], fields[49] = sourceAt.Format("20060102150405"), "1.6"
		quote, err := data.ParseTxStockData("v_sh600000=\"" + strings.Join(fields, "~") + "\"")
		if err != nil {
			t.Fatal(err)
		}
		return map[string]data.StockInfo{"sh600000": *quote}, nil
	}
	if got := engine.tick(context.Background(), 30*time.Second); got != 0 {
		t.Fatalf("first sample triggered %d rules", got)
	}
	now = now.Add(30 * time.Second)
	if got := engine.tick(context.Background(), 30*time.Second); got != 0 {
		t.Fatalf("duplicate timestamp triggered %d rules", got)
	}
	now = now.Add(20 * time.Second)
	sourceAt = now
	if got := engine.tick(context.Background(), 30*time.Second); got != 2 || len(notifier.payloads) != 1 {
		t.Fatalf("confirmed rules = %d, notifications = %v", got, notifier.payloads)
	}
	var events []priceAlertEvent
	if err := db.Dao.Order("id").Find(&events).Error; err != nil || len(events) != 2 ||
		events[0].Batch == "" || events[0].Batch != events[1].Batch || events[1].evidence() == nil {
		t.Fatalf("history = %+v, %v", events, err)
	}
}

func TestAlertEnginePollDeadline(t *testing.T) {
	e := &alertEngine{
		listEnabled: func() ([]priceAlert, error) {
			return []priceAlert{{StockCode: "sh600000", Direction: "above", Price: 9, LastState: "below"}}, nil
		},
		isTrading: func(context.Context, string, time.Time) bool { return true },
		now:       time.Now,
		quotes: func(ctx context.Context, _ []string) (map[string]data.StockInfo, error) {
			<-ctx.Done()
			return map[string]data.StockInfo{"sh600000": {Price: "10"}}, nil
		},
	}
	if got := e.tick(context.Background(), 15*time.Millisecond); got != 0 {
		t.Fatalf("expired round triggered %d rules", got)
	}
}

func TestStrategyAlertNotificationLifecycle(t *testing.T) {
	setupAlertTestDB(t)
	a := priceAlert{StockCode: "sh600000", Mode: "strategy", Revision: 1, Enabled: true, Price: 9, Direction: "above", LastState: "unknown"}
	if err := db.Dao.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	decision := alerts.Decision{State: alerts.StrategyState{Status: "confirmed", NotifiedAt: now}, Confirmed: true,
		Evidence: alerts.QuoteSnapshot{Price: 10, At: now}}
	if err := db.Dao.Exec("CREATE TRIGGER reject_alert_event BEFORE INSERT ON mobile_price_alert_event BEGIN SELECT RAISE(ABORT, 'test'); END").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := persistStrategyDecision(a, decision, now, "round-1"); err == nil {
		t.Fatal("event insertion should fail")
	}
	var stored priceAlert
	if err := db.Dao.First(&stored, a.ID).Error; err != nil || stored.StrategyState != "" || stored.TriggerCount != 0 {
		t.Fatalf("decision was not rolled back: %+v, %v", stored, err)
	}
	if err := db.Dao.Exec("DROP TRIGGER reject_alert_event").Error; err != nil {
		t.Fatal(err)
	}
	event, err := persistStrategyDecision(a, decision, now, "round-1")
	if err != nil || event == nil || event.Price != 10 || event.Batch != "round-1" {
		t.Fatalf("event = %+v, %v", event, err)
	}
	if event, err := persistStrategyDecision(a, decision, now, "round-1"); err != nil || event != nil {
		t.Fatalf("duplicate sample event = %+v, %v", event, err)
	}
	if err := db.Dao.Model(&a).Update("revision", 2).Error; err != nil {
		t.Fatal(err)
	}
	if event, err := persistStrategyDecision(a, decision, now.Add(time.Minute), "round-2"); err != nil || event != nil {
		t.Fatalf("stale rule event = %+v, %v", event, err)
	}
}

func TestAlertEngineResumeNoBackfill(t *testing.T) {
	interval := 30 * time.Second
	if delay := nextPollDelay(interval, 22*time.Minute); delay != interval {
		t.Fatalf("resume delay = %s, want one full interval", delay)
	}
	if delay := nextPollDelay(interval, 3*time.Second); delay != 27*time.Second {
		t.Fatalf("normal poll delay = %s", delay)
	}
}

func TestAlertEngineCalendarDeadline(t *testing.T) {
	e := &alertEngine{
		listEnabled: func() ([]priceAlert, error) {
			return []priceAlert{{StockCode: "sh600000", Market: "A"}}, nil
		},
		isTrading: func(ctx context.Context, _ string, _ time.Time) bool {
			<-ctx.Done()
			return false
		},
		now: time.Now,
		quotes: func(context.Context, []string) (map[string]data.StockInfo, error) {
			t.Fatal("quote request started after the calendar deadline")
			return nil, nil
		},
	}
	if got := e.tick(context.Background(), 15*time.Millisecond); got != 0 {
		t.Fatalf("expired calendar round triggered %d rules", got)
	}
}

func TestPriceQuoteRecoveryRejectsStaleAndOutOfOrderSamples(t *testing.T) {
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, alertQuoteZone)
	for _, at := range []time.Time{now.Add(-22 * time.Minute), now.Add(-30 * time.Second)} {
		if usablePriceQuote("sh600000", at, now.Add(-10*time.Second), now, time.Minute) {
			t.Fatalf("accepted old source time %s", at)
		}
	}
	if !usablePriceQuote("sh600000", now.Add(-5*time.Second), now.Add(-10*time.Second), now, time.Minute) {
		t.Fatal("fresh sample was rejected")
	}
}
