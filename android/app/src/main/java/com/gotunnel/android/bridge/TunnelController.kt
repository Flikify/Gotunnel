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

interface TunnelController {
    interface Listener {
        fun onStatusChanged(status: TunnelStatus, detail: String = "")
        fun onLog(message: String)
    }

    val isRunning: Boolean

    fun setListener(listener: Listener?)
    fun updateConfig(config: AppConfig)
    fun start(config: AppConfig)
    fun stop(reason: String = "manual")
    fun restart(reason: String = "manual")
    fun getActiveTunnels(): List<ActiveTunnel>
}
