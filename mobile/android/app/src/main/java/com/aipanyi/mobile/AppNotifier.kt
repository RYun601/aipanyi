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
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putInt(KEY_ACTIVE, active)
            .apply()
        NotificationHelper.updateOngoing(context, active)
    }

    companion object {
        const val PREFS = "aipanyi"
        const val KEY_ACTIVE = "active_alerts"
        const val KEY_URL = "local_url"
    }
}
