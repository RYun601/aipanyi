package data

import (
	"strings"
	"testing"
)

func testPositionFollows() []FollowedStock {
	return []FollowedStock{
		{StockCode: "sh600519", Name: "贵州茅台", CostPrice: 1500.5, Volume: 100, Price: 1450},
		{StockCode: "sz000001", Name: "平安银行", CostPrice: 10, Volume: 1000, Price: 11},
		{StockCode: "sz300750", Name: "宁德时代", CostPrice: 200, Volume: 100, Price: 210},
		{StockCode: "sh601318", Name: "中国平安", CostPrice: 0, Volume: 0},
	}
}

func TestBuildPositionContextOnlySelectedConfiguredStocks(t *testing.T) {
	follows := testPositionFollows()
	quotes := map[string]StockInfo{
		"sh600519": {Price: "1400.00"},
		"sz000001": {Price: "12.00"},
		"sz300750": {Price: "220.00"},
	}

	ctx := buildPositionContext("sh600519", 0, follows, quotes, []string{
		"sh600519", "sz000001", "sh600519", "sh601318", "bad-code",
	})

	if !ctx.Enabled {
		t.Fatal("expected enabled context")
	}
	if ctx.Count != 2 {
		t.Fatalf("expected 2 selected positions, got %d", ctx.Count)
	}
	if !ctx.CurrentIncluded {
		t.Fatal("expected current stock to be included")
	}
	for _, want := range []string{
		"贵州茅台（当前分析）",
		"平安银行",
		"已勾选的持仓",
		"当前股票在本次勾选组合中",
		"占比不代表总资产仓位",
	} {
		if !strings.Contains(ctx.Markdown, want) {
			t.Errorf("markdown missing %q\n%s", want, ctx.Markdown)
		}
	}
	if strings.Contains(ctx.Markdown, "宁德时代") {
		t.Errorf("unselected stock must not be included\n%s", ctx.Markdown)
	}
	if strings.Contains(ctx.Markdown, "中国平安") {
		t.Errorf("stock without cost/volume must not be included\n%s", ctx.Markdown)
	}
	if want := "|贵州茅台（当前分析）|sh600519|1500.50|100|1400.00|140000.00|150050.00|-10050.00|92.11%|"; !strings.Contains(ctx.Markdown, want) {
		t.Errorf("unexpected current stock row, want %q\n%s", want, ctx.Markdown)
	}
	if want := "|平安银行|sz000001|10.00|1000|12.00|12000.00|10000.00|+2000.00|7.89%|"; !strings.Contains(ctx.Markdown, want) {
		t.Errorf("unexpected second stock row, want %q\n%s", want, ctx.Markdown)
	}
}

func TestBuildPositionContextCurrentStockNotSelected(t *testing.T) {
	follows := testPositionFollows()
	quotes := map[string]StockInfo{
		"sz000001": {Price: "12.00"},
	}

	ctx := buildPositionContext("sh600519", 0, follows, quotes, []string{"sz000001"})

	if ctx.Count != 1 {
		t.Fatalf("expected 1 selected position, got %d", ctx.Count)
	}
	if ctx.CurrentIncluded {
		t.Fatal("current stock should not be included")
	}
	if !strings.Contains(ctx.Markdown, "当前股票不在本次勾选的持仓组合中") {
		t.Errorf("missing current-not-selected note\n%s", ctx.Markdown)
	}
	if strings.Contains(ctx.Markdown, "当前分析") {
		t.Errorf("current stock should not be marked as included\n%s", ctx.Markdown)
	}
}

func TestBuildPositionContextFallbackAndNoQuote(t *testing.T) {
	follows := testPositionFollows()
	follows[1].Price = 0 // 平安银行：无实时行情且没有上次价格 -> 显示 —
	quotes := map[string]StockInfo{
		"sh600519": {Price: ""}, // 回退到 followed_stock.Price=1450
	}

	ctx := buildPositionContext("", 0, follows, quotes, []string{"sh600519", "sz000001"})

	if ctx.Count != 2 {
		t.Fatalf("expected 2 selected positions, got %d", ctx.Count)
	}
	if !strings.Contains(ctx.Markdown, "1450.00*") {
		t.Errorf("expected fallback marker for current stock\n%s", ctx.Markdown)
	}
	if !strings.Contains(ctx.Markdown, "—") {
		t.Errorf("expected no-quote marker\n%s", ctx.Markdown)
	}
	if !strings.Contains(ctx.Markdown, "带 * 的现价使用了上次行情") {
		t.Errorf("expected fallback note\n%s", ctx.Markdown)
	}
	if !strings.Contains(ctx.Markdown, "— 表示暂无行情") {
		t.Errorf("expected no-quote note\n%s", ctx.Markdown)
	}
}

func TestBuildPositionContextRejectsUnconfiguredStocks(t *testing.T) {
	follows := testPositionFollows()
	ctx := buildPositionContext("sh600519", 0, follows, nil, []string{"sh601318", "bad-code"})

	if ctx.Count != 0 || ctx.Markdown != "" {
		t.Fatalf("expected empty context, got count=%d markdown=%q", ctx.Count, ctx.Markdown)
	}
}

func TestBuildPositionContextCurrentPriceOverride(t *testing.T) {
	follows := testPositionFollows()
	quotes := map[string]StockInfo{
		"sh600519": {Price: "1400.00"},
	}

	ctx := buildPositionContext("sh600519", 1600, follows, quotes, []string{"sh600519"})

	if !strings.Contains(ctx.Markdown, "|1600.00|160000.00|") {
		t.Errorf("expected current price override\n%s", ctx.Markdown)
	}
}

func TestApplyPositionTemplates(t *testing.T) {
	followed := FollowedStock{CostPrice: 1500.5, Volume: 100}
	cases := []struct {
		name     string
		position *PositionContext
		cost     string
		volume   string
	}{
		{
			name:     "legacy nil includes current stock",
			position: nil,
			cost:     "1500.5",
			volume:   "100",
		},
		{
			name:     "switch off hides current stock",
			position: &PositionContext{Enabled: false},
			cost:     "未提供",
			volume:   "未提供",
		},
		{
			name:     "switch on but current not selected hides current stock",
			position: &PositionContext{Enabled: true, CurrentIncluded: false},
			cost:     "未提供",
			volume:   "未提供",
		},
		{
			name:     "switch on and current selected includes current stock",
			position: &PositionContext{Enabled: true, CurrentIncluded: true},
			cost:     "1500.5",
			volume:   "100",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			templates := map[string]string{}
			applyPositionTemplates(templates, followed, tc.position)
			for _, key := range []string{"{{costPrice}}", "{costPrice}", "costPrice"} {
				if templates[key] != tc.cost {
					t.Errorf("%s = %q, want %q", key, templates[key], tc.cost)
				}
			}
			for _, key := range []string{"{{costVolume}}", "{costVolume}", "costVolume"} {
				if templates[key] != tc.volume {
					t.Errorf("%s = %q, want %q", key, templates[key], tc.volume)
				}
			}
		})
	}
}

func TestApplyPositionTemplatesWithoutCurrentPosition(t *testing.T) {
	templates := map[string]string{}
	applyPositionTemplates(templates, FollowedStock{}, nil)
	if templates["costPrice"] != "未设置" || templates["costVolume"] != "未设置" {
		t.Fatalf("legacy missing position should be 未设置, got %q/%q", templates["costPrice"], templates["costVolume"])
	}
}

func TestReplacePromptTemplatesUsesLongestPlaceholdersFirst(t *testing.T) {
	templates := map[string]string{
		"{{stockName}}": "赛力斯", "{stockName}": "赛力斯", "stockName": "赛力斯",
		"{{stockCode}}": "sh601127", "{stockCode}": "sh601127", "stockCode": "sh601127",
		"{{costPrice}}": "未提供", "{costPrice}": "未提供", "costPrice": "未提供",
		"{{costVolume}}": "未提供", "{costVolume}": "未提供", "costVolume": "未提供",
	}

	got := replacePromptTemplates("{{stockName}} {{stockCode}} 成本 {{costPrice}} 持仓 {costVolume}", templates)
	want := "赛力斯 sh601127 成本 未提供 持仓 未提供"
	if got != want {
		t.Fatalf("replacePromptTemplates() = %q, want %q", got, want)
	}
}
