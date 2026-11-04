package alerts

import (
	"math"
	"time"
)

type StrategyRule struct {
	Direction      string
	Price          float64
	Phase          string
	MinVolumeRatio float64
	BookSide       string
	MinBookBias    float64
	MaxAge         time.Duration
}

type QuoteSnapshot struct {
	Market      string
	Session     string
	At          time.Time
	Price       float64
	VolumeRatio *float64
	BookBias    *float64
}

type StrategyState struct {
	Status      string
	Reason      string
	Session     string
	SampleAt    time.Time
	Consecutive int
	Latched     bool
	NotifiedAt  time.Time
}

type Decision struct {
	State     StrategyState
	Evidence  QuoteSnapshot
	Confirmed bool
}

func (r StrategyRule) Valid() bool {
	return (r.Direction == "above" || r.Direction == "below") && positive(r.Price) &&
		positive(r.MinVolumeRatio) && (r.BookSide == "buy" || r.BookSide == "sell") &&
		r.MinBookBias > 0 && r.MinBookBias <= 1
}

func positive(v float64) bool { return v > 0 && !math.IsInf(v, 0) }

func EvaluateStrategy(rule StrategyRule, q QuoteSnapshot, state StrategyState, now time.Time) Decision {
	result := Decision{Evidence: q}
	wait := func(status, reason string) Decision {
		state.Status, state.Reason, state.Consecutive = status, reason, 0
		result.State = state
		return result
	}
	if rule.Phase == "close" {
		return wait("waiting", "close_review")
	}
	if !rule.Valid() {
		return wait("waiting", "missing_conditions")
	}
	if q.Session == "" || q.At.IsZero() || q.At.After(now) || now.Sub(q.At) > rule.MaxAge || !positive(q.Price) {
		return wait("waiting", "quote_unavailable")
	}
	if !state.SampleAt.IsZero() && !q.At.After(state.SampleAt) {
		result.State = state
		return result
	}
	if q.Session != state.Session || q.At.Sub(state.SampleAt) > rule.MaxAge {
		state.Consecutive = 0
	}
	state.Session, state.SampleAt = q.Session, q.At
	touched := (rule.Direction == "above" && q.Price >= rule.Price) || (rule.Direction == "below" && q.Price <= rule.Price)
	if !touched {
		state.Latched = false
		return wait("invalidated", "outside_price")
	}
	if q.VolumeRatio == nil || q.BookBias == nil || !positive(*q.VolumeRatio) || math.IsNaN(*q.BookBias) || math.Abs(*q.BookBias) > 1 {
		return wait("observing", "missing_indicators")
	}
	bias := *q.BookBias
	if rule.BookSide == "sell" {
		bias = -bias
	}
	if *q.VolumeRatio < rule.MinVolumeRatio || bias < rule.MinBookBias {
		state.Latched = false
		return wait("invalidated", "indicators_below_threshold")
	}
	state.Consecutive = min(state.Consecutive+1, 2)
	if state.Consecutive < 2 && !state.Latched {
		state.Status, state.Reason = "observing", "need_next_sample"
	} else {
		state.Status, state.Reason = "confirmed", "conditions_met"
		if state.Consecutive == 2 && !state.Latched && now.Sub(state.NotifiedAt) >= 5*time.Minute {
			state.Latched, state.NotifiedAt, result.Confirmed = true, now, true
		}
	}
	result.State = state
	return result
}
