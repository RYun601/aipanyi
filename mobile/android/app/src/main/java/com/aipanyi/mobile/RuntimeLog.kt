package com.aipanyi.mobile

import android.content.Context
import android.util.Log
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/** 应用私有运行日志，文件位置：filesDir/aipanyi/logs/runtime.log。 */
object RuntimeLog {
    private const val TAG = "AiPanyiRuntime"
    private const val MAX_BYTES = 512 * 1024L
    private const val KEEP_FILES = 3

    @Volatile
    private var dir: File? = null
    private val lock = Any()

    fun init(context: Context) {
        if (dir != null) return
        synchronized(lock) {
            if (dir == null) {
                dir = File(context.filesDir, "aipanyi/logs").apply { mkdirs() }
            }
        }
    }

    fun info(tag: String, message: String) = write("I", tag, message, null)
    fun warn(tag: String, message: String, tr: Throwable? = null) = write("W", tag, message, tr)
    fun error(tag: String, message: String, tr: Throwable? = null) = write("E", tag, message, tr)

    private fun write(level: String, tag: String, message: String, tr: Throwable?) {
        val time = SimpleDateFormat("yyyy-MM-dd HH:mm:ss.SSS", Locale.US).format(Date())
        val line = buildString {
            append(time).append(' ').append(level).append('/').append(tag).append(' ').append(message)
            if (tr != null) {
                append(": ").append(tr.javaClass.simpleName).append(' ').append(tr.message ?: "")
            }
        }
        when (level) {
            "E" -> Log.e(TAG, line, tr)
            "W" -> Log.w(TAG, line, tr)
            else -> Log.i(TAG, line)
        }
        val logDir = dir ?: return
        synchronized(lock) {
            try {
                val file = File(logDir, "runtime.log")
                if (file.length() >= MAX_BYTES) rotate(logDir)
                file.appendText(line + "\n")
            } catch (_: Throwable) {
                // 日志失败不能影响业务。
            }
        }
    }

    private fun rotate(logDir: File) {
        File(logDir, "runtime.log.${KEEP_FILES - 1}").delete()
        for (i in KEEP_FILES - 2 downTo 1) {
            val from = File(logDir, "runtime.log.$i")
            if (from.exists()) from.renameTo(File(logDir, "runtime.log.${i + 1}"))
        }
        File(logDir, "runtime.log").renameTo(File(logDir, "runtime.log.1"))
    }
}
