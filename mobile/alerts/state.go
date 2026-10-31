// Package alerts 是价格提醒的纯逻辑：状态机、方向归一化、AI 建议解析。
//
// 刻意不依赖数据库/网络/时间，方便在 Windows 本地与 CI 里直接跑单测；
// 真正的轮询、落库、通知在 mobile/core 里。
package alerts

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Suggestion 是 AI 分析提取出的一条建议（已做基础校验）。
type Suggestion struct {
	Code      string
	Name      string
	Direction string
	Price     float64
	Note      string
	Valid     bool
	Reason    string
}

// NormalizeDirection 把模型可能返回的中英文方向归一化为 above/below。
func NormalizeDirection(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch {
	case v == "above", strings.Contains(v, "涨"), strings.Contains(v, "突破"), strings.Contains(v, "高于"), strings.Contains(v, "up"):
		return "above"
	case v == "below", strings.Contains(v, "跌"), strings.Contains(v, "低于"), strings.Contains(v, "down"):
		return "below"
	default:
		return ""
	}
}

// NormalizeState 把数据库里的状态归一化为 above/below/unknown。
func NormalizeState(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "above" || v == "below" {
		return v
	}
	return "unknown"
}

// Evaluate 单条规则的状态机。
//
// 返回值：
//   - nextState：本次检查后应该落库的状态；
//   - trigger：本次是否应该发通知。
//
// 语义：
//   - above 规则：价格从下方上穿阈值触发一次，回落到下方后重新武装；
//   - below 规则：价格从上方下穿阈值触发一次，回升到上方后重新武装；
//   - unknown（刚创建/刚启用/重启后首次拿到行情）只初始化状态，不通知。
func Evaluate(direction string, threshold float64, state string, price float64) (nextState string, trigger bool) {
	state = NormalizeState(state)
	direction = NormalizeDirection(direction)
	if threshold <= 0 || price <= 0 {
		return state, false
	}

	switch direction {
	case "above":
		if price < threshold {
			return "below", false
		}
		switch state {
		case "above":
			return "above", false
		case "below":
			return "above", true
		default:
			return "above", false
		}
	case "below":
		if price > threshold {
			return "above", false
		}
		switch state {
		case "below":
			return "below", false
		case "above":
			return "below", true
		default:
			return "below", false
		}
	default:
		return state, false
	}
}

// ParseSuggestions 解析并校验模型返回的 JSON。
//
// 校验规则：
//   - 方向必须是 above/below（支持中文“涨到/跌破”）；
//   - 价格必须 > 0；
//   - 已知现价时，价格必须在现价的 0.5~2 倍之间（防止模型幻觉出 10 倍价格）；
//   - 相同方向+价格去重（保留第一条）；
//   - 非法条目不丢弃，标 Valid=false + Reason，前端可以灰显或直接过滤。
func ParseSuggestions(content, code, name string, currentPrice float64) ([]Suggestion, error) {
	payload := StripJSONFence(content)
	var env struct {
		Alerts []struct {
			Direction any    `json:"direction"`
			Price     any    `json:"price"`
			Note      string `json:"note"`
		} `json:"alerts"`
	}
	if err := json.Unmarshal([]byte(payload), &env); err != nil {
		return nil, fmt.Errorf("AI 返回格式无法解析: %w", err)
	}
	out := make([]Suggestion, 0, len(env.Alerts))
	seen := make(map[string]struct{}, len(env.Alerts))
	for _, raw := range env.Alerts {
		sug := Suggestion{Code: code, Name: name, Valid: false}
		dir := NormalizeDirection(fmt.Sprint(raw.Direction))
		price, priceOK := ToFloat(raw.Price)
		switch {
		case dir == "":
			sug.Reason = "方向不明确"
		case !priceOK || price <= 0:
			sug.Reason = "价格无效"
		case currentPrice > 0 && (price < currentPrice*0.5 || price > currentPrice*2):
			sug.Reason = "价格偏离现价过大"
		default:
			sug.Valid = true
		}
		sug.Direction = dir
		sug.Price = math.Round(price*100) / 100
		sug.Note = strings.TrimSpace(raw.Note)
		if len([]rune(sug.Note)) > 100 {
			sug.Note = string([]rune(sug.Note)[:100])
		}
		if sug.Note == "" {
			sug.Note = "AI 建议"
		}
		if sug.Valid {
			key := fmt.Sprintf("%s:%.2f", sug.Direction, sug.Price)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		out = append(out, sug)
	}
	return out, nil
}

// StripJSONFence 去掉模型偶尔包上的 ```json 代码块，并截取最外层 JSON。
func StripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

// ToFloat 兼容模型返回数字或字符串两种价格格式。
func ToFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}
