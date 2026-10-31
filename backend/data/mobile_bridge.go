package data

// 本文件只放移动端（gomobile）需要复用的薄包装，避免把内部实现细节
// 直接暴露成公共 API，也避免移动端复制一份会漂移的逻辑。

// NormalizeStockCode 把各种股票代码格式归一化为 followed_stock 表使用的前缀小写格式。
// 移动端与桌面端共用同一份归一化规则。
func NormalizeStockCode(stockCode string) string {
	return normalizeStockCode(stockCode)
}

// OpenAIChatEndpoint 把用户填写的 BaseUrl 归一化为（服务地址, /chat/completions 路径）。
// 移动端做“策略提取”一次性 JSON 调用时复用，保证与桌面端 AI 对话的 URL 规则一致。
func OpenAIChatEndpoint(baseURL string) (string, string) {
	return openAIChatEndpoint(baseURL)
}
