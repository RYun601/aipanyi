package com.aipanyi.mobile

import android.content.Context

/**
 * Go 核心的通知出口。
 *
 * gomobile 会把 Go 的 Notifier 接口生成成 Java 接口 mobilecore.Notifier，
 * 方法名按 Java 习惯转成 lowerCamelCase：notify / updateMonitorStatus。
 */
class AppNotifier(private val context: Context) : mobilecore.Notifier {
    override fun notify(title: String, body: String, payload: String) {
        NotificationHelper.notifyPriceAlert(context, title, body, payload)
    }

    override fun updateMonitorStatus(active: Int) {
        RuntimeLog.info("AppNotifier", "active alerts=$active")
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putInt(KEY_ACTIVE, active)
            .apply()
        NotificationHelper.updateOngoing(context, active)
    }

    override fun updateMonitorSchedule(startMs: Long, endMs: Long, nextStartMs: Long) {
        RuntimeLog.info(
            "AppNotifier",
            "schedule from go start=$startMs end=$endMs next=$nextStartMs"
        )
        MonitorScheduler.applySchedule(context, startMs, endMs, nextStartMs)
        MonitorService.onScheduleChanged()
    }

    companion object {
        const val PREFS = "aipanyi"
        const val KEY_ACTIVE = "active_alerts"
        const val KEY_URL = "local_url"
        const val KEY_WINDOW_START = "monitor_window_start_ms"
        const val KEY_WINDOW_END = "monitor_window_end_ms"
        const val KEY_NEXT_START = "monitor_next_start_ms"
        const val ACTION_CORE_READY = "com.aipanyi.mobile.CORE_READY"
    }
}
