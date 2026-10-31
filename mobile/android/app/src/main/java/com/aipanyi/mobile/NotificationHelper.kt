package com.aipanyi.mobile

import android.Manifest
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat

object NotificationHelper {
    const val CHANNEL_MONITOR = "monitor"
    const val CHANNEL_ALERT = "price_alert"
    const val ONGOING_ID = 1001

    fun ensureChannels(ctx: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val nm = ctx.getSystemService(NotificationManager::class.java) ?: return
        val monitor = NotificationChannel(
            CHANNEL_MONITOR,
            ctx.getString(R.string.monitor_channel_name),
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = ctx.getString(R.string.monitor_channel_desc)
            setShowBadge(false)
        }
        val alert = NotificationChannel(
            CHANNEL_ALERT,
            ctx.getString(R.string.alert_channel_name),
            NotificationManager.IMPORTANCE_HIGH
        ).apply {
            description = ctx.getString(R.string.alert_channel_desc)
            enableVibration(true)
            enableLights(true)
        }
        nm.createNotificationChannel(monitor)
        nm.createNotificationChannel(alert)
    }

    fun buildOngoing(ctx: Context, active: Int): Notification {
        val intent = Intent(ctx, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
        }
        val pi = PendingIntent.getActivity(
            ctx, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val text = if (active > 0) "监控中 · $active 条价格提醒" else "监控中 · 暂无启用规则"
        return NotificationCompat.Builder(ctx, CHANNEL_MONITOR)
            .setSmallIcon(R.drawable.ic_stat_notify)
            .setContentTitle("AI盘译")
            .setContentText(text)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setContentIntent(pi)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .build()
    }

    fun updateOngoing(ctx: Context, active: Int) {
        if (!notificationsEnabled(ctx)) return
        NotificationManagerCompat.from(ctx).notify(ONGOING_ID, buildOngoing(ctx, active))
    }

    fun notifyPriceAlert(ctx: Context, title: String, body: String, payload: String) {
        if (!notificationsEnabled(ctx)) return
        ensureChannels(ctx)
        val intent = Intent(ctx, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            putExtra("stock_code", payload)
        }
        val pi = PendingIntent.getActivity(
            ctx, payload.hashCode(), intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val n = NotificationCompat.Builder(ctx, CHANNEL_ALERT)
            .setSmallIcon(R.drawable.ic_stat_notify)
            .setContentTitle(title)
            .setContentText(body)
            .setStyle(NotificationCompat.BigTextStyle().bigText(body))
            .setAutoCancel(true)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setCategory(NotificationCompat.CATEGORY_ALARM)
            .setDefaults(NotificationCompat.DEFAULT_ALL)
            .setContentIntent(pi)
            .build()
        NotificationManagerCompat.from(ctx).notify((System.currentTimeMillis() and 0xFFFFFF).toInt(), n)
    }

    fun notificationsEnabled(ctx: Context): Boolean {
        val granted = NotificationManagerCompat.from(ctx).areNotificationsEnabled()
        return granted && (Build.VERSION.SDK_INT < 33 || hasNotificationPermission(ctx))
    }

    fun hasNotificationPermission(ctx: Context): Boolean {
        return Build.VERSION.SDK_INT < 33 ||
            ctx.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
    }
}
