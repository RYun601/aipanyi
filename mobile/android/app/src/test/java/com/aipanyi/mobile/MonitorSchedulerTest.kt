package com.aipanyi.mobile

import org.junit.Assert.assertEquals
import org.junit.Test

class MonitorSchedulerTest {
    private val start = 10_000_000L
    private val end = start + 20_000_000

    @Test
    fun earlyAndLateAlarmsKeepTheCurrentWindow() {
        assertEquals(WindowAlarm.WAIT, MonitorScheduler.startDecision(start, start, end, start - 93_000))
        assertEquals(WindowAlarm.START, MonitorScheduler.startDecision(start, start, end, start))
        assertEquals(WindowAlarm.START, MonitorScheduler.startDecision(start, start, end, start + 2_817_000))
        assertEquals(WindowAlarm.RESCHEDULE, MonitorScheduler.startDecision(start, start, end, end))
    }

    @Test
    fun staleAndUnexpectedlyEarlyAlarmsDoNotStartService() {
        assertEquals(WindowAlarm.IGNORE, MonitorScheduler.startDecision(start - 1, start, end, start))
        assertEquals(WindowAlarm.RESCHEDULE, MonitorScheduler.startDecision(start, start, end, start - 120_001))
    }
}
