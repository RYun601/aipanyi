package com.aipanyi.mobile

import android.content.Context
import android.os.PowerManager
import android.webkit.JavascriptInterface

/**
 * 暴露给 WebView 的最小原生能力集合。
 * 只加载本机 127.0.0.1 页面，不对外部网页开放。
 */
class NativeBridge(private val activity: MainActivity) {

    @JavascriptInterface
    fun notificationsEnabled(): Boolean = NotificationHelper.notificationsEnabled(activity)

    @JavascriptInterface
    fun requestNotifications(): Boolean {
        activity.runOnUiThread { activity.requestNotificationPermissionNow() }
        return true
    }

    @JavascriptInterface
    fun isIgnoringBatteryOptimizations(): Boolean {
        val pm = activity.getSystemService(Context.POWER_SERVICE) as PowerManager
        return pm.isIgnoringBatteryOptimizations(activity.packageName)
    }

    @JavascriptInterface
    fun openBatterySettings() {
        activity.runOnUiThread { activity.openBatterySettings() }
    }

    @JavascriptInterface
    fun openAutoStartSettings() {
        activity.runOnUiThread { activity.openAutoStartSettings() }
    }

    @JavascriptInterface
    fun openNotificationSettings() {
        activity.runOnUiThread { activity.openNotificationSettings() }
    }

    @JavascriptInterface
    fun appVersion(): String = BuildConfig.VERSION_NAME
}
