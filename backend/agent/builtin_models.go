package agent

import "aipanyi/backend/modelinfo"

// GetBuiltinModelContextWindow 返回模型上下文窗口大小。
// 实际数据表已迁到 backend/modelinfo，避免移动端为了模型元数据引入整个 agent 包。
func GetBuiltinModelContextWindow(modelName string) int {
	return modelinfo.GetBuiltinModelContextWindow(modelName)
}

// GetBuiltinModelMaxOutput 返回模型单次生成的输出上限。
func GetBuiltinModelMaxOutput(modelName string) int {
	return modelinfo.GetBuiltinModelMaxOutput(modelName)
}
