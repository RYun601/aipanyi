package com.aipanyi.mobile

import android.Manifest
import android.annotation.SuppressLint
import android.content.BroadcastReceiver
import android.content.ComponentName
import android.content.Intent
import android.content.IntentFilter
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import android.util.Log
import android.view.View
import android.webkit.WebChromeClient
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.ProgressBar
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat

class MainActivity : AppCompatActivity() {

    private lateinit var webView: WebView
    private lateinit var progress: ProgressBar
    private lateinit var errorView: TextView

    private var lastGoodUrl: String? = null
    private var loadingUrl: String? = null
    private var pendingStockCode: String? = null
    private var pendingEventId: Long? = null
    private var loadingDocument = false
    private var loadingStockCode: String? = null
    private var loadingEventId: Long? = null
    private var coreReceiver: BroadcastReceiver? = null
    private var exiting = false
    private var requestId: String? = null
    private var visible = false
    private var navigationFailed = false
    private var autoRetried = false
    private var requestStarted = 0L
    private val handler = Handler(Looper.getMainLooper())
    private val healthWorker = java.util.concurrent.Executors.newSingleThreadExecutor()
    private val coreTimeout = Runnable {
        requestId = null
        navigationFailed = true
        loadingUrl = null
        webView.stopLoading()
        showError("本地页面连接超时，点击重试")
    }

    private val notificationPermissionLauncher =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
            if (!granted) {
                Toast.makeText(this, "未开启通知权限，价格提醒无法弹出", Toast.LENGTH_LONG).show()
            }
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        RuntimeLog.init(this)
        RuntimeLog.info(TAG, "onCreate")
        setContentView(R.layout.activity_main)
        webView = findViewById(R.id.webview)
        progress = findViewById(R.id.progress)
        errorView = findViewById(R.id.error)

        NotificationHelper.ensureChannels(this)
        readTarget(intent)

        setupWebView()
        maybeRequestNotificationPermission()
        registerCoreReceiver()
    }

    @SuppressLint("SetJavaScriptEnabled")
    private fun setupWebView() {
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG)
        webView.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            databaseEnabled = true
            allowFileAccess = false
            allowContentAccess = false
            mixedContentMode = WebSettings.MIXED_CONTENT_COMPATIBILITY_MODE
            mediaPlaybackRequiresUserGesture = false
            cacheMode = WebSettings.LOAD_DEFAULT
        }
        webView.addJavascriptInterface(NativeBridge(this), "AiPanyi")
        webView.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView?, request: WebResourceRequest?): Boolean {
                val uri = request?.url ?: return false
                val host = uri.host ?: ""
                if (host == "127.0.0.1" || host == "localhost") return false
                return try {
                    startActivity(Intent(Intent.ACTION_VIEW, uri))
                    true
                } catch (t: Throwable) {
                    Log.w(TAG, "open external url failed", t)
                    true
                }
            }

            override fun onPageStarted(view: WebView?, url: String?, favicon: android.graphics.Bitmap?) {
                if (matchesLoading(url)) {
                    progress.visibility = View.VISIBLE
                    errorView.visibility = View.GONE
                }
            }

            override fun onPageFinished(view: WebView?, url: String?) {
                if (matchesLoading(url) && !navigationFailed) completeNavigation(url!!)
            }

            override fun doUpdateVisitedHistory(view: WebView?, url: String?, isReload: Boolean) {
                if (!navigationFailed && loadingUrl == null && CoreUrl.origin(url) == CoreUrl.origin(lastGoodUrl)) {
                    lastGoodUrl = url
                }
                // 同一文档的 hash 导航有时不会收到 onPageFinished。
                if (!navigationFailed && !loadingDocument && url != null && url == loadingUrl &&
                    lastGoodUrl?.substringBefore('#') == url.substringBefore('#')) {
                    completeNavigation(url)
                }
            }

            override fun onReceivedError(
                view: WebView?,
                request: WebResourceRequest?,
                error: WebResourceError?
            ) {
                val url = request?.url?.toString()
                if (request?.isForMainFrame == true &&
                    (matchesLoading(url) || (loadingUrl == null && url == lastGoodUrl))) {
                    RuntimeLog.warn(TAG, "navigation failed origin=${CoreUrl.origin(url)} error=${error?.errorCode}")
                    navigationFailed = true
                    loadingUrl = null
                    if (visible) retryConnection(CoreUrl.origin(url))
                }
            }
        }
        webView.webChromeClient = WebChromeClient()
    }

    private fun requestConnection(recoverOrigin: String? = null) {
        val id = java.util.UUID.randomUUID().toString()
        requestId = id
        requestStarted = android.os.SystemClock.elapsedRealtime()
        errorView.visibility = View.GONE
        if (lastGoodUrl == null || navigationFailed) progress.visibility = View.VISIBLE
        startMonitorService(MonitorService.ACTION_UI_VISIBLE, id, recoverOrigin)
        handler.removeCallbacks(coreTimeout)
        handler.postDelayed(coreTimeout, 15000)
    }

    private fun retryConnection(origin: String?) {
        if (autoRetried) {
            handler.removeCallbacks(coreTimeout)
            requestId = null
            showError("本地页面连接失败，点击重试")
            return
        }
        autoRetried = true
        requestConnection(origin)
    }

    private fun registerCoreReceiver() {
        if (coreReceiver != null) return
        val receiver = object : BroadcastReceiver() {
            override fun onReceive(context: android.content.Context?, intent: Intent?) {
                val url = intent?.getStringExtra(AppNotifier.KEY_URL)
                val id = intent?.getStringExtra(MonitorService.EXTRA_REQUEST_ID)
                if (!url.isNullOrBlank() && id != null && id == requestId) onCoreReady(url, id)
            }
        }
        coreReceiver = receiver
        ContextCompat.registerReceiver(
            this,
            receiver,
            IntentFilter(AppNotifier.ACTION_CORE_READY),
            ContextCompat.RECEIVER_NOT_EXPORTED
        )
    }

    private fun onCoreReady(url: String, id: String) {
        val origin = CoreUrl.origin(url) ?: return
        healthWorker.execute {
            val healthy = CoreUrl.healthy(origin)
            handler.post {
                if (!visible || id != requestId) return@post
                RuntimeLog.info(TAG, "core health origin=$origin healthy=$healthy elapsedMs=${android.os.SystemClock.elapsedRealtime() - requestStarted}")
                if (!healthy) {
                    navigationFailed = true
                    retryConnection(origin)
                } else {
                    loadReadyOrigin(origin)
                }
            }
        }
    }

    private fun loadReadyOrigin(origin: String) {
        val code = pendingStockCode
        val eventId = pendingEventId
        if (code == null && !CoreUrl.shouldReload(lastGoodUrl, origin, navigationFailed) && loadingUrl == null) {
            handler.removeCallbacks(coreTimeout)
            requestId = null
            return
        }
        val target = if (code == null) CoreUrl.reloadUrl(origin, lastGoodUrl) else {
            val event = eventId?.let { "?eventId=$it" } ?: ""
            "$origin/#/stock/${Uri.encode(code)}$event"
        }
        if (loadingUrl == target) return
        val forceReload = navigationFailed && CoreUrl.origin(lastGoodUrl) == origin
        loadingStockCode = code
        loadingEventId = eventId
        loadingUrl = target
        navigationFailed = false
        if (target == lastGoodUrl && !forceReload) {
            completeNavigation(target)
            return
        }
        webView.visibility = View.VISIBLE
        loadingDocument = forceReload || CoreUrl.origin(lastGoodUrl) != origin
        RuntimeLog.info(TAG, "navigation origin=$origin recovery=$forceReload")
        if (forceReload) {
            // A hash-only navigation can leave the old document and WS alive.
            webView.loadUrl("about:blank")
        }
        webView.loadUrl(target)
    }

    private fun matchesLoading(url: String?): Boolean =
        url != null && loadingUrl != null && CoreUrl.origin(url) == CoreUrl.origin(loadingUrl) &&
            url.substringBefore('#') == loadingUrl?.substringBefore('#')

    private fun completeNavigation(url: String) {
        lastGoodUrl = url
        loadingUrl = null
        if (loadingStockCode == pendingStockCode && loadingEventId == pendingEventId) {
            pendingStockCode = null
            pendingEventId = null
        }
        handler.removeCallbacks(coreTimeout)
        requestId = null
        progress.visibility = View.GONE
        RuntimeLog.info(TAG, "navigation ready origin=${CoreUrl.origin(url)} elapsedMs=${android.os.SystemClock.elapsedRealtime() - requestStarted}")
    }

    private fun readTarget(intent: Intent?) {
        intent?.getStringExtra("stock_code")?.takeIf { it.isNotBlank() }?.let {
            pendingStockCode = it
            pendingEventId = intent.getLongExtra("alert_event_id", 0L).takeIf { id -> id > 0 }
        }
    }

    private fun showError(message: String) {
        progress.visibility = View.GONE
        webView.visibility = View.GONE
        errorView.text = message
        errorView.visibility = View.VISIBLE
        errorView.setOnClickListener {
            autoRetried = false
            requestConnection()
        }
    }

    private fun startMonitorService(action: String, id: String, recoverOrigin: String?) {
        try {
            ContextCompat.startForegroundService(
                this,
                Intent(this, MonitorService::class.java).setAction(action)
                    .putExtra(MonitorService.EXTRA_REQUEST_ID, id)
                    .putExtra(MonitorService.EXTRA_RECOVER_ORIGIN, recoverOrigin)
            )
            RuntimeLog.info(TAG, "start service action=$action")
        } catch (t: Throwable) {
            RuntimeLog.error(TAG, "start monitor service failed action=$action", t)
            Log.w(TAG, "start monitor service failed", t)
        }
    }

    private fun maybeRequestNotificationPermission() {
        if (Build.VERSION.SDK_INT < 33) return
        if (NotificationHelper.hasNotificationPermission(this)) return
        val prefs = getSharedPreferences(AppNotifier.PREFS, MODE_PRIVATE)
        if (prefs.getBoolean("asked_notifications", false)) return
        prefs.edit().putBoolean("asked_notifications", true).apply()
        notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
    }

    fun requestNotificationPermissionNow() {
        if (Build.VERSION.SDK_INT < 33) return
        if (NotificationHelper.hasNotificationPermission(this)) {
            openNotificationSettings()
            return
        }
        notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
    }

    fun openBatterySettings() {
        try {
            startActivity(
                Intent(
                    Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                    Uri.parse("package:$packageName")
                )
            )
        } catch (t: Throwable) {
            try {
                startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS))
            } catch (_: Throwable) {
                openAppDetails()
            }
        }
    }

    fun openAutoStartSettings() {
        val candidates = listOf(
            ComponentName(
                "com.hihonor.systemmanager",
                "com.hihonor.systemmanager.startupmgr.ui.StartupNormalAppListActivity"
            ),
            ComponentName(
                "com.huawei.systemmanager",
                "com.huawei.systemmanager.startupmgr.ui.StartupNormalAppListActivity"
            ),
            ComponentName(
                "com.hihonor.systemmanager",
                "com.hihonor.systemmanager.appcontrol.activity.StartupAppControlActivity"
            ),
            ComponentName(
                "com.huawei.systemmanager",
                "com.huawei.systemmanager.appcontrol.activity.StartupAppControlActivity"
            )
        )
        for (c in candidates) {
            try {
                startActivity(Intent().setComponent(c))
                return
            } catch (_: Throwable) {
            }
        }
        openAppDetails()
    }

    fun openNotificationSettings() {
        try {
            startActivity(
                Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS)
                    .putExtra(Settings.EXTRA_APP_PACKAGE, packageName)
            )
        } catch (_: Throwable) {
            openAppDetails()
        }
    }

    private fun openAppDetails() {
        try {
            startActivity(
                Intent(
                    Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                    Uri.parse("package:$packageName")
                )
            )
        } catch (t: Throwable) {
            Toast.makeText(this, "无法打开系统设置", Toast.LENGTH_SHORT).show()
        }
    }

    override fun onStart() {
        super.onStart()
        RuntimeLog.info(TAG, "onStart")
        visible = true
        autoRetried = false
        requestConnection()
    }

    override fun onStop() {
        super.onStop()
        RuntimeLog.info(TAG, "onStop exiting=$exiting")
        visible = false
        requestId = null
        handler.removeCallbacks(coreTimeout)
        if (exiting || isChangingConfigurations) return
        // 不要把调度状态读回主进程：SharedPreferences 跨进程缓存可能过期。
        // 只通知“UI 不可见”，由 :monitor 进程里的服务按自己新鲜的调度决定是否停止。
        try {
            startService(
                Intent(this, MonitorService::class.java)
                    .setAction(MonitorService.ACTION_UI_HIDDEN)
            )
        } catch (t: Throwable) {
            RuntimeLog.error(TAG, "send ui hidden failed", t)
            stopService(Intent(this, MonitorService::class.java))
        }
    }

    fun exitApp() {
        exiting = true
        RuntimeLog.info(TAG, "manual exit requested")
        try {
            startService(
                Intent(this, MonitorService::class.java)
                    .setAction(MonitorService.ACTION_EXIT)
            )
        } catch (t: Throwable) {
            RuntimeLog.error(TAG, "request service exit failed", t)
            stopService(Intent(this, MonitorService::class.java))
        }
        finishAndRemoveTask()
    }

    fun canScheduleExactAlarms(): Boolean = MonitorScheduler.canScheduleExactAlarms(this)

    fun requestExactAlarmPermission() {
        RuntimeLog.info(TAG, "request exact alarm permission")
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S) return
        try {
            startActivity(
                Intent(
                    Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM,
                    Uri.parse("package:$packageName")
                )
            )
        } catch (t: Throwable) {
            RuntimeLog.error(TAG, "open exact alarm settings failed", t)
            openAppDetails()
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        readTarget(intent)
        if (visible && pendingStockCode != null) {
            autoRetried = false
            requestConnection()
        }
    }

    @Suppress("DEPRECATION")
    override fun onBackPressed() {
        if (this::webView.isInitialized && webView.canGoBack()) {
            webView.goBack()
        } else {
            super.onBackPressed()
        }
    }

    override fun onDestroy() {
        RuntimeLog.info(TAG, "onDestroy")
        requestId = null
        healthWorker.shutdownNow()
        handler.removeCallbacks(coreTimeout)
        coreReceiver?.let {
            try {
                unregisterReceiver(it)
            } catch (_: Throwable) {
            }
        }
        coreReceiver = null
        if (this::webView.isInitialized) {
            webView.removeJavascriptInterface("AiPanyi")
            webView.destroy()
        }
        super.onDestroy()
    }

    companion object {
        private const val TAG = "MainActivity"
    }
}
