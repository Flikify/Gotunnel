package com.gotunnel.android.bridge

import com.gotunnel.android.config.AppConfig

enum class TunnelStatus {
    STOPPED,
    STARTING,
    RUNNING,
    RECONNECTING,
    ERROR,
}

data class ActiveTunnel(
    val name: String = "",
    val type: String = "",
    val remotePort: Int = 0,
    val localIP: String = "",
    val localPort: Int = 0,
    val status: String = "",
    val connectedAt: Long = 0L,
)

data class TunnelSnapshot(
    val isRunning: Boolean = false,
    val status: TunnelStatus = TunnelStatus.STOPPED,
    val detail: String = "",
    val lastError: String = "",
    val recentLogs: String = "",
    val activeTunnels: List<ActiveTunnel> = emptyList(),
)

interface TunnelController {
    interface Listener {
        fun onSnapshot(snapshot: TunnelSnapshot)
    }

    val isRunning: Boolean

    fun setListener(listener: Listener?)
    fun snapshot(): TunnelSnapshot
    fun updateConfig(config: AppConfig)
    fun start(config: AppConfig)
    fun stop(reason: String = "manual")
    fun restart(reason: String = "manual")
    fun appendHostLog(
        level: String = "info",
        eventCode: String,
        source: String,
        message: String,
        fieldsJson: String = "{}",
    )
}
