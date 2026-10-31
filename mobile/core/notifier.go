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
