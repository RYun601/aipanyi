package mobilecore

import "testing"

func TestGetAnalysisStatusIncludesCode(t *testing.T) {
	app := &mobileApp{
		analysisRunID:  "run-1",
		analysisCode:   "sh601127",
		analysisCancel: func() {},
	}
	status := app.GetAnalysisStatus()
	if status["running"] != true {
		t.Fatalf("running = %#v, want true", status["running"])
	}
	if status["runId"] != "run-1" {
		t.Fatalf("runId = %#v, want run-1", status["runId"])
	}
	if status["code"] != "sh601127" {
		t.Fatalf("code = %#v, want sh601127", status["code"])
	}

	app.finishAnalysis("run-1")
	status = app.GetAnalysisStatus()
	if status["running"] != false || status["runId"] != "" || status["code"] != "" {
		t.Fatalf("status after finish = %#v, want cleared", status)
	}
}
