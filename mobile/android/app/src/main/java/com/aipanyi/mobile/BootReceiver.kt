package com.aipanyi.mobile

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        RuntimeLog.init(context)
        val action = intent.action ?: return
        if (action != Intent.ACTION_BOOT_COMPLETED && action != Intent.ACTION_MY_PACKAGE_REPLACED &&
            action != Intent.ACTION_TIME_CHANGED && action != Intent.ACTION_TIMEZONE_CHANGED &&
            action != android.app.AlarmManager.ACTION_SCHEDULE_EXACT_ALARM_PERMISSION_STATE_CHANGED) return
        RuntimeLog.info("BootReceiver", "action=$action")
        val active = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
            .getInt(AppNotifier.KEY_ACTIVE, 0)
        RuntimeLog.info("BootReceiver", "active alerts=$active")
        if (active <= 0) return
        MonitorScheduler.resumeAfterBoot(context)
    }
}
