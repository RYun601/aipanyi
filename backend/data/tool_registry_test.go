package data

import "testing"

func TestExcludeTools(t *testing.T) {
	tools := []Tool{
		{Function: ToolFunction{Name: "GetStockPrice"}},
		{Function: ToolFunction{Name: "GetFollowedStocks"}},
		{Function: ToolFunction{Name: "GetTradingRecordList"}},
		{Function: ToolFunction{Name: "GetTradingRecordStatistics"}},
		{Function: ToolFunction{Name: "GetNews"}},
	}

	filtered := ExcludeTools(tools,
		"GetFollowedStocks",
		"GetTradingRecordList",
		"GetTradingRecordStatistics",
	)

	if len(filtered) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(filtered))
	}
	if filtered[0].Function.Name != "GetStockPrice" || filtered[1].Function.Name != "GetNews" {
		t.Fatalf("unexpected tools left: %+v", filtered)
	}
	if len(tools) != 5 {
		t.Fatalf("ExcludeTools must not mutate the input slice, len=%d", len(tools))
	}
}
