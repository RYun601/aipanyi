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

    private var readyRequestId: String? = null

    private var stopRunnable: Runnable? = null
    private var alarmTimeout: Runnable? = null
    private var earlyStart: Runnable? = null
    private var earlyWakeLock: android.os.PowerManager.WakeLock? = null

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

    }

    private fun ensureCore(recoverOrigin: String? = null) {
        coreWorker.execute {
            if (stopping) return@execute
            try {
                val previous = coreUrl
                if (previous != null && CoreUrl.origin(previous) == recoverOrigin && !CoreUrl.healthy(previous)) {
                    RuntimeLog.warn(TAG, "restart unhealthy core origin=$recoverOrigin")
                    coreUrl = null
                    mobilecore.Mobilecore.stop()
                }
                val url = mobilecore.Mobilecore.start(filesDir.absolutePath, notifier)
                coreUrl = url
                handler.post {
                    if (!stopping && coreUrl == url) {
                        if (previous != url) RuntimeLog.info(TAG, "core started url=$url")
                        sendCoreReady(url)
                        if (!waitingForSchedule) scheduleStopFromPrefs()
                    }
                }
            } catch (t: Throwable) {
                RuntimeLog.error(TAG, "start mobile core failed", t)
            }
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        RuntimeLog.init(this)
        val action = intent?.action ?: "null"
        RuntimeLog.info(TAG, "onStartCommand action=$action uiVisible=$uiVisible")

        when (action) {
            ACTION_UI_VISIBLE -> {
                uiVisible = true
                readyRequestId = intent?.getStringExtra(EXTRA_REQUEST_ID)
                RuntimeLog.info(TAG, "ui visible")
                ensureCore(intent?.getStringExtra(EXTRA_RECOVER_ORIGIN))
            }
            ACTION_UI_HIDDEN -> {
                uiVisible = false
                readyRequestId = null
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
                startFromAlarm(intent?.getLongExtra(MonitorAlarmReceiver.EXTRA_TARGET, 0L) ?: 0L)
            }
            else -> {
                scheduleStopFromPrefs()
                stopIfIdle("sticky_restart")
                if (!stopping) ensureCore()
            }
        }

        if (!waitingForSchedule) {
            scheduleStopFromPrefs()
        }
        return START_STICKY
    }

    private fun startFromAlarm(target: Long) {
        stopRunnable?.let { handler.removeCallbacks(it) }
        stopRunnable = null
        if (earlyStart != null || waitingForSchedule) return
        waitingForSchedule = true
        val delay = target - System.currentTimeMillis()
        if (delay in 1..MonitorScheduler.EARLY_MS) {
            val power = getSystemService(POWER_SERVICE) as android.os.PowerManager
            earlyWakeLock = power.newWakeLock(android.os.PowerManager.PARTIAL_WAKE_LOCK, "aipanyi:opening")
                .apply { acquire(MonitorScheduler.EARLY_MS + 2_000L) }
            val check = object : Runnable {
                override fun run() {
                    val remaining = target - System.currentTimeMillis()
                    if (remaining in 1..MonitorScheduler.EARLY_MS) {
                        handler.postDelayed(this, remaining)
                        return
                    }
                    clearEarlyStart()
                    if (remaining > 0) {
                        waitingForSchedule = false
                        stopIfIdle("clock_moved_before_window")
                        return
                    }
                    refreshAlarmWindow()
                }
            }
            earlyStart = check
            handler.postDelayed(check, delay)
            RuntimeLog.info(TAG, "early alarm waiting target=${formatTime(target)} delayMs=$delay")
        } else {
            refreshAlarmWindow()
        }
    }

    private fun clearEarlyStart() {
        earlyStart?.let { handler.removeCallbacks(it) }
        earlyStart = null
        earlyWakeLock?.let { if (it.isHeld) it.release() }
        earlyWakeLock = null
    }

    private fun refreshAlarmWindow() {
        RuntimeLog.info(TAG, "alarm start; waiting for Go schedule")
        startAlarmTimeout()
        ensureCore()
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
        if (uiVisible || earlyStart != null || waitingForSchedule) {
            RuntimeLog.info(TAG, "keep service reason=$reason uiVisible=$uiVisible waiting=$waitingForSchedule")
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
        clearEarlyStart()
        alarmTimeout?.let { handler.removeCallbacks(it) }
        alarmTimeout = null
        RuntimeLog.info(TAG, "stop service reason=$reason")
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun sendCoreReady(url: String) {
        val id = readyRequestId ?: return
        if (!uiVisible) return
        try {
            val ready = Intent(AppNotifier.ACTION_CORE_READY)
                .setPackage(packageName)
                .putExtra(AppNotifier.KEY_URL, url)
                .putExtra(EXTRA_REQUEST_ID, id)
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
        clearEarlyStart()
        stopRunnable?.let { handler.removeCallbacks(it) }
        alarmTimeout?.let { handler.removeCallbacks(it) }
        handler.removeCallbacksAndMessages(null)
        stopping = true
        coreWorker.execute {
            try {
                mobilecore.Mobilecore.stop()
            } catch (t: Throwable) {
                RuntimeLog.error(TAG, "onDestroy stop core failed", t)
            }
        }
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
        const val EXTRA_REQUEST_ID = "connection_request_id"
        const val EXTRA_RECOVER_ORIGIN = "recover_origin"
        private val coreWorker = java.util.concurrent.Executors.newSingleThreadExecutor()
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

        fun deliverOpeningAlarm(target: Long): Boolean {
            val service = instance ?: return false
            if (service.stopping) return false
            service.startFromAlarm(target)
            return true
        }

        fun onWindowEnd() {
            instance?.stopIfIdle("wall_clock_end_alarm")
        }

        fun onScheduleChanged() {
            instance?.let {
                it.handler.post {
                    if (it.earlyStart != null) return@post
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
