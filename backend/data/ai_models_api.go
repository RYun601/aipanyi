package data

import (
	"strings"

	"aipanyi/backend/logger"
	"aipanyi/backend/modelinfo"
)

// AiModelInfo 模型能力信息。MaxTokens 是输出上限，ContextWindow 是输入+输出总容量。
type AiModelInfo struct {
	ModelName     string `json:"modelName"`
	MaxTokens     int    `json:"maxTokens"`
	ContextWindow int    `json:"contextWindow"`
	Source        string `json:"source"` // api / builtin
}

type aiModelListItem struct {
	ID string `json:"id"`
}

// FetchAiModels 请求 OpenAI 兼容的 /models 接口，返回去重后的模型 ID。
// 失败返回空切片，不抛异常，方便前端保留手动输入。
func FetchAiModels(baseUrl, apiKey, extraHeaders string) []string {
	baseUrl = strings.TrimSpace(baseUrl)
	apiKey = strings.TrimSpace(apiKey)
	if baseUrl == "" || apiKey == "" {
		return []string{}
	}

	var respData struct {
		Data []aiModelListItem `json:"data"`
	}
	req := SharedHTTPClient.R().
		SetHeader("Authorization", "Bearer "+apiKey).
		SetHeader("Content-Type", "application/json").
		SetResult(&respData)
	if extra := BuildExtraHeaders(extraHeaders, ""); len(extra) > 0 {
		req = req.SetHeaders(extra)
	}

	resp, err := req.Get(strings.TrimRight(baseUrl, "/") + "/models")
	if err != nil {
		logger.SugaredLogger.Errorf("FetchAiModels error: %v", err)
		return []string{}
	}
	if resp.IsError() {
		logger.SugaredLogger.Errorf("FetchAiModels http error: %s", resp.Status())
		return []string{}
	}

	out := make([]string, 0, len(respData.Data))
	seen := make(map[string]bool, len(respData.Data))
	for _, item := range respData.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// FetchAiModelInfo 先请求 /models/{modelName}，再用内置模型表兜底。
func FetchAiModelInfo(baseUrl, apiKey, modelName, extraHeaders string) *AiModelInfo {
	baseUrl = strings.TrimSpace(baseUrl)
	modelName = strings.TrimSpace(modelName)
	if baseUrl == "" || modelName == "" {
		return nil
	}

	info := &AiModelInfo{ModelName: modelName}
	if apiKey != "" {
		type modelDetail struct {
			ID             string `json:"id"`
			ContextWindow  int    `json:"context_window"`
			MaxContextLen  int    `json:"max_context_length"`
			ContextLength  int    `json:"context_length"`
			MaxOutputTok   int    `json:"max_output_tokens"`
			MaxTokensField int    `json:"max_tokens"`
		}
		var detail modelDetail

		req := SharedHTTPClient.R().
			SetHeader("Authorization", "Bearer "+apiKey).
			SetHeader("Content-Type", "application/json").
			SetResult(&detail)
		if extra := BuildExtraHeaders(extraHeaders, ""); len(extra) > 0 {
			req = req.SetHeaders(extra)
		}

		resp, err := req.Get(strings.TrimRight(baseUrl, "/") + "/models/" + modelName)
		if err == nil && !resp.IsError() && detail.ID != "" {
			if detail.ContextWindow > 0 {
				info.ContextWindow = detail.ContextWindow
				info.Source = "api"
			} else if detail.MaxContextLen > 0 {
				info.ContextWindow = detail.MaxContextLen
				info.Source = "api"
			} else if detail.ContextLength > 0 {
				info.ContextWindow = detail.ContextLength
				info.Source = "api"
			}
			if detail.MaxOutputTok > 0 {
				info.MaxTokens = detail.MaxOutputTok
				info.Source = "api"
			} else if detail.MaxTokensField > 0 {
				info.MaxTokens = detail.MaxTokensField
				info.Source = "api"
			}
		}
	}

	if info.ContextWindow == 0 {
		if cw := modelinfo.GetBuiltinModelContextWindow(modelName); cw > 0 {
			info.ContextWindow = cw
			if info.Source == "" {
				info.Source = "builtin"
			}
		}
	}
	if info.MaxTokens == 0 {
		if mo := modelinfo.GetBuiltinModelMaxOutput(modelName); mo > 0 {
			info.MaxTokens = mo
			if info.Source == "" {
				info.Source = "builtin"
			}
		}
	}
	return info
}
