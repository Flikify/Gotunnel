package com.gotunnel.android

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Typeface
import android.os.Bundle
import android.view.View
import android.view.ViewGroup
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.annotation.IdRes
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import com.google.android.material.card.MaterialCardView
import com.gotunnel.android.bridge.ActiveTunnel
import com.gotunnel.android.bridge.GoTunnelBridge
import com.gotunnel.android.bridge.TunnelStatus
import com.gotunnel.android.config.ConfigStore
import com.gotunnel.android.config.LogStore
import com.gotunnel.android.config.ServiceStateStore
import com.gotunnel.android.databinding.ActivityMainBinding
import com.gotunnel.android.service.TunnelService
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.text.DateFormat
import java.util.Date

class MainActivity : AppCompatActivity() {
    private lateinit var binding: ActivityMainBinding
    private val uiScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var refreshJob: Job? = null
    private lateinit var configStore: ConfigStore
    private lateinit var stateStore: ServiceStateStore
    private lateinit var logStore: LogStore
    private lateinit var tunnelController: com.gotunnel.android.bridge.TunnelController

    private val notificationPermissionLauncher =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
            if (!granted) {
                Toast.makeText(this, R.string.notification_permission_denied, Toast.LENGTH_SHORT).show()
            }
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        requestNotificationPermissionIfNeeded()

        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        configStore = ConfigStore(this)
        stateStore = ServiceStateStore(this)
        logStore = LogStore(this)
        tunnelController = GoTunnelBridge.create(applicationContext)

        binding.topToolbar.setOnMenuItemClickListener { item ->
            handleToolbarAction(item.itemId)
        }

        binding.startButton.setOnClickListener {
            val config = configStore.load()
            if (config.serverAddress.isBlank() || config.token.isBlank()) {
                Toast.makeText(this, R.string.config_missing, Toast.LENGTH_SHORT).show()
                startActivity(Intent(this, SettingsActivity::class.java))
                return@setOnClickListener
            }

            ContextCompat.startForegroundService(
                this,
                TunnelService.createStartIntent(this, "manual-start"),
            )
            Toast.makeText(this, R.string.service_start_requested, Toast.LENGTH_SHORT).show()
        }

        binding.stopButton.setOnClickListener {
            ContextCompat.startForegroundService(
                this,
                TunnelService.createStopIntent(this, "manual-stop"),
            )
            Toast.makeText(this, R.string.service_stop_requested, Toast.LENGTH_SHORT).show()
        }
    }

    override fun onResume() {
        super.onResume()
        renderScreen()
    }

    override fun onStart() {
        super.onStart()
        startRefreshLoop()
    }

    override fun onStop() {
        refreshJob?.cancel()
        refreshJob = null
        super.onStop()
    }

    override fun onDestroy() {
        uiScope.cancel()
        super.onDestroy()
    }

    private fun renderScreen() {
        val config = configStore.load()
        val state = stateStore.load()
        val timestamp = if (state.updatedAt > 0L) {
            DateFormat.getDateTimeInstance().format(Date(state.updatedAt))
        } else {
            getString(R.string.state_never_updated)
        }

        binding.statusValue.text = getStatusLabel(state.status)
        binding.statusDetail.text = state.detail.ifBlank { getString(R.string.state_no_detail) }
        binding.stateMeta.text = getString(R.string.state_meta_format, timestamp)
        binding.stateHint.text = getStateHint(state.status)
        binding.serverSummary.text = if (config.serverAddress.isBlank()) {
            getString(R.string.status_server_unconfigured)
        } else {
            getString(R.string.status_server_configured, config.serverAddress)
        }
        binding.logValue.text = logStore.render()
        renderActiveTunnels(tunnelController.getActiveTunnels())
    }

    private fun getStatusLabel(status: TunnelStatus): String {
        return when (status) {
            TunnelStatus.RUNNING -> getString(R.string.status_running)
            TunnelStatus.STARTING -> getString(R.string.status_starting)
            TunnelStatus.RECONNECTING -> getString(R.string.status_reconnecting)
            TunnelStatus.ERROR -> getString(R.string.status_error)
            TunnelStatus.STOPPED -> getString(R.string.status_stopped)
        }
    }

    private fun getStateHint(status: TunnelStatus): String {
        val messageId = when (status) {
            TunnelStatus.RUNNING -> R.string.state_hint_running
            TunnelStatus.STARTING -> R.string.state_hint_starting
            TunnelStatus.RECONNECTING -> R.string.state_hint_reconnecting
            TunnelStatus.ERROR -> R.string.state_hint_error
            TunnelStatus.STOPPED -> R.string.state_hint_stopped
        }
        return getString(messageId)
    }

    private fun requestNotificationPermissionIfNeeded() {
        if (android.os.Build.VERSION.SDK_INT < android.os.Build.VERSION_CODES.TIRAMISU) {
            return
        }

        val granted = ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.POST_NOTIFICATIONS,
        ) == PackageManager.PERMISSION_GRANTED
        if (!granted) {
            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
    }

    private fun startRefreshLoop() {
        refreshJob?.cancel()
        refreshJob = uiScope.launch {
            while (isActive) {
                renderScreen()
                delay(1_000L)
            }
        }
    }

    private fun renderActiveTunnels(tunnels: List<ActiveTunnel>) {
        binding.activeTunnelList.removeAllViews()
        binding.activeTunnelEmpty.visibility = if (tunnels.isEmpty()) View.VISIBLE else View.GONE

        tunnels.forEachIndexed { index, tunnel ->
            binding.activeTunnelList.addView(buildTunnelItemView(tunnel, index > 0))
        }
    }

    private fun buildTunnelItemView(tunnel: ActiveTunnel, addTopMargin: Boolean): View {
        val card = MaterialCardView(this).apply {
            radius = dp(18).toFloat()
            strokeWidth = dp(1)
            setCardBackgroundColor(ContextCompat.getColor(context, R.color.gotunnel_background))
            strokeColor = ContextCompat.getColor(context, R.color.gotunnel_border)
            layoutParams = LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT,
                ViewGroup.LayoutParams.WRAP_CONTENT,
            ).apply {
                if (addTopMargin) {
                    topMargin = dp(12)
                }
            }
        }

        val container = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(16), dp(16), dp(16), dp(16))
        }

        val title = TextView(this).apply {
            text = tunnel.name.ifBlank {
                tunnel.type.uppercase().ifBlank { getString(R.string.active_tunnel_title_fallback) }
            }
            setTextColor(ContextCompat.getColor(context, R.color.gotunnel_text))
            textSize = 16f
            setTypeface(typeface, Typeface.BOLD)
        }

        val serverPort = TextView(this).apply {
            text = getString(R.string.active_tunnel_server_port, formatPort(tunnel.remotePort))
            setTextColor(ContextCompat.getColor(context, R.color.gotunnel_text))
            textSize = 14f
            setPadding(0, dp(10), 0, 0)
        }

        val clientPort = TextView(this).apply {
            text = getString(R.string.active_tunnel_client_port, formatPort(tunnel.localPort))
            setTextColor(ContextCompat.getColor(context, R.color.gotunnel_text))
            textSize = 14f
            setPadding(0, dp(4), 0, 0)
        }

        val establishedAt = TextView(this).apply {
            text = getString(R.string.active_tunnel_established_at, formatEstablishedAt(tunnel.connectedAt))
            setTextColor(ContextCompat.getColor(context, R.color.gotunnel_text_muted))
            textSize = 13f
            setPadding(0, dp(8), 0, 0)
        }

        val duration = TextView(this).apply {
            text = getString(R.string.active_tunnel_duration, formatDuration(tunnel.connectedAt))
            setTextColor(ContextCompat.getColor(context, R.color.gotunnel_text_muted))
            textSize = 13f
            setPadding(0, dp(4), 0, 0)
        }

        container.addView(title)
        container.addView(serverPort)
        container.addView(clientPort)
        container.addView(establishedAt)
        container.addView(duration)
        card.addView(container)
        return card
    }

    private fun formatEstablishedAt(timestamp: Long): String {
        if (timestamp <= 0L) {
            return getString(R.string.state_never_updated)
        }
        return DateFormat.getDateTimeInstance(DateFormat.SHORT, DateFormat.MEDIUM).format(Date(timestamp))
    }

    private fun formatDuration(connectedAt: Long): String {
        if (connectedAt <= 0L) {
            return "00:00:00"
        }

        val totalSeconds = ((System.currentTimeMillis() - connectedAt).coerceAtLeast(0L)) / 1_000L
        val hours = totalSeconds / 3_600L
        val minutes = (totalSeconds % 3_600L) / 60L
        val seconds = totalSeconds % 60L
        return String.format("%02d:%02d:%02d", hours, minutes, seconds)
    }

    private fun formatPort(port: Int): String {
        return if (port > 0) {
            port.toString()
        } else {
            getString(R.string.active_tunnel_unknown_port)
        }
    }

    private fun dp(value: Int): Int = (value * resources.displayMetrics.density).toInt()

    private fun handleToolbarAction(@IdRes itemId: Int): Boolean {
        return when (itemId) {
            R.id.action_settings -> {
                startActivity(Intent(this, SettingsActivity::class.java))
                true
            }

            else -> false
        }
    }
}
