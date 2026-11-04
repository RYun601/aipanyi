package mobilecore

import "sync"

// Notifier is implemented by the Android shell to show system notifications.
//
// gomobile generates the Java interface mobilecore.Notifier. Kotlin implements:
//
//	override fun notify(title: String, body: String, payload: String)
//	override fun updateMonitorStatus(active: Int)
type Notifier interface {
	// Notify posts a price alert. payload is usually the stock code.
	Notify(title, body, payload string)
	// UpdateMonitorStatus updates the ongoing notification's active rule count.
	// int32 maps to Java int (Go int maps to Java long).
	UpdateMonitorStatus(active int32)
	// UpdateMonitorSchedule 下发当前/下一个监控窗口，Unix 毫秒；0 表示没有。
	// int64 在 gomobile 中映射成 Java long，避免 2038 年问题。
	UpdateMonitorSchedule(startMs, endMs, nextStartMs int64)
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

func notifyDevice(title, body, payload string) (status string) {
	n := currentNotifier()
	if n == nil {
		return "no_notifier"
	}
	defer func() {
		if recover() != nil {
			status = "bridge_failed"
		}
	}()
	n.Notify(title, body, payload)
	return "bridge_delivered"
}

func updateMonitorStatus(active int) {
	n := currentNotifier()
	if n == nil {
		return
	}
	defer func() { _ = recover() }()
	n.UpdateMonitorStatus(int32(active))
}

func sendMonitorSchedule(startMs, endMs, nextStartMs int64) {
	n := currentNotifier()
	if n == nil {
		return
	}
	defer func() { _ = recover() }()
	n.UpdateMonitorSchedule(startMs, endMs, nextStartMs)
}
