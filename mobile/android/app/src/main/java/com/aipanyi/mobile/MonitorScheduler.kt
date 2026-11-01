package com.aipanyi.mobile

import android.app.AlarmManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.content.ContextCompat
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

object MonitorScheduler {
    private const val REQ_START = 1001

    fun applySchedule(context: Context, startMs: Long, endMs: Long, nextStartMs: Long) {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        prefs.edit()
            .putLong(AppNotifier.KEY_WINDOW_START, startMs)
            .putLong(AppNotifier.KEY_WINDOW_END, endMs)
            .putLong(AppNotifier.KEY_NEXT_START, nextStartMs)
            .apply()

        val now = System.currentTimeMillis()
        RuntimeLog.info(
            "MonitorScheduler",
            "apply schedule start=${format(startMs)} end=${format(endMs)} next=${format(nextStartMs)} now=${format(now)}"
        )

        val active = startMs > 0 && startMs <= now && now < endMs
        if (active) {
            startMonitorService(context, "schedule_active")
            if (nextStartMs > now) scheduleStart(context, nextStartMs) else cancelStartAlarm(context)
        } else if (startMs > now) {
            scheduleStart(context, startMs)
        } else if (nextStartMs > now) {
            scheduleStart(context, nextStartMs)
        } else {
            cancelStartAlarm(context)
        }
    }

    fun scheduleNext(context: Context, reason: String) {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        val start = prefs.getLong(AppNotifier.KEY_WINDOW_START, 0)
        val next = prefs.getLong(AppNotifier.KEY_NEXT_START, 0)
        val now = System.currentTimeMillis()
        val at = when {
            start > now -> start
            next > now -> next
            else -> 0L
        }
        if (at > 0) scheduleStart(context, at) else cancelStartAlarm(context)
        RuntimeLog.info("MonitorScheduler", "scheduleNext reason=$reason at=${format(at)}")
    }

    fun resumeAfterBoot(context: Context) {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        val start = prefs.getLong(AppNotifier.KEY_WINDOW_START, 0)
        val end = prefs.getLong(AppNotifier.KEY_WINDOW_END, 0)
        val now = System.currentTimeMillis()
        if (start > 0 && start <= now && now < end) {
            RuntimeLog.info("MonitorScheduler", "boot inside window start=${format(start)} end=${format(end)}")
            startMonitorService(context, "boot_active")
            return
        }
        scheduleNext(context, "boot")
    }

    fun isInWindow(context: Context): Boolean {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        val start = prefs.getLong(AppNotifier.KEY_WINDOW_START, 0)
        val end = prefs.getLong(AppNotifier.KEY_WINDOW_END, 0)
        val now = System.currentTimeMillis()
        return start > 0 && start <= now && now < end
    }

    fun canScheduleExactAlarms(context: Context): Boolean {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S) return true
        val am = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        return am.canScheduleExactAlarms()
    }

    fun startMonitorService(context: Context, reason: String, action: String? = null) {
        try {
            val intent = Intent(context, MonitorService::class.java)
            if (action != null) intent.action = action
            ContextCompat.startForegroundService(context, intent)
            RuntimeLog.info("MonitorScheduler", "start service requested reason=$reason action=${action ?: "-"}")
        } catch (t: Throwable) {
            RuntimeLog.error("MonitorScheduler", "start service failed reason=$reason action=${action ?: "-"}", t)
            try {
                NotificationHelper.notifyPriceAlert(
                    context,
                    "AI盘译",
                    "后台监控启动失败，点击打开应用恢复",
                    ""
                )
            } catch (_: Throwable) {
            }
        }
    }

    private fun scheduleStart(context: Context, atMs: Long) {
        val am = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        val intent = Intent(context, MonitorAlarmReceiver::class.java)
            .setAction(MonitorAlarmReceiver.ACTION_ALARM)
        val pi = PendingIntent.getBroadcast(
            context,
            REQ_START,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val exact = canScheduleExactAlarms(context)
        try {
            if (exact) {
                am.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, atMs, pi)
            } else {
                am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, atMs, pi)
            }
            RuntimeLog.info("MonitorScheduler", "alarm scheduled at=${format(atMs)} exact=$exact")
        } catch (t: Throwable) {
            RuntimeLog.error("MonitorScheduler", "alarm schedule failed at=${format(atMs)} exact=$exact", t)
            try {
                am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, atMs, pi)
                RuntimeLog.info("MonitorScheduler", "alarm fallback to inexact at=${format(atMs)}")
            } catch (fallback: Throwable) {
                RuntimeLog.error("MonitorScheduler", "alarm fallback failed", fallback)
            }
        }
    }

    private fun cancelStartAlarm(context: Context) {
        val am = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        val intent = Intent(context, MonitorAlarmReceiver::class.java)
            .setAction(MonitorAlarmReceiver.ACTION_ALARM)
        val pi = PendingIntent.getBroadcast(
            context,
            REQ_START,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        am.cancel(pi)
        RuntimeLog.info("MonitorScheduler", "alarm cancelled")
    }

    private fun format(ms: Long): String {
        if (ms <= 0) return "-"
        return SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.US).format(Date(ms))
    }
}
