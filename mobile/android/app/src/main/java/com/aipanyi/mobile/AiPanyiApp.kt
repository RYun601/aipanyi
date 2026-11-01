package com.aipanyi.mobile

import android.app.Application
import android.os.Process

class AiPanyiApp : Application() {
    override fun onCreate() {
        super.onCreate()
        RuntimeLog.init(this)
        RuntimeLog.info("AiPanyiApp", "process start pid=${Process.myPid()}")
    }
}
