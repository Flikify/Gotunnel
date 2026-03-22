package com.gotunnel.android.config

import android.content.Context
import com.gotunnel.android.bridge.TunnelStatus

data class ServiceState(
    val status: TunnelStatus = TunnelStatus.STOPPED,
    val detail: String = "",
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
            updatedAt = prefs.getLong(KEY_UPDATED_AT, 0L),
        )
    }

    fun save(status: TunnelStatus, detail: String) {
        prefs.edit()
            .putString(KEY_STATUS, status.name)
            .putString(KEY_DETAIL, detail)
            .putLong(KEY_UPDATED_AT, System.currentTimeMillis())
            .apply()
    }

    companion object {
        private const val PREFS_NAME = "gotunnel_state"
        private const val KEY_STATUS = "status"
        private const val KEY_DETAIL = "detail"
        private const val KEY_UPDATED_AT = "updated_at"
    }
}
