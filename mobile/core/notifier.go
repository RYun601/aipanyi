package mobilecore

import "sync"

// Notifier 是 Android 壳实现的系统通知出口。
//
// gomobile 会把该接口生成 Java 接口 mobilecore.Notifier，Kotlin 侧实现：
//
//	override fun notify(title: String, body: String, payload: String)
//	override fun updateMonitorStatus(active: Int)
type Notifier interface {
	// Notify 弹出一条价格提醒。payload 一般是股票代码，用于点击跳转。
	Notify(title, body, payload string)
	// UpdateMonitorStatus 更新常驻通知中的“监控中 N 条规则”。
	// 用 int32 是为了让 gomobile 生成 Java int（Go int 会映射成 Java long）。
	UpdateMonitorStatus(active int32)
}

var (
	notifierMu sync.RWMutex
	notifierV  Notifier
)

func setNotifier(n Notifier) {
	notifierMu.Lock()
	notifierV = n
	notifierMu.Unlock()
}

func currentNotifier() Notifier {
	notifierMu.RLock()
	defer notifierMu.RUnlock()
	return notifierV
}

func notifyDevice(title, body, payload string) {
	n := currentNotifier()
	if n == nil {
		return
	}
	defer func() { _ = recover() }()
	n.Notify(title, body, payload)
}

func updateMonitorStatus(active int) {
	n := currentNotifier()
	if n == nil {
		return
	}
	defer func() { _ = recover() }()
	n.UpdateMonitorStatus(int32(active))
}
