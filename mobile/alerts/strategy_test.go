package alerts

import (
	"testing"
	"time"
)

func TestEvaluateStrategyConfirmation(t *testing.T) {
	start := time.Date(2026, 11, 4, 2, 0, 0, 0, time.UTC)
	rule := StrategyRule{Direction: "above", Price: 10, MinVolumeRatio: 1.5, BookSide: "buy", MinBookBias: 0.15, MaxAge: time.Minute}
	state := StrategyState{}
	ratio, bias := 1.6, 0.2
	first := QuoteSnapshot{At: start, Price: 10.01, BookBias: &bias, Session: "AM"}
	second := QuoteSnapshot{At: start.Add(30 * time.Second), Price: 10.02, VolumeRatio: &ratio, BookBias: &bias, Session: "AM"}
	third := second
	third.At = third.At.Add(30 * time.Second)
	for i, q := range []QuoteSnapshot{first, second, second, third} {
		result := EvaluateStrategy(rule, q, state, q.At)
		if result.Confirmed != (i == 3) {
			t.Fatalf("sample %d: %+v", i, result)
		}
		state = result.State
	}
	stale := EvaluateStrategy(rule, third, state, third.At.Add(2*time.Minute))
	if stale.State.Status == "confirmed" || stale.Confirmed {
		t.Fatalf("stale quote: %+v", stale)
	}
}

func TestEvaluateStrategyBreakdownWithBuyingSupport(t *testing.T) {
	start := time.Date(2026, 11, 4, 2, 0, 0, 0, time.UTC)
	ratio, bias := 1.6, 0.2
	rule := StrategyRule{Direction: "below", Price: 10, MinVolumeRatio: 1.5, BookSide: "buy", MinBookBias: 0.15, MaxAge: time.Minute}
	quote := QuoteSnapshot{At: start, Price: 9.9, VolumeRatio: &ratio, BookBias: &bias, Session: "AM"}
	first := EvaluateStrategy(rule, quote, StrategyState{}, start)
	quote.Session = "PM"
	quote.At = start.Add(30 * time.Second)
	second := EvaluateStrategy(rule, quote, first.State, quote.At)
	if second.Confirmed {
		t.Fatal("samples straddling sessions were counted together")
	}
	quote.At = quote.At.Add(30 * time.Second)
	if !EvaluateStrategy(rule, quote, second.State, quote.At).Confirmed {
		t.Fatal("two valid breakdown samples with buying support should confirm")
	}
}
