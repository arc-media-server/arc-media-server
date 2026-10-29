package org.arcmediaserver.tv.data

import android.util.Log

/**
 * The app's log, under one tag ("Arc") so `adb logcat -s Arc` shows everything. Outcomes and
 * failures only (CLAUDE.md, "Logging"): never the device token, PINs or full URLs.
 */
interface ArcLog {
    fun info(message: String)
    fun warn(message: String, error: Throwable? = null)
    fun debug(message: String)

    object Android : ArcLog {
        private const val TAG = "Arc"
        override fun info(message: String) {
            Log.i(TAG, message)
        }

        override fun warn(message: String, error: Throwable?) {
            if (error != null) Log.w(TAG, message, error) else Log.w(TAG, message)
        }

        override fun debug(message: String) {
            if (Log.isLoggable(TAG, Log.DEBUG)) Log.d(TAG, message)
        }
    }

    /** For tests on the JVM, where android.util.Log isn't there. */
    object Print : ArcLog {
        override fun info(message: String) = println("I/Arc: $message")
        override fun warn(message: String, error: Throwable?) = println("W/Arc: $message${error?.let { " ($it)" } ?: ""}")
        override fun debug(message: String) = Unit
    }
}
