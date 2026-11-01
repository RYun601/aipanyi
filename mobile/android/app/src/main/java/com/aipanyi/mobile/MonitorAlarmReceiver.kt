package com.aipanyi.mobile

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class MonitorAlarmReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        RuntimeLog.init(context)
        if (intent?.action != ACTION_ALARM) return
        RuntimeLog.info("MonitorAlarmReceiver", "alarm fired at=${System.currentTimeMillis()}")
        MonitorScheduler.startMonitorService(context, "alarm", MonitorService.ACTION_ALARM_START)
    }

    companion object {
        const val ACTION_ALARM = "com.aipanyi.mobile.MONITOR_ALARM"
    }
}
