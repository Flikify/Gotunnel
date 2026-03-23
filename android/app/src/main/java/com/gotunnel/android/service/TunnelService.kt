package com.gotunnel.android.service

import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import androidx.core.app.NotificationManagerCompat
import com.gotunnel.android.bridge.GoTunnelBridge
import com.gotunnel.android.bridge.TunnelController
import com.gotunnel.android.bridge.TunnelSnapshot
import com.gotunnel.android.bridge.TunnelStatus
import com.gotunnel.android.config.AppConfig
import com.gotunnel.android.config.ConfigStore
import com.gotunnel.android.config.ServiceStateStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel

class TunnelService : Service() {
    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private lateinit var configStore: ConfigStore
    private lateinit var stateStore: ServiceStateStore
    private lateinit var controller: TunnelController
    private lateinit var networkMonitor: NetworkMonitor
    private var currentConfig: AppConfig = AppConfig()
    private var networkMonitorPrimed = false

    override fun onCreate() {
        super.onCreate()
        configStore = ConfigStore(this)
        stateStore = ServiceStateStore(this)
        controller = GoTunnelBridge.create(applicationContext)
        controller.setListener(object : TunnelController.Listener {
            override fun onSnapshot(snapshot: TunnelSnapshot) {
                stateStore.save(snapshot)
                updateNotification(snapshot.status, snapshot.detail)
            }
        })
        networkMonitor = NetworkMonitor(
            this,
            onAvailable = {
                if (networkMonitorPrimed) {
                    networkMonitorPrimed = false
                } else {
                    val config = configStore.load()
                    if (config.autoReconnect && controller.isRunning) {
                        controller.restart("network-restored")
                    }
                }
            },
            onLost = {
                val detail = getString(com.gotunnel.android.R.string.network_lost)
                val state = stateStore.load()
                stateStore.save(
                    TunnelSnapshot(
                        isRunning = controller.isRunning,
                        status = TunnelStatus.RECONNECTING,
                        detail = detail,
                        lastError = state.lastError,
                        recentLogs = appendLog(state.recentLogs, detail),
                        activeTunnels = controller.snapshot().activeTunnels,
                    ),
                )
                updateNotification(TunnelStatus.RECONNECTING, detail)
            },
        )
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        ensureForeground()

        when (intent?.action) {
            ACTION_STOP -> {
                stopServiceInternal(intent.getStringExtra(EXTRA_REASON) ?: "stop")
                return START_NOT_STICKY
            }

            ACTION_RESTART -> {
                controller.restart(intent.getStringExtra(EXTRA_REASON) ?: "restart")
            }

            else -> {
                startOrRefreshTunnel(intent?.getStringExtra(EXTRA_REASON) ?: "start")
            }
        }

        return START_STICKY
    }

    override fun onDestroy() {
        runCatching { networkMonitor.stop() }
        runCatching { controller.stop("service-destroyed") }
        controller.setListener(null)
        serviceScope.cancel()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun ensureForeground() {
        val state = stateStore.load()
        val config = configStore.load()
        NotificationHelper.ensureChannel(this)
        startForeground(
            NotificationHelper.NOTIFICATION_ID,
            NotificationHelper.build(this, state.status, state.detail, config),
        )
    }

    private fun startOrRefreshTunnel(reason: String) {
        currentConfig = configStore.load()
        controller.updateConfig(currentConfig)
        stateStore.save(
            TunnelSnapshot(
                isRunning = controller.isRunning,
                status = TunnelStatus.STARTING,
                detail = reason,
                recentLogs = appendLog(stateStore.load().recentLogs, "start requested: $reason"),
                activeTunnels = controller.snapshot().activeTunnels,
            ),
        )
        updateNotification(TunnelStatus.STARTING, reason)

        if (!isConfigReady(currentConfig)) {
            val detail = getString(com.gotunnel.android.R.string.config_missing)
            stateStore.save(
                TunnelSnapshot(
                    isRunning = false,
                    status = TunnelStatus.STOPPED,
                    detail = detail,
                    recentLogs = appendLog(stateStore.load().recentLogs, detail),
                ),
            )
            updateNotification(TunnelStatus.STOPPED, detail)
            return
        }

        networkMonitorPrimed = networkMonitor.isConnected()
        controller.start(currentConfig)
        runCatching { networkMonitor.start() }
    }

    private fun stopServiceInternal(reason: String) {
        runCatching { networkMonitor.stop() }
        networkMonitorPrimed = false
        controller.stop(reason)
        stateStore.save(
            TunnelSnapshot(
                isRunning = false,
                status = TunnelStatus.STOPPED,
                detail = reason,
                recentLogs = appendLog(stateStore.load().recentLogs, "stop requested: $reason"),
            ),
        )
        updateNotification(TunnelStatus.STOPPED, reason)
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun updateNotification(status: TunnelStatus, detail: String) {
        val config = currentConfig.takeIf { it.serverAddress.isNotBlank() } ?: configStore.load()
        NotificationManagerCompat.from(this).notify(
            NotificationHelper.NOTIFICATION_ID,
            NotificationHelper.build(this, status, detail, config),
        )
    }

    private fun isConfigReady(config: AppConfig): Boolean {
        return config.serverAddress.isNotBlank() && config.token.isNotBlank()
    }

    private fun appendLog(existing: String, message: String): String {
        val lines = existing.lines().filter { it.isNotBlank() }.toMutableList()
        lines += message
        while (lines.size > 80) {
            lines.removeAt(0)
        }
        return lines.joinToString("\n")
    }

    companion object {
        const val ACTION_START = "com.gotunnel.android.service.action.START"
        const val ACTION_STOP = "com.gotunnel.android.service.action.STOP"
        const val ACTION_RESTART = "com.gotunnel.android.service.action.RESTART"
        const val EXTRA_REASON = "extra_reason"

        fun createStartIntent(context: Context, reason: String): Intent {
            return Intent(context, TunnelService::class.java).apply {
                action = ACTION_START
                putExtra(EXTRA_REASON, reason)
            }
        }

        fun createStopIntent(context: Context, reason: String): Intent {
            return Intent(context, TunnelService::class.java).apply {
                action = ACTION_STOP
                putExtra(EXTRA_REASON, reason)
            }
        }

        fun createRestartIntent(context: Context, reason: String): Intent {
            return Intent(context, TunnelService::class.java).apply {
                action = ACTION_RESTART
                putExtra(EXTRA_REASON, reason)
            }
        }
    }
}
