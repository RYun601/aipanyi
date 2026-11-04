package com.aipanyi.mobile

import java.net.HttpURLConnection
import java.net.Proxy
import java.net.URI

object CoreUrl {
    fun origin(url: String?): String? {
        val uri = runCatching { URI(url ?: "") }.getOrNull() ?: return null
        if (uri.scheme != "http" || uri.host != "127.0.0.1" || uri.port !in 1..65535 || uri.userInfo != null) {
            return null
        }
        return "http://127.0.0.1:${uri.port}"
    }

    fun shouldReload(currentUrl: String?, newUrl: String, failed: Boolean = false): Boolean =
        failed || origin(currentUrl) == null || origin(currentUrl) != origin(newUrl)

    fun reloadUrl(newUrl: String, currentUrl: String?): String {
        val base = newUrl.trimEnd('/')
        val route = currentUrl?.substringAfter('#', "")
        return if (route.isNullOrBlank()) "$base/" else "$base/#$route"
    }

    fun healthy(url: String): Boolean {
        val base = origin(url) ?: return false
        return runCatching {
            val connection = URI("$base/api/health").toURL().openConnection(Proxy.NO_PROXY) as HttpURLConnection
            try {
                connection.connectTimeout = 1500
                connection.readTimeout = 1500
                connection.instanceFollowRedirects = false
                connection.responseCode == HttpURLConnection.HTTP_OK
            } finally {
                connection.disconnect()
            }
        }.getOrDefault(false)
    }
}
