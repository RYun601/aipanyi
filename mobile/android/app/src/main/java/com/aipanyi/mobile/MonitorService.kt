package com.aipanyi.mobile

import android.app.Service
import android.content.Intent
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.util.Log

class MonitorService : Service() {
    private val notifier by lazy { AppNotifier(applicationContext) }
    private val handler = Handler(Looper.getMainLooper())

    @Volatile
    private var coreUrl: String? = null

    @Volatile
    private var uiVisible = false

    @Volatile
    private var stopping = false

    @Volatile
    private var waitingForSchedule = false

    private var stopRunnable: Runnable? = null
    private var alarmTimeout: Runnable? = null

    override fun onCreate() {
        super.onCreate()
        RuntimeLog.init(this)
        instance = this
        isRunning = true
        RuntimeLog.info(TAG, "service onCreate")
        NotificationHelper.ensureChannels(this)
        startForeground(
            NotificationHelper.ONGOING_ID,
            NotificationHelper.buildOngoing(this, activeCount())
        )

        Thread {
            try {
                RuntimeLog.info(TAG, "core starting dataDir=${filesDir.absolutePath}")
                val url = mobilecore.Mobilecore.start(filesDir.absolutePath, notifier)
                if (stopping) {
                    RuntimeLog.info(TAG, "core started after stop request, stopping core")
                    mobilecore.Mobilecore.stop()
                    return@Thread
                }
                coreUrl = url
                getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
                    .edit()
                    .putString(AppNotifier.KEY_URL, url)
                    .commit()
                RuntimeLog.info(TAG, "core started url=$url")
                sendCoreReady(url)
                if (!waitingForSchedule) {
                    scheduleStopFromPrefs()
                }
            } catch (t: Throwable) {
                RuntimeLog.error(TAG, "start mobile core failed", t)
                Log.e(TAG, "start mobile core failed", t)
            }
        }.start()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        RuntimeLog.init(this)
        val action = intent?.action ?: "null"
        RuntimeLog.info(TAG, "onStartCommand action=$action uiVisible=$uiVisible")

        when (action) {
            ACTION_UI_VISIBLE -> {
                uiVisible = true
                RuntimeLog.info(TAG, "ui visible")
            }
            ACTION_UI_HIDDEN -> {
                uiVisible = false
                RuntimeLog.info(TAG, "ui hidden")
                stopIfIdle("ui_hidden")
            }
            ACTION_EXIT -> {
                RuntimeLog.info(TAG, "manual exit action")
                stopForExit()
                return START_NOT_STICKY
            }
            ACTION_SCHEDULE_CHANGED -> {
                scheduleStopFromPrefs()
                stopIfIdle("schedule_changed")
            }
            ACTION_ALARM_START -> {
                // 这个闹钟是给“下一个窗口”用的；此时 SharedPreferences 里仍是上一个窗口的
                // start/end，不能按旧窗口立即停止。等 Go 核心启动/刷新后通过
                // UpdateMonitorSchedule 写入新窗口。
                stopRunnable?.let { handler.removeCallbacks(it) }
                stopRunnable = null
                uiVisible = false
                waitingForSchedule = true
                RuntimeLog.info(TAG, "alarm start; waiting for Go schedule")
                startAlarmTimeout()
                refreshCoreForAlarm()
            }
            else -> {
                scheduleStopFromPrefs()
                stopIfIdle("sticky_restart")
            }
        }

        coreUrl?.let { sendCoreReady(it) }
        if (!waitingForSchedule) {
            scheduleStopFromPrefs()
        }
        return START_STICKY
    }

    private fun refreshCoreForAlarm() {
        Thread {
            try {
                RuntimeLog.info(TAG, "refresh core for alarm")
                val url = mobilecore.Mobilecore.start(filesDir.absolutePath, notifier)
                if (stopping) {
                    mobilecore.Mobilecore.stop()
                    return@Thread
                }
                coreUrl = url
                getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
                    .edit()
                    .putString(AppNotifier.KEY_URL, url)
                    .commit()
                sendCoreReady(url)
            } catch (t: Throwable) {
                RuntimeLog.error(TAG, "refresh core for alarm failed", t)
            }
        }.start()
    }

    private fun startAlarmTimeout() {
        alarmTimeout?.let { handler.removeCallbacks(it) }
        val runnable = Runnable {
            if (waitingForSchedule) {
                waitingForSchedule = false
                RuntimeLog.warn(TAG, "no schedule received after alarm, stopping")
                stopIfIdle("alarm_schedule_timeout")
            }
        }
        alarmTimeout = runnable
        handler.postDelayed(runnable, 120_000L)
    }

    private fun scheduleStopFromPrefs() {
        if (waitingForSchedule) return
        val prefs = getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
        val end = prefs.getLong(AppNotifier.KEY_WINDOW_END, 0)
        stopRunnable?.let { handler.removeCallbacks(it) }
        stopRunnable = null
        if (end <= 0) return

        val delay = end - System.currentTimeMillis()
        val runnable = Runnable {
            RuntimeLog.info(TAG, "window end reached end=${formatTime(end)}")
            stopIfIdle("window_end")
        }
        stopRunnable = runnable
        if (delay <= 0) {
            RuntimeLog.info(TAG, "window already ended")
            handler.post(runnable)
        } else {
            RuntimeLog.info(TAG, "stop timer scheduled delayMs=$delay")
            handler.postDelayed(runnable, delay)
        }
    }

    private fun stopIfIdle(reason: String) {
        if (uiVisible) {
            RuntimeLog.info(TAG, "keep service reason=$reason uiVisible=true")
            return
        }
        if (MonitorScheduler.isInWindow(this)) {
            RuntimeLog.info(TAG, "keep service reason=$reason inside window")
            return
        }
        stopInternal(reason)
    }

    private fun stopForExit() {
        stopInternal("manual_exit")
    }

    private fun stopInternal(reason: String) {
        if (stopping) return
        stopping = true
        waitingForSchedule = false
        alarmTimeout?.let { handler.removeCallbacks(it) }
        alarmTimeout = null
        RuntimeLog.info(TAG, "stop service reason=$reason")
        Thread {
            try {
                mobilecore.Mobilecore.stop()
            } catch (t: Throwable) {
                RuntimeLog.error(TAG, "stop mobile core failed", t)
            }
        }.start()
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun sendCoreReady(url: String) {
        try {
            val ready = Intent(AppNotifier.ACTION_CORE_READY)
                .setPackage(packageName)
                .putExtra(AppNotifier.KEY_URL, url)
            sendBroadcast(ready)
        } catch (t: Throwable) {
            RuntimeLog.error(TAG, "send core ready failed", t)
        }
    }

    override fun onTaskRemoved(rootIntent: Intent?) {
        RuntimeLog.info(TAG, "onTaskRemoved uiVisible=$uiVisible")
        stopIfIdle("task_removed")
        super.onTaskRemoved(rootIntent)
    }

    override fun onDestroy() {
        RuntimeLog.info(TAG, "service onDestroy")
        stopRunnable?.let { handler.removeCallbacks(it) }
        alarmTimeout?.let { handler.removeCallbacks(it) }
        handler.removeCallbacksAndMessages(null)
        Thread {
            try {
                mobilecore.Mobilecore.stop()
            } catch (t: Throwable) {
                RuntimeLog.error(TAG, "onDestroy stop core failed", t)
            }
        }.start()
        instance = null
        isRunning = false
        MonitorScheduler.scheduleNext(applicationContext, "service_destroy")
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun activeCount(): Int =
        getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
            .getInt(AppNotifier.KEY_ACTIVE, 0)

    private fun formatTime(ms: Long): String =
        java.text.SimpleDateFormat("yyyy-MM-dd HH:mm:ss", java.util.Locale.US)
            .format(java.util.Date(ms))

    companion object {
        const val TAG = "MonitorService"
        const val ACTION_UI_VISIBLE = "com.aipanyi.mobile.UI_VISIBLE"
        const val ACTION_UI_HIDDEN = "com.aipanyi.mobile.UI_HIDDEN"
        const val ACTION_EXIT = "com.aipanyi.mobile.EXIT"
        const val ACTION_SCHEDULE_CHANGED = "com.aipanyi.mobile.SCHEDULE_CHANGED"
        const val ACTION_ALARM_START = "com.aipanyi.mobile.ALARM_START"

        @Volatile
        var instance: MonitorService? = null
            private set

        @Volatile
        var isRunning: Boolean = false
            private set

        fun onScheduleChanged() {
            instance?.let {
                it.handler.post {
                    it.waitingForSchedule = false
                    it.alarmTimeout?.let { r -> it.handler.removeCallbacks(r) }
                    it.alarmTimeout = null
                    it.scheduleStopFromPrefs()
                    it.stopIfIdle("schedule_changed")
                }
            }
        }
    }
}
