package com.aipanyi.mobile

import android.app.AlarmManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import android.util.Log

/**
 * 前台服务：保活 + 常驻通知。
 *
 * Go 核心（本地 HTTP + 提醒引擎）跑在这个进程里；服务在前台，系统不会因为
 * Activity 退到后台就立刻回收进程。真正是否被杀仍取决于 ROM，设置页会给用户
 * 白名单/自启动的引导。
 */
class MonitorService : Service() {
    private val notifier by lazy { AppNotifier(applicationContext) }

    override fun onCreate() {
        super.onCreate()
        NotificationHelper.ensureChannels(this)
        startForeground(
            NotificationHelper.ONGOING_ID,
            NotificationHelper.buildOngoing(this, activeCount())
        )
        Thread {
            try {
                val url = mobilecore.Mobilecore.start(filesDir.absolutePath, notifier)
                getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
                    .edit()
                    .putString(AppNotifier.KEY_URL, url)
                    .apply()
            } catch (t: Throwable) {
                Log.e(TAG, "start mobile core failed", t)
            }
        }.start()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int = START_STICKY

    override fun onTaskRemoved(rootIntent: Intent?) {
        // 部分 ROM 划掉最近任务会连服务一起清；1 秒后用非精确闹钟尝试拉起。
        try {
            val restart = Intent(applicationContext, MonitorService::class.java)
            val pi = PendingIntent.getService(
                applicationContext, 1, restart,
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
            )
            val am = getSystemService(Context.ALARM_SERVICE) as AlarmManager
            am.set(AlarmManager.RTC, System.currentTimeMillis() + 1000, pi)
        } catch (t: Throwable) {
            Log.w(TAG, "schedule restart failed", t)
        }
        super.onTaskRemoved(rootIntent)
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun activeCount(): Int =
        getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
            .getInt(AppNotifier.KEY_ACTIVE, 0)

    companion object {
        private const val TAG = "MonitorService"
    }
}
