package com.gotunnel.android.config

import android.content.Context
import com.gotunnel.android.bridge.TunnelStatus
import com.gotunnel.android.bridge.TunnelSnapshot

data class ServiceState(
    val status: TunnelStatus = TunnelStatus.STOPPED,
    val detail: String = "",
    val lastError: String = "",
    val recentLogs: String = "",
    val updatedAt: Long = 0L,
)

class ServiceStateStore(context: Context) {
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    fun load(): ServiceState {
        val statusName = prefs.getString(KEY_STATUS, TunnelStatus.STOPPED.name) ?: TunnelStatus.STOPPED.name
        val status = runCatching { TunnelStatus.valueOf(statusName) }.getOrDefault(TunnelStatus.STOPPED)

        return ServiceState(
            status = status,
            detail = prefs.getString(KEY_DETAIL, "") ?: "",
            lastError = prefs.getString(KEY_LAST_ERROR, "") ?: "",
            recentLogs = prefs.getString(KEY_RECENT_LOGS, "") ?: "",
            updatedAt = prefs.getLong(KEY_UPDATED_AT, 0L),
        )
    }

    fun save(snapshot: TunnelSnapshot) {
        prefs.edit()
            .putString(KEY_STATUS, snapshot.status.name)
            .putString(KEY_DETAIL, snapshot.detail)
            .putString(KEY_LAST_ERROR, snapshot.lastError)
            .putString(KEY_RECENT_LOGS, snapshot.recentLogs)
            .putLong(KEY_UPDATED_AT, System.currentTimeMillis())
            .apply()
    }

    companion object {
        private const val PREFS_NAME = "gotunnel_state"
        private const val KEY_STATUS = "status"
        private const val KEY_DETAIL = "detail"
        private const val KEY_LAST_ERROR = "last_error"
        private const val KEY_RECENT_LOGS = "recent_logs"
        private const val KEY_UPDATED_AT = "updated_at"
    }
}
