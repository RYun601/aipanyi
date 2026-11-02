package mobilecore

import (
	"path/filepath"
	"strings"
	"testing"

	"aipanyi/backend/db"
	"aipanyi/backend/models"
)

func TestBuiltinAnalysisPrompts(t *testing.T) {
	prompts := builtinAnalysisPrompts()
	if len(prompts) != 3 {
		t.Fatalf("builtinAnalysisPrompts() len = %d, want 3", len(prompts))
	}
	if prompts[0].ID != analysisPromptDefault || prompts[0].Name != "默认（跟随设置）" {
		t.Fatalf("default prompt = %#v", prompts[0])
	}
	if prompts[1].ID != analysisPromptNeutral || prompts[1].Content != defaultAnalysisSystemPromptNeutral {
		t.Fatalf("neutral prompt = %#v", prompts[1])
	}
	if prompts[2].ID != analysisPromptPosition || prompts[2].Content != defaultAnalysisSystemPromptPosition {
		t.Fatalf("position prompt = %#v", prompts[2])
	}
}

func TestBuildAnalysisSystemPromptUsesSelectedBuiltin(t *testing.T) {
	repl := strings.NewReplacer()
	if got := buildAnalysisSystemPrompt(mobileSettings{}, repl, false, analysisPromptNeutral); got != defaultAnalysisSystemPromptNeutral {
		t.Fatalf("neutral prompt mismatch: %q", got)
	}
	if got := buildAnalysisSystemPrompt(mobileSettings{}, repl, true, analysisPromptPosition); got != defaultAnalysisSystemPromptPosition {
		t.Fatalf("position prompt mismatch: %q", got)
	}
}

func TestBuildAnalysisUserPromptUsesQuestionOverride(t *testing.T) {
	repl := strings.NewReplacer("{{stockName}}", "赛力斯")
	got := buildAnalysisUserPrompt(
		repl,
		"DATA",
		"",
		true,
		"",
		"结合 {{stockName}} 的持仓给出止损位",
		"设置里的旧问题",
	)
	if !strings.Contains(got, "结合 赛力斯 的持仓给出止损位") {
		t.Fatalf("override question missing: %q", got)
	}
	if strings.Contains(got, "设置里的旧问题") {
		t.Fatalf("setting question leaked into override: %q", got)
	}
}

func TestBuildAnalysisUserPromptFallsBackToSettingQuestion(t *testing.T) {
	repl := strings.NewReplacer("{{stockName}}", "赛力斯")
	got := buildAnalysisUserPrompt(repl, "DATA", "", true, "", "", "分析 {{stockName}} 的趋势")
	if !strings.Contains(got, "分析 赛力斯 的趋势") {
		t.Fatalf("setting question missing: %q", got)
	}
}

func TestBuildAnalysisUserPromptUsesDefaultWhenQuestionEmpty(t *testing.T) {
	repl := strings.NewReplacer("{{stockName}}", "赛力斯", "{{stockCode}}", "sh601127")
	got := buildAnalysisUserPrompt(repl, "DATA", "", false, "", "", "")
	if !strings.Contains(got, "请基于当前价和近 30 日 K 线分析 赛力斯") {
		t.Fatalf("default question missing: %q", got)
	}
}

func TestListAnalysisPromptsIncludesSavedTemplates(t *testing.T) {
	dir := t.TempDir()
	db.InitWithOptions(filepath.Join(dir, "prompt.db"), db.MobileOptions())
	if sqlDB, err := db.Dao.DB(); err == nil {
		defer sqlDB.Close()
	}
	if err := db.Dao.AutoMigrate(&models.PromptTemplate{}); err != nil {
		t.Fatal(err)
	}
	created := &models.PromptTemplate{
		Name:    "今日策略",
		Content: "根据最新数据以及持仓数据分析",
		Type:    "stock",
	}
	if err := db.Dao.Create(created).Error; err != nil {
		t.Fatal(err)
	}

	prompts := listAnalysisPrompts()
	if len(prompts) != 4 {
		t.Fatalf("listAnalysisPrompts() len = %d, want 4", len(prompts))
	}
	last := prompts[len(prompts)-1]
	if last.Name != "今日策略" || !strings.Contains(last.Content, "持仓数据") {
		t.Fatalf("saved prompt = %#v", last)
	}

	// 本次选择必须优先于设置里的默认提示词。
	settings := mobileSettings{DefaultPromptID: 999}
	got := buildAnalysisSystemPrompt(settings, strings.NewReplacer(), false, int64(created.ID))
	if !strings.Contains(got, "持仓数据分析") {
		t.Fatalf("selected saved prompt not used: %q", got)
	}

	// 设置里的默认提示词如果选了内置项（负数 ID），也必须生效。
	neutral := buildAnalysisSystemPrompt(
		mobileSettings{DefaultPromptID: analysisPromptNeutral},
		strings.NewReplacer(), true, 0,
	)
	if neutral != defaultAnalysisSystemPromptNeutral {
		t.Fatalf("builtin neutral default not used: %q", neutral)
	}
	position := buildAnalysisSystemPrompt(
		mobileSettings{DefaultPromptID: analysisPromptPosition},
		strings.NewReplacer(), false, 0,
	)
	if position != defaultAnalysisSystemPromptPosition {
		t.Fatalf("builtin position default not used: %q", position)
	}
}
