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

    private var baseUrl: String? = null
    private var pendingStockCode: String? = null
    private var coreReceiver: BroadcastReceiver? = null
    private var exiting = false
    private val handler = Handler(Looper.getMainLooper())
    private val coreTimeout = Runnable {
        if (baseUrl == null) {
            showError("本地服务启动失败：请确认通知权限/后台设置后点击重试")
        }
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
        pendingStockCode = intent?.getStringExtra("stock_code")

        setupWebView()
        maybeRequestNotificationPermission()
        startMonitorServiceAndLoad()
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
                progress.visibility = View.VISIBLE
                errorView.visibility = View.GONE
                webView.visibility = View.VISIBLE
            }

            override fun onPageFinished(view: WebView?, url: String?) {
                progress.visibility = View.GONE
            }

            override fun onReceivedError(
                view: WebView?,
                request: WebResourceRequest?,
                error: WebResourceError?
            ) {
                if (request?.isForMainFrame == true) {
                    showError("页面加载失败：${error?.description ?: "未知错误"}")
                }
            }
        }
        webView.webChromeClient = WebChromeClient()
    }

    private fun startMonitorServiceAndLoad() {
        errorView.visibility = View.GONE
        progress.visibility = View.VISIBLE
        registerCoreReceiver()
        startMonitorService(MonitorService.ACTION_UI_VISIBLE)
        handler.removeCallbacks(coreTimeout)
        handler.postDelayed(coreTimeout, 15000)
    }

    private fun registerCoreReceiver() {
        if (coreReceiver != null) return
        val receiver = object : BroadcastReceiver() {
            override fun onReceive(context: android.content.Context?, intent: Intent?) {
                val url = intent?.getStringExtra(AppNotifier.KEY_URL)
                if (!url.isNullOrBlank()) onCoreReady(url)
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

    private fun onCoreReady(url: String) {
        if (baseUrl != null) return
        baseUrl = url
        handler.removeCallbacks(coreTimeout)
        loadTarget()
    }

    private fun loadTarget() {
        val url = baseUrl ?: return
        val base = url.trimEnd('/')
        val code = pendingStockCode
        val target = if (code.isNullOrBlank()) "$base/" else "$base/#/stock/${Uri.encode(code)}"
        pendingStockCode = null
        webView.loadUrl(target)
    }

    private fun showError(message: String) {
        progress.visibility = View.GONE
        webView.visibility = View.GONE
        errorView.text = message
        errorView.visibility = View.VISIBLE
        errorView.setOnClickListener { startMonitorServiceAndLoad() }
    }

    private fun startMonitorService(action: String = MonitorService.ACTION_UI_VISIBLE) {
        try {
            ContextCompat.startForegroundService(
                this,
                Intent(this, MonitorService::class.java).setAction(action)
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
        if (baseUrl != null) {
            startMonitorService(MonitorService.ACTION_UI_VISIBLE)
        }
    }

    override fun onStop() {
        super.onStop()
        RuntimeLog.info(TAG, "onStop exiting=$exiting")
        if (exiting || isChangingConfigurations) return
        if (MonitorScheduler.isInWindow(this)) {
            try {
                startService(
                    Intent(this, MonitorService::class.java)
                        .setAction(MonitorService.ACTION_UI_HIDDEN)
                )
            } catch (t: Throwable) {
                RuntimeLog.error(TAG, "send ui hidden failed", t)
            }
        } else {
            RuntimeLog.info(TAG, "ui hidden outside window, stop service")
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
        val code = intent.getStringExtra("stock_code")
        if (!code.isNullOrBlank()) {
            pendingStockCode = code
            if (baseUrl != null) loadTarget()
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
