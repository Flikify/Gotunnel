package com.gotunnel.android.config

import android.content.Context
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

class LogStore(context: Context) {
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    fun append(message: String) {
        if (message.isBlank()) {
            return
        }

        val current = load().toMutableList()
        current += "${timestamp()}  $message"
        while (current.size > MAX_LINES) {
            current.removeAt(0)
        }

        prefs.edit().putString(KEY_LOGS, current.joinToString(SEPARATOR)).apply()
    }

    fun load(): List<String> {
        val raw = prefs.getString(KEY_LOGS, "") ?: ""
        if (raw.isBlank()) {
            return emptyList()
        }
        return raw.split(SEPARATOR).filter { it.isNotBlank() }
    }

    fun render(): String {
        val lines = load()
        return if (lines.isEmpty()) {
            "No logs yet."
        } else {
            lines.joinToString("\n")
        }
    }

    private fun timestamp(): String {
        return SimpleDateFormat("HH:mm:ss", Locale.getDefault()).format(Date())
    }

    companion object {
        private const val PREFS_NAME = "gotunnel_logs"
        private const val KEY_LOGS = "logs"
        private const val MAX_LINES = 80
        private const val SEPARATOR = "\u0001"
    }
}
