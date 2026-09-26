package main

// app_file_bridge.go — 文件对话框的 Web 模式桥接
//
// 背景：Wails 的 runtime.OpenFileDialog / SaveFileDialog 在浏览器中不存在。
// 解决思路是把「弹窗选择」与「业务逻辑」分离：
//   - 桌面模式：仍走原有 *Dialog 方法，行为不变；
//   - Web 模式：前端用 <input type=file> + /api/upload 取得服务端路径，
//     或用本文件的 *Bytes / *Data 方法直接拿到内容，由前端触发浏览器下载。
//
// 本文件只提供"取数据"的纯方法，不触碰任何对话框 API，两种模式都可安全调用。

import (
	"encoding/base64"
	"fmt"

	"aipanyi/backend/data"
	"aipanyi/backend/logger"
)

// ExportConfigData 返回配置导出的 JSON 文本（不含文件对话框）。
func (a *App) ExportConfigData() string {
	return data.NewSettingsApi().Export()
}

// MarkdownContentForSave 返回 AI 分析结果的 Markdown 内容与建议文件名。
// 第二个返回值为空表示没有可保存的内容。
func (a *App) MarkdownContentForSave(stockCode, stockName string) (string, string) {
	res := data.NewDeepSeekOpenAi(a.ctx, 0).GetAIResponseResult(stockCode)
	if res == nil || len(res.Content) <= 100 {
		return "", ""
	}
	analysisTime := res.CreatedAt.Format("2006-01-02_15_04_05")
	filename := fmt.Sprintf("%s[%s]AI分析结果_%s.md", stockName, stockCode, analysisTime)
	return filename, res.Content
}

// TradingRecordTemplateBytes 返回交易记录导入模板的文件名与 base64 内容。
func (a *App) TradingRecordTemplateBytes() (string, string, error) {
	xlsxData, err := data.NewStockDataApi().TradingRecordTemplateXLSX()
	if err != nil {
		return "", "", err
	}
	return "交易记录导入模板.xlsx", base64.StdEncoding.EncodeToString(xlsxData), nil
}

// BuildTableXLSXBytes 返回表格导出的文件名与 base64 内容（前端表格导出用）。
func (a *App) BuildTableXLSXBytes(defaultFileName string, table data.ExportTableData) (string, string, error) {
	if defaultFileName == "" {
		defaultFileName = "导出数据.xlsx"
	}
	xlsxData, err := data.NewStockDataApi().BuildTableXLSX(table)
	if err != nil {
		return "", "", err
	}
	return defaultFileName, base64.StdEncoding.EncodeToString(xlsxData), nil
}

// ImportTradingRecordsFromPath 导入已上传的成交记录文件（Web 模式入口）。
func (a *App) ImportTradingRecordsFromPath(filePath string) (*data.TradingRecordImportResult, error) {
	if filePath == "" {
		return nil, nil
	}
	return data.NewStockDataApi().ImportTradingRecords(filePath)
}

// ImportSkillPackageFromPath 导入已上传的技能包 ZIP（Web 模式入口）。
func (a *App) ImportSkillPackageFromPath(zipPath string) string {
	if zipPath == "" {
		return "未选择文件"
	}
	logger.SugaredLogger.Infof("ImportSkillPackageFromPath: %s", zipPath)
	return importSkillPackageFromZip(zipPath)
}
