package mobilecore

import (
	"encoding/json"
	"fmt"
	"strings"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"aipanyi/backend/logger"
	buildassets "aipanyi/build"
)

// seedStockBasics 首次启动时把内置 A 股基础表导入 SQLite。
//
// 桌面端的 initStockData 在根目录 main.go（package main），移动端无法复用；
// 这里用同一份 JSON 做一次批量导入，只在表为空时执行，避免每次启动都扫 5000 行。
func seedStockBasics() error {
	var count int64
	if err := db.Dao.Model(&data.StockBasic{}).Count(&count).Error; err != nil {
		return fmt.Errorf("统计股票基础表失败: %w", err)
	}
	if count > 0 {
		return nil
	}
	var res data.TushareStockBasicResponse
	if err := json.Unmarshal(buildassets.StockBasicJSON, &res); err != nil {
		return fmt.Errorf("解析内置股票基础表失败: %w", err)
	}
	fields := "ts_code,symbol,name,area,industry,cnspell,market,list_date,act_name,act_ent_type,fullname,exchange,list_status,curr_type,enname,delist_date,is_hs"
	fieldNames := strings.Split(fields, ",")
	fieldIdx := make(map[string]int, len(res.Data.Fields))
	for i, f := range res.Data.Fields {
		fieldIdx[f] = i
	}

	stocks := make([]data.StockBasic, 0, len(res.Data.Items))
	for _, item := range res.Data.Items {
		row := make(map[string]any, len(fieldNames))
		for _, field := range fieldNames {
			if idx, ok := fieldIdx[field]; ok && idx < len(item) {
				row[field] = item[idx]
			}
		}
		raw, err := json.Marshal(row)
		if err != nil {
			continue
		}
		var stock data.StockBasic
		if err := json.Unmarshal(raw, &stock); err != nil {
			continue
		}
		if stock.TsCode == "" || stock.Name == "" {
			continue
		}
		stock.ID = 0
		stocks = append(stocks, stock)
	}
	if len(stocks) == 0 {
		return fmt.Errorf("内置股票基础表为空")
	}
	if err := db.Dao.CreateInBatches(stocks, 500).Error; err != nil {
		return fmt.Errorf("写入股票基础表失败: %w", err)
	}
	logger.SugaredLogger.Infof("mobile: seeded %d stock basics", len(stocks))
	return nil
}
