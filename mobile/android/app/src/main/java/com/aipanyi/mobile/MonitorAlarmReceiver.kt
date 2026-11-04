package com.aipanyi.mobile

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class MonitorAlarmReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        RuntimeLog.init(context)
        MonitorScheduler.handleAlarm(context, intent?.action, intent?.getLongExtra(EXTRA_TARGET, 0L) ?: 0L)
    }

    companion object {
        const val ACTION_END = "com.aipanyi.mobile.MONITOR_END"
        const val EXTRA_TARGET = "monitor_alarm_target"
        const val ACTION_ALARM = "com.aipanyi.mobile.MONITOR_ALARM"
    }
}
