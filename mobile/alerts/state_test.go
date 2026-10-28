package alerts

import "testing"

func TestEvaluateAbove(t *testing.T) {
	cases := []struct {
		name      string
		state     string
		price     float64
		wantState string
		wantFire  bool
	}{
		{"below threshold", "below", 99, "below", false},
		{"cross up", "below", 101, "above", true},
		{"stay above", "above", 102, "above", false},
		{"back below", "above", 99, "below", false},
		{"recross", "below", 100, "above", true},
		{"unknown initializes", "unknown", 105, "above", false},
		{"unknown below initializes", "unknown", 95, "below", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, fire := Evaluate("above", 100, c.state, c.price)
			if state != c.wantState || fire != c.wantFire {
				t.Fatalf("Evaluate(above,100,%q,%v)=(%q,%v), want (%q,%v)",
					c.state, c.price, state, fire, c.wantState, c.wantFire)
			}
		})
	}
}

func TestEvaluateBelow(t *testing.T) {
	cases := []struct {
		name      string
		state     string
		price     float64
		wantState string
		wantFire  bool
	}{
		{"above threshold", "above", 11, "above", false},
		{"cross down", "above", 9.8, "below", true},
		{"stay below", "below", 9.5, "below", false},
		{"back above", "below", 10.5, "above", false},
		{"recross", "above", 10, "below", true},
		{"unknown initializes", "unknown", 9, "below", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, fire := Evaluate("below", 10, c.state, c.price)
			if state != c.wantState || fire != c.wantFire {
				t.Fatalf("Evaluate(below,10,%q,%v)=(%q,%v), want (%q,%v)",
					c.state, c.price, state, fire, c.wantState, c.wantFire)
			}
		})
	}
}

func TestEvaluateGuards(t *testing.T) {
	if _, fire := Evaluate("above", 0, "below", 10); fire {
		t.Fatal("zero threshold must not fire")
	}
	if _, fire := Evaluate("above", 10, "below", 0); fire {
		t.Fatal("zero price must not fire")
	}
	if _, fire := Evaluate("sideways", 10, "above", 20); fire {
		t.Fatal("unknown direction must not fire")
	}
}

func TestNormalizeDirection(t *testing.T) {
	cases := map[string]string{
		"above": "above", "涨到": "above", "突破": "above", "UP": "above",
		"below": "below", "跌破": "below", "低于": "below", "down": "below",
		"sideways": "",
	}
	for in, want := range cases {
		if got := NormalizeDirection(in); got != want {
			t.Errorf("NormalizeDirection(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestParseSuggestions(t *testing.T) {
	content := "```json\n" + `{"alerts":[
		{"direction":"above","price":110,"note":"突破加仓"},
		{"direction":"跌破","price":"95","note":"止损"},
		{"direction":"above","price":110,"note":"重复"},
		{"direction":"above","price":0,"note":"无效"},
		{"direction":"below","price":999,"note":"偏离过大"},
		{"direction":"sideways","price":100,"note":"方向不明"}
	]}` + "\n```"
	out, err := ParseSuggestions(content, "sh600519", "贵州茅台", 100)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(out) != 5 {
		t.Fatalf("expected 5 suggestions (duplicate removed), got %d: %+v", len(out), out)
	}
	valid := 0
	for _, s := range out {
		if s.Valid {
			valid++
			if s.Direction != "above" && s.Direction != "below" {
				t.Fatalf("invalid direction: %+v", s)
			}
		}
	}
	if valid != 2 {
		t.Fatalf("expected 2 valid suggestions, got %d: %+v", valid, out)
	}
}

func TestParseSuggestionsEmpty(t *testing.T) {
	out, err := ParseSuggestions(`{"alerts":[]}`, "sh600519", "贵州茅台", 100)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty, got %+v", out)
	}
}

func TestStripJSONFence(t *testing.T) {
	in := "好的：\n```json\n{\"alerts\":[]}\n```\n"
	if got := StripJSONFence(in); got != `{"alerts":[]}` {
		t.Fatalf("unexpected: %q", got)
	}
}
