package mobilecore

import (
	"math"
	"strings"
	"unicode"

	"aipanyi/backend/data"
	"aipanyi/backend/modelinfo"
)

const (
	// 未显式配置 MaxTokens 时，移动端以 128K 为输出目标。
	// 仍然会受模型输出上限和上下文窗口约束，不会盲目发给所有模型。
	defaultOutputTargetTokens = 128000
	// 模型/配置都没给出上下文窗口时的兜底值。
	fallbackContextWindowTokens = 128000
	// 给输入、系统消息和协议开销预留的余量。
	outputReserveMarginTokens = 2048
	// 极端小上下文下仍给模型留一点正文空间，避免出现 0 或负数。
	minOutputTokens = 1024
	// 当前主流模型的最大输出上限（DeepSeek V4 系列为 393216）。
	maxOutputTokenCeiling = 393216
)

// estimatePromptTokens 估算输入 token 数。
// 中文按 1.3 字/token，英文/符号按 4 字符/token，与桌面端估算口径保持一致。
func estimatePromptTokens(text string) int {
	if text == "" {
		return 0
	}
	chinese := 0
	other := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			chinese++
		} else {
			other++
		}
	}
	return int(math.Ceil(float64(chinese)/1.3)) + (other+3)/4
}

// resolveOutputTokenBudget 解析本次请求应发送的 max_tokens。
//
// 优先级：
//  1. 用户显式配置的 MaxTokens（旧配置里的 4096 也算显式，尊重用户设置）；
//  2. 模型元数据/内置表中的输出上限；
//  3. 128K 移动端默认目标。
//
// 未显式配置时，即使模型支持 384K，也只以 128K 为目标；用户想用满模型上限，
// 可以在 AI 配置里显式填写。最后按“上下文窗口 - 输入估算 - 2K 余量”收紧，
// 确保 input + max_tokens 不超过模型上下文。
func resolveOutputTokenBudget(cfg data.AIConfig, prompts ...string) int {
	contextWindow := cfg.ContextWindow
	if contextWindow <= 0 {
		contextWindow = modelinfo.GetBuiltinModelContextWindow(cfg.ModelName)
	}
	if contextWindow <= 0 {
		contextWindow = fallbackContextWindowTokens
	}

	output := cfg.MaxTokens
	explicit := output > 0
	if output <= 0 {
		output = modelinfo.GetBuiltinModelMaxOutput(cfg.ModelName)
	}
	if output <= 0 {
		output = defaultOutputTargetTokens
	}
	if !explicit && output > defaultOutputTargetTokens {
		output = defaultOutputTargetTokens
	}
	if output > maxOutputTokenCeiling {
		output = maxOutputTokenCeiling
	}

	inputTokens := 0
	for _, prompt := range prompts {
		inputTokens += estimatePromptTokens(prompt)
	}
	if limit := contextWindow - inputTokens - outputReserveMarginTokens; limit < output {
		output = limit
	}
	if output < minOutputTokens {
		output = minOutputTokens
	}
	return output
}

// shouldRetryMaxTokens 判断 400 是否明确由 max_tokens 超限引起。
// 只对这类错误降级重试，避免把鉴权、模型不存在等问题也重试一遍。
func shouldRetryMaxTokens(status int, message string) bool {
	if status != 400 {
		return false
	}
	lower := strings.ToLower(message)
	for _, key := range []string{
		"max_tokens",
		"max_completion_tokens",
		"max_output_tokens",
		"maximum output tokens",
		"max output tokens",
	} {
		if strings.Contains(lower, key) {
			return true
		}
	}
	return false
}

// nextMaxTokensAfterLimit 返回下一次重试的 max_tokens；0 表示不再重试。
// 128K → 64K → 32K → 8K，覆盖大多数 OpenAI 兼容服务的上限。
func nextMaxTokensAfterLimit(current int) int {
	switch {
	case current > 64000:
		return 64000
	case current > 32000:
		return 32000
	case current > 8192:
		return 8192
	default:
		return 0
	}
}
