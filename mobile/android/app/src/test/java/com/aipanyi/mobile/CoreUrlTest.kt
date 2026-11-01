package com.aipanyi.mobile

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class CoreUrlTest {
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
