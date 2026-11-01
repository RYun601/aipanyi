package com.aipanyi.mobile

/**
 * 本地 Go core 会用随机端口重启；WebView 必须知道 base URL 是否变化，
 * 否则旧页面继续 fetch 旧端口，所有 /api/call 都会 failed to fetch。
 */
object CoreUrl {
    fun shouldReload(currentUrl: String?, newUrl: String): Boolean {
        if (currentUrl.isNullOrBlank()) return true
        return !currentUrl.startsWith(newUrl.trimEnd('/'))
    }

    fun reloadUrl(newUrl: String, currentUrl: String?): String {
        val base = newUrl.trimEnd('/')
        val route = currentUrl?.substringAfter('#', "")
        return if (route.isNullOrBlank()) "$base/" else "$base/#$route"
    }
}
