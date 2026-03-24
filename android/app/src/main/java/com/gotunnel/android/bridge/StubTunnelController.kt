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
    private var currentSnapshot: TunnelSnapshot = TunnelSnapshot()

    override val isRunning: Boolean
        get() = job?.isActive == true

    override fun setListener(listener: TunnelController.Listener?) {
        this.listener = listener
        listener?.onSnapshot(currentSnapshot)
    }

    override fun snapshot(): TunnelSnapshot {
        return currentSnapshot
    }

    override fun updateConfig(config: AppConfig) {
        this.config = config
    }

    override fun start(config: AppConfig) {
        updateConfig(config)
        if (isRunning) {
            listener?.onSnapshot(currentSnapshot)
            return
        }

        job = scope.launch {
            emitSnapshot(
                currentSnapshot.copy(
                    isRunning = true,
                    status = TunnelStatus.STARTING,
                    detail = "Preparing tunnel session",
                ),
            )
            delay(400)
            emitSnapshot(
                currentSnapshot.copy(
                    isRunning = true,
                    status = TunnelStatus.RUNNING,
                    detail = "Waiting for native Go core",
                    recentLogs = appendLog(currentSnapshot.recentLogs, "Stub tunnel prepared for ${config.serverAddress}"),
                ),
            )

            while (isActive) {
                delay(30_000)
                emitSnapshot(
                    currentSnapshot.copy(
                        recentLogs = appendLog(currentSnapshot.recentLogs, "Stub keepalive tick for ${this@StubTunnelController.config.serverAddress}"),
                    ),
                )
            }
        }
    }

    override fun stop(reason: String) {
        job?.cancel()
        job = null
        emitSnapshot(
            currentSnapshot.copy(
                isRunning = false,
                status = TunnelStatus.STOPPED,
                detail = reason,
                recentLogs = appendLog(currentSnapshot.recentLogs, "Stub tunnel stop requested: $reason"),
            ),
        )
    }

    override fun restart(reason: String) {
        emitSnapshot(
            currentSnapshot.copy(
                status = TunnelStatus.RECONNECTING,
                detail = reason,
            ),
        )
        stop(reason)
        start(config)
    }

    override fun appendHostLog(level: String, eventCode: String, source: String, message: String, fieldsJson: String) {
        emitSnapshot(
            currentSnapshot.copy(
                recentLogs = appendLog(currentSnapshot.recentLogs, "[$level][$eventCode][$source] $message"),
            ),
        )
    }

    private fun emitSnapshot(snapshot: TunnelSnapshot) {
        currentSnapshot = snapshot
        listener?.onSnapshot(snapshot)
    }

    private fun appendLog(existing: String, message: String): String {
        val lines = existing.lines().filter { it.isNotBlank() }.toMutableList()
        lines += message
        while (lines.size > 80) {
            lines.removeAt(0)
        }
        return lines.joinToString("\n")
    }
}
