package mobilecore

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"aipanyi/backend/data"
	"aipanyi/backend/db"
	"gorm.io/gorm"
)

const (
	tradeScale     int64 = 10_000
	maxTradeMoney  int64 = 1<<62 - 1
	maxTradeShares int64 = 1<<53 - 1
)

var (
	errTradeBaseline  = errors.New("请先核对并保存期初持仓")
	errTradePrecision = errors.New("金额须为非负十进制数字，最多 4 位小数")
	errTradeOverflow  = errors.New("成交数量或金额超出记账范围")
	errTradeOversell  = errors.New("按成交时间重放后持仓不足，请核对成交顺序和数量")
)

type tradeBaselineInput struct {
	ExpectedRevision *int64    `json:"expectedRevision"`
	RequestID        string    `json:"requestId"`
	Code             string    `json:"code"`
	At               time.Time `json:"at"`
	Quantity         int64     `json:"quantity"`
	UnitCost         string    `json:"unitCost"`
}

type tradeInput struct {
	ExpectedRevision *int64    `json:"expectedRevision"`
	RequestID        string    `json:"requestId"`
	Code             string    `json:"code"`
	At               time.Time `json:"at"`
	Side             string    `json:"side"`
	Quantity         int64     `json:"quantity"`
	Price            string    `json:"price"`
	Fee              string    `json:"fee"`
	Note             string    `json:"note"`
	AlertEventID     int64     `json:"alertEventId"`
}

type tradeCorrection struct {
	ExpectedRevision *int64      `json:"expectedRevision"`
	RequestID        string      `json:"requestId"`
	Code             string      `json:"code"`
	TargetID         int64       `json:"targetId"`
	Replacement      *tradeInput `json:"replacement"`
}

type tradeEntry struct {
	ExpectedRevision *int64 `gorm:"-"`
	ID               int64  `gorm:"primaryKey"`
	CreatedAt        time.Time
	Code             string `gorm:"index:trade_symbol_baseline;size:20;not null"`
	RequestID        string `gorm:"uniqueIndex;size:80;not null"`
	BaselineID       int64  `gorm:"index:trade_symbol_baseline"`
	Kind             string
	TargetID         int64 `gorm:"index"`
	At               time.Time
	Side             string
	Quantity         int64
	Price            int64
	Fee              int64
	Note             string
	AlertEventID     int64
	Receipt          string
}

func (tradeEntry) TableName() string { return "mobile_trade_entry" }

// The form stages the confirmed payload before writing the ledger. It can then
// find and retry the same request even if Android replaces the loopback origin.
type tradeDraft struct {
	Code      string `gorm:"primaryKey;size:20"`
	RequestID string
	Kind      string
	Payload   string
}

func (tradeDraft) TableName() string { return "mobile_trade_draft" }

type tradeDraftDTO struct {
	Kind  string          `json:"kind"`
	Input json.RawMessage `json:"input"`
}

func tradeDraftEntry(d tradeDraftDTO) (tradeEntry, error) {
	switch d.Kind {
	case "baseline":
		var input tradeBaselineInput
		if err := json.Unmarshal(d.Input, &input); err != nil {
			return tradeEntry{}, err
		}
		return baselineEntry(input)
	case "fill":
		var input tradeInput
		if err := json.Unmarshal(d.Input, &input); err != nil {
			return tradeEntry{}, err
		}
		return fillEntry(input)
	case "correction":
		var input tradeCorrection
		if err := json.Unmarshal(d.Input, &input); err != nil {
			return tradeEntry{}, err
		}
		return correctionEntry(input)
	default:
		return tradeEntry{}, errors.New("无效的成交操作")
	}
}

func saveTradeDraft(input tradeDraftDTO) error {
	entry, err := tradeDraftEntry(input)
	if err != nil {
		return err
	}
	if entry.Code == "" || strings.TrimSpace(entry.RequestID) == "" || len(entry.RequestID) > 80 || len(input.Input) > 4096 {
		return errors.New("成交请求无效")
	}
	portfolioWriteMu.Lock()
	defer portfolioWriteMu.Unlock()
	return db.Dao.Transaction(func(tx *gorm.DB) error {
		var existing tradeDraft
		err := tx.First(&existing, "code = ?", entry.Code).Error
		if err == nil {
			prior, parseErr := tradeDraftEntry(tradeDraftDTO{Kind: existing.Kind, Input: json.RawMessage(existing.Payload)})
			if parseErr != nil || existing.RequestID != entry.RequestID || !sameTradeRequest(entry, prior) {
				return errors.New("有一笔提交待核对，请先恢复该请求")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&tradeDraft{Code: entry.Code, RequestID: entry.RequestID, Kind: input.Kind, Payload: string(input.Input)}).Error
	})
}

func getTradeDraft(code string) (*tradeDraftDTO, error) {
	var draft tradeDraft
	err := db.Dao.First(&draft, "code = ?", normalizeCode(code)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tradeDraftDTO{Kind: draft.Kind, Input: json.RawMessage(draft.Payload)}, nil
}

func acknowledgeTradeDraft(code, requestID string) error {
	return db.Dao.Where("code = ? AND request_id = ?", normalizeCode(code), requestID).Delete(&tradeDraft{}).Error
}

type tradePosition struct {
	Code          string `gorm:"primaryKey;size:20"`
	BaselineID    int64
	Revision      int64
	Quantity      int64
	BookValue     int64
	NetInvested   int64
	Realized      int64
	CycleRealized int64
	Pending       bool
	StrategyJSON  string
	EvaluatedAt   int64
}

func (tradePosition) TableName() string { return "mobile_trade_position" }

type tradeResult struct {
	ID          int64  `json:"id"`
	Revision    int64  `json:"revision"`
	Quantity    int64  `json:"quantity"`
	AverageCost string `json:"averageCost"`
	NetCost     string `json:"netCost"`
	NetInvested string `json:"netInvested"`
	Realized    string `json:"realized"`
}

type tradeDTO struct {
	ID                int64      `json:"id"`
	Kind              string     `json:"kind"`
	BaselineID        int64      `json:"baselineId"`
	TargetID          int64      `json:"targetId"`
	Side              string     `json:"side"`
	At                *time.Time `json:"at"`
	RecordedAt        time.Time  `json:"recordedAt"`
	Quantity          int64      `json:"quantity"`
	Price             string     `json:"price"`
	Fee               string     `json:"fee"`
	Note              string     `json:"note"`
	AlertEventID      int64      `json:"alertEventId"`
	CanCorrect        bool       `json:"canCorrect"`
	PastSegment       bool       `json:"pastSegment,omitempty"`
	CanAdjustBaseline bool       `json:"canAdjustBaseline,omitempty"`
	Superseded        bool       `json:"superseded"`
}

func parseTradeMoney(raw string) (int64, error) {
	whole, fraction, hasFraction := strings.Cut(strings.TrimSpace(raw), ".")
	if whole == "" || len(fraction) > 4 || (hasFraction && fraction == "") {
		return 0, errTradePrecision
	}
	for _, c := range whole + fraction {
		if c < '0' || c > '9' {
			return 0, errTradePrecision
		}
	}
	value, err := strconv.ParseInt(whole+fraction+strings.Repeat("0", 4-len(fraction)), 10, 64)
	if err != nil || value > maxTradeMoney {
		return 0, errTradeOverflow
	}
	return value, nil
}

func formatTradeMoney(value int64) string {
	if value < 0 {
		return "-" + formatTradeMoney(-value)
	}
	return fmt.Sprintf("%d.%04d", value/tradeScale, value%tradeScale)
}

func tradeProduct(price, shares int64) (int64, error) {
	if shares > 0 && price > maxTradeMoney/shares {
		return 0, errTradeOverflow
	}
	return price * shares, nil
}

// proportionalTradeMoney rounds to the nearest 1/10000, ties away from zero.
func proportionalTradeMoney(amount, part, whole int64) int64 {
	if whole == 0 {
		return 0
	}
	negative := amount < 0
	if negative {
		amount = -amount
	}
	n := new(big.Int).Mul(big.NewInt(amount), big.NewInt(part))
	n.Add(n, big.NewInt(whole/2))
	n.Quo(n, big.NewInt(whole))
	if negative {
		return -n.Int64()
	}
	return n.Int64()
}

func checkedTradeMoney(value int64) bool { return value >= -maxTradeMoney && value <= maxTradeMoney }

func (p tradePosition) result(id int64) tradeResult {
	return tradeResult{ID: id, Revision: p.Revision, Quantity: p.Quantity,
		AverageCost: formatTradeMoney(proportionalTradeMoney(p.BookValue, 1, p.Quantity)),
		NetCost:     formatTradeMoney(proportionalTradeMoney(p.NetInvested, 1, p.Quantity)),
		NetInvested: formatTradeMoney(p.NetInvested), Realized: formatTradeMoney(p.Realized)}
}

func validateTradeTime(at time.Time) error {
	if at.IsZero() || at.After(time.Now()) || at.Year() < 1970 {
		return errors.New("请输入有效时间，时间不能晚于当前时刻")
	}
	return nil
}

func baselineEntry(in tradeBaselineInput) (tradeEntry, error) {
	if err := validateTradeTime(in.At); err != nil {
		return tradeEntry{}, err
	}
	if in.Quantity < 0 || in.Quantity > maxTradeShares {
		return tradeEntry{}, errTradeOverflow
	}
	cost, err := parseTradeMoney(in.UnitCost)
	if err != nil {
		return tradeEntry{}, err
	}
	if in.Quantity == 0 && cost != 0 {
		return tradeEntry{}, errors.New("空仓期初的单位成本须为 0")
	}
	return tradeEntry{ExpectedRevision: in.ExpectedRevision, Code: normalizeCode(in.Code), RequestID: in.RequestID, Kind: "baseline", At: in.At.UTC(), Quantity: in.Quantity, Price: cost}, nil
}

func fillEntry(in tradeInput) (tradeEntry, error) {
	if err := validateTradeTime(in.At); err != nil {
		return tradeEntry{}, err
	}
	if in.Quantity <= 0 || in.Quantity > maxTradeShares || (in.Side != "buy" && in.Side != "sell") {
		return tradeEntry{}, errors.New("请选择买卖方向并输入正整数股数")
	}
	price, err := parseTradeMoney(in.Price)
	if err != nil {
		return tradeEntry{}, err
	}
	fee, err := parseTradeMoney(in.Fee)
	if err != nil {
		return tradeEntry{}, err
	}
	if price == 0 || len([]rune(in.Note)) > 200 || in.AlertEventID < 0 {
		return tradeEntry{}, errors.New("请检查成交价、备注和关联提醒")
	}
	return tradeEntry{ExpectedRevision: in.ExpectedRevision, Code: normalizeCode(in.Code), RequestID: in.RequestID, Kind: "fill", At: in.At.UTC(),
		Side: in.Side, Quantity: in.Quantity, Price: price, Fee: fee, Note: strings.TrimSpace(in.Note), AlertEventID: in.AlertEventID}, nil
}

func correctionEntry(in tradeCorrection) (tradeEntry, error) {
	code := normalizeCode(in.Code)
	entry := tradeEntry{ExpectedRevision: in.ExpectedRevision, Code: code, RequestID: in.RequestID, Kind: "void", TargetID: in.TargetID}
	if in.TargetID <= 0 {
		return entry, errors.New("无效的原成交记录")
	}
	if in.Replacement != nil {
		replacement := *in.Replacement
		replacement.Code, replacement.RequestID = code, in.RequestID
		fill, err := fillEntry(replacement)
		if err != nil {
			return entry, err
		}
		entry = fill
		entry.Kind, entry.TargetID, entry.ExpectedRevision = "replace", in.TargetID, in.ExpectedRevision
	}
	return entry, nil
}

func sameTradeRequest(a, b tradeEntry) bool {
	return a.Code == b.Code && a.Kind == b.Kind && a.TargetID == b.TargetID && a.At.Equal(b.At) &&
		a.Side == b.Side && a.Quantity == b.Quantity && a.Price == b.Price && a.Fee == b.Fee &&
		a.Note == b.Note && a.AlertEventID == b.AlertEventID
}

func setTradeBaseline(in tradeBaselineInput) (tradeResult, error) {
	entry, err := baselineEntry(in)
	if err != nil {
		return tradeResult{}, err
	}
	return applyTrade(entry, false)
}

func recordTrade(in tradeInput) (tradeResult, error) {
	entry, err := fillEntry(in)
	if err != nil {
		return tradeResult{}, err
	}
	return applyTrade(entry, false)
}

func correctTrade(in tradeCorrection) (tradeResult, error) {
	entry, err := correctionEntry(in)
	if err != nil {
		return tradeResult{}, err
	}
	return applyTrade(entry, false)
}

func applyTrade(entry tradeEntry, preview bool) (tradeResult, error) {
	if entry.Code == "" || strings.TrimSpace(entry.RequestID) == "" || len(entry.RequestID) > 80 {
		return tradeResult{}, errors.New("股票代码或请求 ID 无效")
	}
	portfolioWriteMu.Lock()
	defer portfolioWriteMu.Unlock()
	var result tradeResult
	err := db.Dao.Transaction(func(tx *gorm.DB) error {
		var existing tradeEntry
		err := tx.Where("request_id = ?", entry.RequestID).First(&existing).Error
		if err == nil {
			if !sameTradeRequest(entry, existing) {
				return errors.New("该请求 ID 已用于另一笔操作")
			}
			return json.Unmarshal([]byte(existing.Receipt), &result)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var followed data.FollowedStock
		if err := tx.Where("stock_code = ?", entry.Code).First(&followed).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errAlertNotFollowed
			}
			return err
		}
		if entry.AlertEventID != 0 {
			var count int64
			if err := tx.Model(&priceAlertEvent{}).Where("id = ? AND stock_code = ?", entry.AlertEventID, entry.Code).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("关联提醒不存在或属于另一只股票")
			}
		}
		cur, err := readTradePosition(tx, entry.Code)
		if err != nil {
			return err
		}
		if entry.ExpectedRevision != nil && *entry.ExpectedRevision != cur.Revision {
			return errors.New("持仓已更新，请重新预览")
		}
		if entry.Kind != "baseline" {
			entry.BaselineID = cur.BaselineID
		}
		if entry.TargetID != 0 {
			var original tradeEntry
			err := tx.Where("code = ? AND kind IN ?", entry.Code, []string{"fill", "replace"}).
				First(&original, entry.TargetID).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("原成交记录不存在")
			}
			if err != nil {
				return err
			}
			entry.BaselineID = original.BaselineID
		}
		next, err := calculateTrade(tx, entry, cur)
		if err != nil {
			return err
		}
		next.Revision, next.Pending = cur.Revision+1, true
		next.StrategyJSON, next.EvaluatedAt = "", 0
		if preview {
			result = next.result(0)
			return nil
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		if entry.Kind == "baseline" {
			next.BaselineID = entry.ID
		}
		if cur.Revision == 0 {
			if err := tx.Create(&next).Error; err != nil {
				return err
			}
		} else {
			updated := tx.Model(&tradePosition{}).Where("code = ? AND revision = ?", entry.Code, cur.Revision).Select(
				"baseline_id", "revision", "quantity", "book_value", "net_invested", "realized", "cycle_realized", "pending", "strategy_json", "evaluated_at").Updates(next)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return errors.New("持仓已变更，请刷新后重试")
			}
		}
		if err := writeFollowedPosition(tx, next); err != nil {
			return err
		}
		if entry.TargetID != 0 && entry.BaselineID != cur.BaselineID {
			// The current reconciliation still describes the same holdings after an archived correction.
			if err := tx.Model(&priceAlert{}).Where("stock_code = ? AND position_revision = ?", cur.Code, cur.Revision).
				UpdateColumn("position_revision", next.Revision).Error; err != nil {
				return err
			}
		} else if err := invalidatePositionAlerts(tx, next); err != nil {
			return err
		}
		result = next.result(entry.ID)
		receipt, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Model(&entry).Update("receipt", string(receipt)).Error
	})
	if err != nil {
		return tradeResult{}, err
	}
	return result, nil
}

func readTradePosition(tx *gorm.DB, code string) (tradePosition, error) {
	position := tradePosition{Code: code}
	err := tx.First(&position, "code = ?", code).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return position, err
}

func writeFollowedPosition(tx *gorm.DB, p tradePosition) error {
	average := float64(proportionalTradeMoney(p.BookValue, 1, p.Quantity)) / float64(tradeScale)
	result := tx.Model(&data.FollowedStock{}).Where("stock_code = ?", p.Code).
		Updates(map[string]any{"volume": p.Quantity, "cost_price": average})
	if result.Error == nil && result.RowsAffected == 0 {
		return errAlertNotFollowed
	}
	return result.Error
}

func invalidatePositionAlerts(tx *gorm.DB, position tradePosition) error {
	return tx.Model(&priceAlert{}).Where("stock_code = ? AND position_revision IS NOT NULL AND position_revision <> ?",
		position.Code, position.Revision).Updates(map[string]any{
		"enabled": false, "strategy_state": `{"Status":"position_changed"}`, "revision": gorm.Expr("revision + 1"),
	}).Error
}

func calculateTrade(tx *gorm.DB, entry tradeEntry, current tradePosition) (tradePosition, error) {
	if entry.Kind == "baseline" {
		if current.BaselineID > 0 {
			var latest tradeEntry
			err := tx.Where("code = ? AND kind IN ?", entry.Code, []string{"fill", "replace"}).
				Order("at DESC, id DESC").First(&latest).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return current, err
			}
			if err == nil && entry.At.Before(latest.At) {
				return current, errors.New("新期初不能早于上一段账本的最新成交")
			}
		}
		return replayTradePosition(entry, nil)
	}
	if entry.BaselineID == 0 {
		return current, errTradeBaseline
	}
	baseline, rows, err := tradeRowsForBaseline(tx, entry.Code, entry.BaselineID)
	if err != nil {
		return current, err
	}
	if entry.Kind != "void" && entry.At.Before(baseline.At) {
		return current, errors.New("成交时间早于期初。若这只股票尚无正式成交，可先调整对账期初")
	}
	var originalAt time.Time
	if entry.TargetID != 0 {
		found := false
		for _, row := range activeTrades(rows) {
			if row.ID == entry.TargetID {
				found = true
				originalAt = row.At
				break
			}
		}
		if !found {
			return current, errors.New("成交已修改或撤销，请选择最新记录")
		}
	}
	pastSegment := entry.BaselineID != current.BaselineID
	if pastSegment && entry.Kind != "void" {
		var nextBaseline tradeEntry
		if err := tx.Where("code = ? AND kind = ? AND id > ?", entry.Code, "baseline", entry.BaselineID).
			Order("id").First(&nextBaseline).Error; err != nil {
			return current, err
		}
		if !entry.At.Before(nextBaseline.At) && !entry.At.Equal(originalAt) {
			return current, errors.New("修改后的成交时间应早于下一段对账期初")
		}
	}
	// Preview and commit both put a new row last when execution times tie.
	entry.ID = 1<<63 - 1
	rows = append(rows, entry)
	next, err := replayTradePosition(baseline, activeTrades(rows))
	if pastSegment {
		// A subsequent reconciliation is an independent snapshot of actual holdings.
		return current, err
	}
	next.BaselineID = current.BaselineID
	return next, err
}

func tradeRowsForBaseline(tx *gorm.DB, code string, baselineID int64) (tradeEntry, []tradeEntry, error) {
	var baseline tradeEntry
	var rows []tradeEntry
	if err := tx.First(&baseline, baselineID).Error; err != nil {
		return baseline, nil, err
	}
	err := tx.Where("baseline_id = ? AND code = ?", baselineID, code).Order("id").Find(&rows).Error
	return baseline, rows, err
}

func activeTrades(rows []tradeEntry) []tradeEntry {
	superseded := make(map[int64]bool)
	byID := make(map[int64]tradeEntry, len(rows))
	order := make(map[int64]int64, len(rows))
	for _, row := range rows {
		order[row.ID] = row.ID
		if previous, ok := byID[row.TargetID]; ok && previous.At.Equal(row.At) {
			// A correction to price, quantity or fees keeps the original position among equal timestamps.
			order[row.ID] = order[previous.ID]
		}
		byID[row.ID] = row
		if row.TargetID != 0 {
			superseded[row.TargetID] = true
		}
	}
	out := make([]tradeEntry, 0, len(rows))
	for _, row := range rows {
		if row.Kind != "void" && !superseded[row.ID] {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].At.Equal(out[j].At) {
			return order[out[i].ID] < order[out[j].ID]
		}
		return out[i].At.Before(out[j].At)
	})
	return out
}

func replayTradePosition(baseline tradeEntry, fills []tradeEntry) (tradePosition, error) {
	book, err := tradeProduct(baseline.Price, baseline.Quantity)
	position := tradePosition{Code: baseline.Code, Quantity: baseline.Quantity, BookValue: book, NetInvested: book}
	if err != nil {
		return position, err
	}
	for _, fill := range fills {
		if _, err := position.applyFill(fill); err != nil {
			return position, err
		}
	}
	return position, nil
}

// applyFill returns the realized amount attributable to this fill.
func (p *tradePosition) applyFill(fill tradeEntry) (int64, error) {
	amount, err := tradeProduct(fill.Price, fill.Quantity)
	if err != nil {
		return 0, err
	}
	var realized int64
	if fill.Side == "buy" {
		if p.Quantity == 0 {
			p.CycleRealized = 0
		}
		gross := amount + fill.Fee
		if !checkedTradeMoney(gross) {
			return 0, errTradeOverflow
		}
		p.Quantity += fill.Quantity
		p.BookValue += gross
		p.NetInvested += gross
	} else {
		if fill.Quantity > p.Quantity {
			return 0, errTradeOversell
		}
		remaining := p.Quantity - fill.Quantity
		book := proportionalTradeMoney(p.BookValue, remaining, p.Quantity)
		proceeds := amount - fill.Fee
		realized = proceeds - (p.BookValue - book)
		if !checkedTradeMoney(realized) {
			return 0, errTradeOverflow
		}
		p.Quantity, p.BookValue = remaining, book
		p.NetInvested -= proceeds
		p.Realized += realized
		p.CycleRealized += realized
		if remaining == 0 {
			p.NetInvested = 0
		}
	}
	if p.Quantity > maxTradeShares || !checkedTradeMoney(p.BookValue) || !checkedTradeMoney(p.NetInvested) ||
		!checkedTradeMoney(p.Realized) || !checkedTradeMoney(p.CycleRealized) {
		return 0, errTradeOverflow
	}
	return realized, nil
}

func listTrades(code string, beforeID int64, limit int) ([]tradeDTO, error) {
	norm := normalizeCode(code)
	limit = min(max(limit, 1), 100)
	cur, err := readTradePosition(db.Dao, norm)
	if err != nil {
		return nil, err
	}
	var rows []tradeEntry
	query := db.Dao.Where("code = ?", norm)
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}
	if err := query.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	var canAdjustBaseline bool
	for _, row := range rows {
		if row.Kind == "baseline" && row.ID == cur.BaselineID {
			var count int64
			if err := db.Dao.Model(&tradeEntry{}).Where("code = ? AND kind IN ?", norm, []string{"fill", "replace"}).Count(&count).Error; err != nil {
				return nil, err
			}
			canAdjustBaseline = count == 0
			break
		}
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	var targets []int64
	if len(ids) > 0 {
		if err := db.Dao.Model(&tradeEntry{}).Where("code = ? AND target_id IN ?", norm, ids).Pluck("target_id", &targets).Error; err != nil {
			return nil, err
		}
	}
	superseded := make(map[int64]bool)
	for _, id := range targets {
		superseded[id] = true
	}
	result := make([]tradeDTO, 0, len(rows))
	for _, row := range rows {
		var at *time.Time
		if !row.At.IsZero() {
			at = &row.At
		}
		result = append(result, tradeDTO{ID: row.ID, Kind: row.Kind, BaselineID: row.BaselineID, TargetID: row.TargetID,
			Side: row.Side, At: at, RecordedAt: row.CreatedAt, Quantity: row.Quantity, Price: formatTradeMoney(row.Price),
			Fee: formatTradeMoney(row.Fee), Note: row.Note, AlertEventID: row.AlertEventID, Superseded: superseded[row.ID],
			CanCorrect:        (row.Kind == "fill" || row.Kind == "replace") && !superseded[row.ID],
			PastSegment:       row.BaselineID != 0 && row.BaselineID != cur.BaselineID,
			CanAdjustBaseline: row.Kind == "baseline" && row.ID == cur.BaselineID && canAdjustBaseline})
	}
	return result, nil
}
