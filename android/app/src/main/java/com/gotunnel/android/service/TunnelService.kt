package com.gotunnel.android.service

import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.IBinder
import androidx.core.app.NotificationManagerCompat
import com.gotunnel.android.bridge.GoTunnelBridge
import com.gotunnel.android.bridge.TunnelController
import com.gotunnel.android.bridge.TunnelStatus
import com.gotunnel.android.config.AppConfig
import com.gotunnel.android.config.ConfigStore
import com.gotunnel.android.config.LogStore
import com.gotunnel.android.config.ServiceStateStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel

class TunnelService : Service() {
    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private lateinit var configStore: ConfigStore
    private lateinit var stateStore: ServiceStateStore
    private lateinit var logStore: LogStore
    private lateinit var controller: TunnelController
    private lateinit var networkMonitor: NetworkMonitor
    private var currentConfig: AppConfig = AppConfig()
    private var networkMonitorPrimed = false

    override fun onCreate() {
        super.onCreate()
        configStore = ConfigStore(this)
        stateStore = ServiceStateStore(this)
        logStore = LogStore(this)
        controller = GoTunnelBridge.create(applicationContext)
        controller.setListener(object : TunnelController.Listener {
            override fun onStatusChanged(status: TunnelStatus, detail: String) {
                stateStore.save(status, detail)
                logStore.append("status: ${status.name} ${detail.ifBlank { "" }}".trim())
                updateNotification(status, detail)
            }

            override fun onLog(message: String) {
                val current = stateStore.load()
                logStore.append(message)
                updateNotification(current.status, message)
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
                stateStore.save(TunnelStatus.RECONNECTING, detail)
                logStore.append(detail)
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
        stateStore.save(TunnelStatus.STARTING, reason)
        logStore.append("start requested: $reason")
        updateNotification(TunnelStatus.STARTING, reason)

        if (!isConfigReady(currentConfig)) {
            val detail = getString(com.gotunnel.android.R.string.config_missing)
            stateStore.save(TunnelStatus.STOPPED, detail)
            logStore.append(detail)
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
        stateStore.save(TunnelStatus.STOPPED, reason)
        logStore.append("stop requested: $reason")
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
