package com.aipanyi.mobile

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class CoreUrlTest {
    @Test
    fun staleNotificationNavigationDoesNotCompleteNewTarget() {
        val expected = "http://127.0.0.1:40213/#/stock/sh600000?eventId=42"
        assertFalse(CoreUrl.matchesNavigation(expected, "http://127.0.0.1:40213/#/stock/sh600519?eventId=41"))
        assertTrue(CoreUrl.matchesNavigation(expected, expected))
    }

    @Test
    fun notificationRouteTakesPrecedenceAfterRestart() {
        assertEquals(
            "http://127.0.0.1:40213/#/stock/sh600000?eventId=42",
            CoreUrl.reloadUrl("http://127.0.0.1:40213", "http://127.0.0.1:37229/#/watchlist", "/stock/sh600000?eventId=42")
        )
    }

    @Test
    fun portPrefixDoesNotMatchOrigin() {
        assertTrue(CoreUrl.shouldReload("http://127.0.0.1:40401/#/watchlist", "http://127.0.0.1:4040"))
    }

    @Test
    fun sameOriginReloadsAfterNavigationFailure() {
        assertTrue(CoreUrl.shouldReload("http://127.0.0.1:4040/#/watchlist", "http://127.0.0.1:4040", true))
    }

    @Test
    fun sameCoreUrlDoesNotReload() {
        assertFalse(CoreUrl.shouldReload("http://127.0.0.1:40000/#/watchlist", "http://127.0.0.1:40000"))
    }

    @Test
    fun changedCoreUrlReloads() {
        assertTrue(CoreUrl.shouldReload("http://127.0.0.1:37229/#/watchlist", "http://127.0.0.1:40213"))
    }

    @Test
    fun emptyCurrentUrlReloads() {
        assertTrue(CoreUrl.shouldReload(null, "http://127.0.0.1:40213"))
    }

    @Test
    fun reloadPreservesHashRoute() {
        assertEquals(
            "http://127.0.0.1:40213/#/stock/sh600519",
            CoreUrl.reloadUrl("http://127.0.0.1:40213", "http://127.0.0.1:37229/#/stock/sh600519")
        )
    }

    @Test
    fun reloadWithoutRouteUsesRoot() {
        assertEquals(
            "http://127.0.0.1:40213/",
            CoreUrl.reloadUrl("http://127.0.0.1:40213", "http://127.0.0.1:37229")
        )
    }
}
