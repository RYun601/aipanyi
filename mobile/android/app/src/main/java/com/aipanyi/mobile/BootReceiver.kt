package com.aipanyi.mobile

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log
import androidx.core.content.ContextCompat

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val action = intent.action ?: return
        if (action != Intent.ACTION_BOOT_COMPLETED && action != Intent.ACTION_MY_PACKAGE_REPLACED) return
        val active = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
            .getInt(AppNotifier.KEY_ACTIVE, 0)
        if (active <= 0) return
        try {
            ContextCompat.startForegroundService(
                context,
                Intent(context, MonitorService::class.java)
            )
        } catch (t: Throwable) {
            // Android 12+ 可能禁止从后台启动 FGS；退化为一条可点击的通知。
            Log.w("BootReceiver", "start service failed", t)
            try {
                NotificationHelper.notifyPriceAlert(
                    context,
                    "AI盘译",
                    "点击恢复价格监控",
                    ""
                )
            } catch (_: Throwable) {
            }
        }
    }
}
