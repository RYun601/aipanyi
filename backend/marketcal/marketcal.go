// Package marketcal 提供 A 股 / 港股 / 美股的交易时段与交易日判断。
//
// 这些逻辑原先散落在根目录 app.go（package main）中，桌面端与 Android 端
// 都需要同一份实现，因此抽到这里。桌面端在 app.go 中保留同名薄包装，
// 行为与抽取前完全一致。
package marketcal

import (
	"fmt"
	"sort"
	"strings"
	"time"
	// 让 marketcal 自己保证 IANA 数据可用：Android ROM 的 zoneinfo 可能不完整，
	// 而依赖 mobile/core 的空白导入存在初始化顺序不确定性。
	_ "time/tzdata"

	"github.com/coocood/freecache"

	"aipanyi/backend/data"
)

// shanghai 北京时间固定偏移。Android 上不保证 /usr/share/zoneinfo 一定可用，
// 固定 +08:00 比 LoadLocation 更稳（与桌面端 ShanghaiTimezone 保持一致）。
var shanghai = time.FixedZone("CST", 8*60*60)

var tradingDayCache = freecache.NewCache(64 * 1024)

// IsTradingDay 判断 A 股是否为交易日。
func IsTradingDay(date time.Time) bool {
	date = date.In(shanghai)
	weekday := date.Weekday()
	dateStr := date.Format("2006-01-02")

	cacheKey := []byte(dateStr)
	if cached, err := tradingDayCache.Get(cacheKey); err == nil {
		return string(cached) == "1"
	}

	if weekday == time.Saturday || weekday == time.Sunday {
		_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
		return false
	}

	isHoliday, apiOk := checkHolidayAPI(dateStr)
	if apiOk {
		if isHoliday {
			_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
			return false
		}
		_ = tradingDayCache.Set(cacheKey, []byte("1"), 86400)
		return true
	}

	_ = tradingDayCache.Set(cacheKey, []byte("1"), 600)
	return true
}

func checkHolidayAPI(date string) (isHoliday bool, apiOk bool) {
	type holidayResp struct {
		Code    int `json:"code"`
		Holiday struct {
			Holiday bool   `json:"holiday"`
			Name    string `json:"name"`
		} `json:"holiday"`
	}
	var result holidayResp
	resp, err := data.SharedHTTPClient.R().SetResult(&result).Get(fmt.Sprintf("https://timor.tech/api/holiday/info/%s", date))
	if err != nil || resp.StatusCode() != 200 {
		return false, false
	}
	if result.Code == 0 && result.Holiday.Holiday {
		return true, true
	}
	return false, true
}

// IsTradingTime 判断当前是否在 A 股交易时段（含 9:15 集合竞价）。
func IsTradingTime(date time.Time) bool {
	date = date.In(shanghai)
	if !IsTradingDay(date) {
		return false
	}

	hour, minute, _ := date.Clock()

	if (hour == 9 && minute >= 15) || (hour == 10) || (hour == 11 && minute <= 30) {
		return true
	}
	if (hour == 13) || (hour == 14) || (hour == 15 && minute <= 0) {
		return true
	}
	return false
}

// IsHKTradingTime 判断当前是否在港股交易时段。
func IsHKTradingTime(date time.Time) bool {
	date = date.In(shanghai)
	if !isHKTradingDay(date) {
		return false
	}

	hour, minute, _ := date.Clock()

	if (hour == 9 && minute >= 0) || (hour == 9 && minute <= 30) {
		return true
	}
	if (hour == 9 && minute > 30) || (hour >= 10 && hour < 12) || (hour == 12 && minute == 0) {
		return true
	}
	if (hour == 13 && minute >= 0) || (hour >= 14 && hour < 16) || (hour == 16 && minute == 0) {
		return true
	}
	if (hour == 16 && minute >= 0) || (hour == 16 && minute <= 10) {
		return true
	}
	return false
}

func isHKTradingDay(date time.Time) bool {
	date = date.In(shanghai)
	weekday := date.Weekday()
	dateStr := date.Format("2006-01-02")

	cacheKey := []byte("hk:" + dateStr)
	if cached, err := tradingDayCache.Get(cacheKey); err == nil {
		return string(cached) == "1"
	}

	if weekday == time.Saturday || weekday == time.Sunday {
		_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
		return false
	}

	isHoliday, apiOk := checkHKHolidayAPI(dateStr)
	if apiOk {
		if isHoliday {
			_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
			return false
		}
		_ = tradingDayCache.Set(cacheKey, []byte("1"), 86400)
		return true
	}

	_ = tradingDayCache.Set(cacheKey, []byte("1"), 600)
	return true
}

func checkHKHolidayAPI(date string) (isHoliday bool, apiOk bool) {
	type klineResp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	var result klineResp
	dateClean := strings.ReplaceAll(date, "-", "")
	apiURL := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=100.HSI&fields1=f1&fields2=f51&klt=101&fqt=0&beg=%s&end=%s", dateClean, dateClean)
	resp, err := data.SharedHTTPClient.R().SetResult(&result).Get(apiURL)
	if err != nil || resp.StatusCode() != 200 {
		return false, false
	}
	if len(result.Data.Klines) > 0 {
		return false, true
	}
	return true, true
}

// IsUSTradingTime 判断当前是否在美股交易时段（盘前/盘中/盘后，按纽约时间）。
func IsUSTradingTime(date time.Time) bool {
	est, err := time.LoadLocation("America/New_York")
	var estTime time.Time
	if err != nil {
		estTime = date.Add(time.Hour * -12)
	} else {
		estTime = date.In(est)
	}

	if !isUSTradingDay(estTime) {
		return false
	}

	hour, minute, _ := estTime.Clock()

	if (hour == 4) || (hour == 5) || (hour == 6) || (hour == 7) || (hour == 8) || (hour == 9 && minute < 30) {
		return true
	}
	if (hour == 9 && minute >= 30) || (hour >= 10 && hour < 16) || (hour == 16 && minute == 0) {
		return true
	}
	if (hour == 16 && minute > 0) || (hour >= 17 && hour < 20) || (hour == 20 && minute == 0) {
		return true
	}
	return false
}

func isUSTradingDay(estTime time.Time) bool {
	weekday := estTime.Weekday()
	dateStr := estTime.Format("2006-01-02")

	cacheKey := []byte("us:" + dateStr)
	if cached, err := tradingDayCache.Get(cacheKey); err == nil {
		return string(cached) == "1"
	}

	if weekday == time.Saturday || weekday == time.Sunday {
		_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
		return false
	}

	isHoliday, apiOk := checkUSHolidayAPI(dateStr)
	if apiOk {
		if isHoliday {
			_ = tradingDayCache.Set(cacheKey, []byte("0"), 86400)
			return false
		}
		_ = tradingDayCache.Set(cacheKey, []byte("1"), 86400)
		return true
	}

	_ = tradingDayCache.Set(cacheKey, []byte("1"), 600)
	return true
}

func checkUSHolidayAPI(date string) (isHoliday bool, apiOk bool) {
	type usHolidayResp struct {
		IsHoliday    bool   `json:"is_holiday"`
		IsEarlyClose bool   `json:"is_early_close"`
		IsWeekend    bool   `json:"is_weekend"`
		Status       string `json:"status"`
	}
	var result usHolidayResp
	apiURL := fmt.Sprintf("https://fincalapi.com/v1/day_status?calendar=NYSE&date=%s", date)
	resp, err := data.SharedHTTPClient.R().SetResult(&result).Get(apiURL)
	if err != nil || resp.StatusCode() != 200 {
		return false, false
	}
	return result.IsHoliday, true
}

// PreCache 预热前后 days 天的交易日缓存，避免盘中首次判断时同步请求节假日接口。
func PreCache(days int) {
	now := time.Now().In(shanghai)
	go func() {
		for i := -days; i <= days; i++ {
			IsTradingDay(now.AddDate(0, 0, i))
		}
	}()
	go func() {
		for i := -days; i <= days; i++ {
			isHKTradingDay(now.AddDate(0, 0, i))
		}
	}()
	go func() {
		est, _ := time.LoadLocation("America/New_York")
		for i := -days; i <= days; i++ {
			d := now.AddDate(0, 0, i)
			if est != nil {
				d = d.In(est)
			}
			isUSTradingDay(d)
		}
	}()
}

// IsTradingTimeForMarket 按市场代码返回对应交易时段判断。
// market 取 A / HK / US；未知市场按 A 股处理。
// IsTradingTimeForMarket 判断某市场在指定时间是否处于交易时段。
// market 取 A / HK / US，未知市场按 A 股处理。
func IsTradingTimeForMarket(market string, date time.Time) bool {
	switch market {
	case "HK":
		return IsHKTradingTime(date)
	case "US":
		return IsUSTradingTime(date)
	default:
		return IsTradingTime(date)
	}
}

// MonitorWindow 一个连续的监控窗口。
type MonitorWindow struct {
	Start time.Time
	End   time.Time
}

var newYork = func() *time.Location {
	if loc, err := time.LoadLocation("America/New_York"); err == nil {
		return loc
	}
	return time.FixedZone("EST", -5*60*60)
}()

// IsHKTradingDay 导出港股交易日判断，供监控窗口计算使用。
func IsHKTradingDay(date time.Time) bool { return isHKTradingDay(date) }

// IsUSTradingDay 导出美股交易日判断，内部先转纽约时间。
func IsUSTradingDay(date time.Time) bool {
	return isUSTradingDay(date.In(newYork))
}

func nextSession(
	now time.Time,
	loc *time.Location,
	startHour, startMin, endHour, endMin int,
	isTradingDay func(time.Time) bool,
) (time.Time, time.Time, bool) {
	now = now.In(loc)
	for i := 0; i <= 10; i++ {
		day := now.AddDate(0, 0, i)
		y, m, d := day.Date()
		start := time.Date(y, m, d, startHour, startMin, 0, 0, loc)
		end := time.Date(y, m, d, endHour, endMin, 0, 0, loc)
		if !end.After(now) {
			continue
		}
		if !isTradingDay(day) {
			continue
		}
		return start, end, true
	}
	return time.Time{}, time.Time{}, false
}

// NextTradingSession 返回某市场当前或下一个交易窗口。
func NextTradingSession(market string, now time.Time) (time.Time, time.Time, bool) {
	switch market {
	case "HK":
		return nextSession(now, shanghai, 9, 0, 16, 10, IsHKTradingDay)
	case "US":
		return nextSession(now, newYork, 4, 0, 20, 0, IsUSTradingDay)
	default:
		return nextSession(now, shanghai, 9, 15, 15, 0, IsTradingDay)
	}
}

func mergeMonitorWindows(in []MonitorWindow) []MonitorWindow {
	if len(in) == 0 {
		return nil
	}
	windows := append([]MonitorWindow(nil), in...)
	sort.Slice(windows, func(i, j int) bool {
		return windows[i].Start.Before(windows[j].Start)
	})
	out := []MonitorWindow{windows[0]}
	for _, w := range windows[1:] {
		last := &out[len(out)-1]
		if !w.Start.After(last.End) {
			if w.End.After(last.End) {
				last.End = w.End
			}
			continue
		}
		out = append(out, w)
	}
	return out
}

// NextMonitorWindow 返回包含 now 的窗口；都不包含时返回最早开始的未来窗口。
func NextMonitorWindow(markets []string, now time.Time) (MonitorWindow, bool) {
	windows := make([]MonitorWindow, 0, len(markets))
	seen := make(map[string]bool, len(markets))
	for _, market := range markets {
		market = strings.ToUpper(strings.TrimSpace(market))
		if market == "" || seen[market] {
			continue
		}
		seen[market] = true
		start, end, ok := NextTradingSession(market, now)
		if ok {
			windows = append(windows, MonitorWindow{Start: start, End: end})
		}
	}
	if len(windows) == 0 {
		return MonitorWindow{}, false
	}
	merged := mergeMonitorWindows(windows)
	for _, w := range merged {
		if !w.Start.After(now) && !w.End.Before(now) {
			return w, true
		}
	}
	for _, w := range merged {
		if w.Start.After(now) {
			return w, true
		}
	}
	return MonitorWindow{}, false
}

// NextMonitorWindowAfter 返回 after 之后的下一个窗口；after 在窗口内时从该窗口结束处继续找。
func NextMonitorWindowAfter(markets []string, after time.Time) (MonitorWindow, bool) {
	w, ok := NextMonitorWindow(markets, after)
	if !ok {
		return MonitorWindow{}, false
	}
	if w.Start.After(after) {
		return w, true
	}
	return NextMonitorWindow(markets, w.End)
}
