package com.gotunnel.android.bridge

import android.content.Context
import com.gotunnel.android.config.AppConfig
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

class StubTunnelController(
    @Suppress("unused") private val context: Context,
) : TunnelController {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var listener: TunnelController.Listener? = null
    private var config: AppConfig = AppConfig()
    private var job: Job? = null

    override val isRunning: Boolean
        get() = job?.isActive == true

    override fun setListener(listener: TunnelController.Listener?) {
        this.listener = listener
    }

    override fun updateConfig(config: AppConfig) {
        this.config = config
    }

    override fun start(config: AppConfig) {
        updateConfig(config)
        if (isRunning) {
            listener?.onLog("Stub tunnel already running")
            return
        }

        job = scope.launch {
            listener?.onStatusChanged(TunnelStatus.STARTING, "Preparing tunnel session")
            delay(400)
            listener?.onLog("Stub tunnel prepared for ${config.serverAddress}")
            listener?.onStatusChanged(TunnelStatus.RUNNING, "Waiting for native Go core")

            while (isActive) {
                delay(30_000)
                listener?.onLog("Stub keepalive tick for ${this@StubTunnelController.config.serverAddress}")
            }
        }
    }

    override fun stop(reason: String) {
        listener?.onLog("Stub tunnel stop requested: $reason")
        job?.cancel()
        job = null
        listener?.onStatusChanged(TunnelStatus.STOPPED, reason)
    }

    override fun restart(reason: String) {
        listener?.onStatusChanged(TunnelStatus.RECONNECTING, reason)
        stop(reason)
        start(config)
    }
}
