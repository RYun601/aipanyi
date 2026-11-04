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

internal enum class WindowAlarm { WAIT, START, RESCHEDULE, IGNORE }

object MonitorScheduler {
    private const val REQ_START = 1001
    private const val REQ_END = 1002
    private const val START_TARGET = "registered_monitor_start"
    const val EARLY_MS = 120_000L

    internal fun startDecision(at: Long, expected: Long, end: Long, now: Long): WindowAlarm = when {
        at <= 0 || at != expected -> WindowAlarm.IGNORE
        now < at - EARLY_MS || (end > at && now >= end) -> WindowAlarm.RESCHEDULE
        now < at -> WindowAlarm.WAIT
        else -> WindowAlarm.START
    }

    fun handleAlarm(context: Context, action: String?, at: Long) {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        val now = System.currentTimeMillis()
        val start = prefs.getLong(AppNotifier.KEY_WINDOW_START, 0)
        val end = prefs.getLong(AppNotifier.KEY_WINDOW_END, 0)
        RuntimeLog.info("MonitorScheduler", "alarm action=$action target=${format(at)} actual=${format(now)} exact=${canScheduleExactAlarms(context)}")
        if (action == MonitorAlarmReceiver.ACTION_END) {
            if (at <= 0 || at != end) return
            if (now < at) scheduleAlarm(context, at, endAlarm = true)
            else MonitorService.onWindowEnd()
            return
        }
        if (action != MonitorAlarmReceiver.ACTION_ALARM) return
        val decision = startDecision(at, prefs.getLong(START_TARGET, 0), if (at == start) end else 0, now)
        when (decision) {
            WindowAlarm.IGNORE -> Unit
            WindowAlarm.RESCHEDULE -> scheduleNext(context, "stale_window", refreshIfMissing = true)
            WindowAlarm.START, WindowAlarm.WAIT -> {
                prefs.edit().remove(START_TARGET).apply()
                if (!MonitorService.deliverOpeningAlarm(at) &&
                    !startMonitorService(context, "alarm_${decision.name.lowercase()}", MonitorService.ACTION_ALARM_START, at)) {
                    scheduleNext(context, "alarm_launch_rejected", after = at)
                }
            }
        }
    }

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
        if (!active) cancelAlarm(context, endAlarm = true)
        if (active) {
            if (!MonitorService.isRunning) startMonitorService(context, "schedule_active")
            scheduleAlarm(context, endMs, endAlarm = true)
            if (nextStartMs > now) scheduleStart(context, nextStartMs) else cancelStartAlarm(context)
        } else if (startMs > now) {
            scheduleStart(context, startMs)
        } else if (nextStartMs > now) {
            scheduleStart(context, nextStartMs)
        } else {
            cancelStartAlarm(context)
        }
    }

    fun scheduleNext(context: Context, reason: String, refreshIfMissing: Boolean = false, after: Long = 0) {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        val start = prefs.getLong(AppNotifier.KEY_WINDOW_START, 0)
        val next = prefs.getLong(AppNotifier.KEY_NEXT_START, 0)
        val now = maxOf(System.currentTimeMillis(), after)
        val at = when {
            start > now -> start
            next > now -> next
            else -> 0L
        }
        if (at > 0) {
            scheduleStart(context, at)
        } else {
            cancelStartAlarm(context)
            if (refreshIfMissing && prefs.getInt(AppNotifier.KEY_ACTIVE, 0) > 0 &&
                !MonitorService.deliverOpeningAlarm(0)) {
                startMonitorService(context, "recompute_schedule", MonitorService.ACTION_ALARM_START)
            }
        }
        RuntimeLog.info("MonitorScheduler", "scheduleNext reason=$reason at=${format(at)}")
    }

    fun resumeAfterBoot(context: Context) {
        val prefs = context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
        val start = prefs.getLong(AppNotifier.KEY_WINDOW_START, 0)
        val end = prefs.getLong(AppNotifier.KEY_WINDOW_END, 0)
        val now = System.currentTimeMillis()
        if (start > 0 && start <= now && now < end) {
            RuntimeLog.info("MonitorScheduler", "boot inside window start=${format(start)} end=${format(end)}")
            scheduleAlarm(context, end, endAlarm = true)
            startMonitorService(context, "boot_active")
            return
        }
        scheduleNext(context, "boot_or_clock_change", refreshIfMissing = true)
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

    fun startMonitorService(context: Context, reason: String, action: String? = null, target: Long = 0): Boolean {
        return try {
            val intent = Intent(context, MonitorService::class.java)
            if (action != null) intent.action = action
            intent.putExtra(MonitorAlarmReceiver.EXTRA_TARGET, target)
            ContextCompat.startForegroundService(context, intent)
            RuntimeLog.info("MonitorScheduler", "start service requested reason=$reason action=${action ?: "-"}")
            true
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
            false
        }
    }

    private fun scheduleStart(context: Context, atMs: Long) {
        context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
            .edit().putLong(START_TARGET, atMs).apply()
        scheduleAlarm(context, atMs, endAlarm = false)
    }

    private fun pendingAlarm(context: Context, endAlarm: Boolean, atMs: Long?, flags: Int): PendingIntent? {
        val intent = Intent(context, MonitorAlarmReceiver::class.java)
            .setAction(if (endAlarm) MonitorAlarmReceiver.ACTION_END else MonitorAlarmReceiver.ACTION_ALARM)
        if (atMs != null) intent.putExtra(MonitorAlarmReceiver.EXTRA_TARGET, atMs)
        return PendingIntent.getBroadcast(
            context, if (endAlarm) REQ_END else REQ_START, intent, flags or PendingIntent.FLAG_IMMUTABLE
        )
    }

    private fun scheduleAlarm(context: Context, atMs: Long, endAlarm: Boolean) {
        val am = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        val pi = pendingAlarm(context, endAlarm, atMs, PendingIntent.FLAG_UPDATE_CURRENT) ?: return
        val exact = canScheduleExactAlarms(context)
        try {
            if (exact) {
                am.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, atMs, pi)
            } else {
                am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, atMs, pi)
            }
            RuntimeLog.info("MonitorScheduler", "alarm scheduled end=$endAlarm at=${format(atMs)} exact=$exact")
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
        context.getSharedPreferences(AppNotifier.PREFS, Context.MODE_PRIVATE)
            .edit().remove(START_TARGET).apply()
        cancelAlarm(context, endAlarm = false)
    }

    private fun cancelAlarm(context: Context, endAlarm: Boolean) {
        val am = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        pendingAlarm(context, endAlarm, null, PendingIntent.FLAG_NO_CREATE)?.let(am::cancel)
    }

    private fun format(ms: Long): String {
        if (ms <= 0) return "-"
        return SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.US).format(Date(ms))
    }
}
