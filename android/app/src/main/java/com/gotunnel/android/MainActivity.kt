package com.gotunnel.android

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.res.ColorStateList
import android.graphics.Color
import android.net.Uri
import android.graphics.Typeface
import android.os.Bundle
import android.provider.Settings
import android.view.View
import android.view.ViewGroup
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import com.google.android.material.card.MaterialCardView
import com.google.android.material.button.MaterialButton
import com.gotunnel.android.bridge.ActiveTunnel
import com.gotunnel.android.bridge.GoTunnelBridge
import com.gotunnel.android.bridge.TunnelController
import com.gotunnel.android.bridge.TunnelSnapshot
import com.gotunnel.android.bridge.TunnelStatus
import com.gotunnel.android.config.ConfigStore
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
    private var tunnelController: com.gotunnel.android.bridge.TunnelController? = null

    private val notificationPermissionLauncher =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
            recordNotificationPermissionPrompt()
            if (granted) {
                startTunnelService()
            } else {
                Toast.makeText(this, R.string.notification_permission_denied, Toast.LENGTH_SHORT).show()
            }
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        val initialized = runCatching {
            binding = ActivityMainBinding.inflate(layoutInflater)
            setContentView(binding.root)

            configStore = ConfigStore(this)
            stateStore = ServiceStateStore(this)
            tunnelController = runCatching { GoTunnelBridge.create(applicationContext) }.getOrElse { error ->
                buildUnavailableTunnelController(error)
            }

            binding.settingsButton.setOnClickListener {
                startActivity(Intent(this, SettingsActivity::class.java))
            }
            binding.actionButton.setOnClickListener {
                handlePrimaryAction()
            }
        }

        initialized.onFailure { error ->
            renderFatalStartupError(error)
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
        if (!::binding.isInitialized || !::configStore.isInitialized || !::stateStore.isInitialized) {
            return
        }
        val config = configStore.load()
        val state = stateStore.load()
        val runtimeSnapshot = tunnelController?.snapshot() ?: TunnelSnapshot()
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
        binding.logValue.text = state.recentLogs.ifBlank { "No logs yet." }
        renderPrimaryAction(state.status)
        renderActiveTunnels(runtimeSnapshot.activeTunnels)
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

    private fun requestNotificationPermissionIfNeededForStart(): Boolean {
        if (android.os.Build.VERSION.SDK_INT < android.os.Build.VERSION_CODES.TIRAMISU) {
            return true
        }

        val granted = ContextCompat.checkSelfPermission(
            this,
            Manifest.permission.POST_NOTIFICATIONS,
        ) == PackageManager.PERMISSION_GRANTED
        if (granted) {
            return true
        }

        if (!hasRequestedNotificationPermission()) {
            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
            return false
        }

        if (shouldShowRequestPermissionRationale(Manifest.permission.POST_NOTIFICATIONS)) {
            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
            return false
        }

        Toast.makeText(this, R.string.notification_permission_settings_required, Toast.LENGTH_LONG).show()
        openAppNotificationSettings()
        return false
    }

    private fun handlePrimaryAction() {
        if (!::configStore.isInitialized || !::stateStore.isInitialized) {
            Toast.makeText(this, R.string.startup_error_title, Toast.LENGTH_LONG).show()
            return
        }

        val state = stateStore.load()
        if (state.status.isActive()) {
            ContextCompat.startForegroundService(
                this,
                TunnelService.createStopIntent(this, "manual-stop"),
            )
            Toast.makeText(this, R.string.service_stop_requested, Toast.LENGTH_SHORT).show()
            return
        }

        val config = configStore.load()
        if (config.serverAddress.isBlank() || config.token.isBlank()) {
            Toast.makeText(this, R.string.config_missing, Toast.LENGTH_SHORT).show()
            startActivity(Intent(this, SettingsActivity::class.java))
            return
        }

        if (!requestNotificationPermissionIfNeededForStart()) {
            return
        }

        startTunnelService()
    }

    private fun startTunnelService() {
        ContextCompat.startForegroundService(
            this,
            TunnelService.createStartIntent(this, "manual-start"),
        )
        Toast.makeText(this, R.string.service_start_requested, Toast.LENGTH_SHORT).show()
    }

    private fun hasRequestedNotificationPermission(): Boolean {
        return getSharedPreferences(PERMISSION_PREFS_NAME, Context.MODE_PRIVATE)
            .getBoolean(KEY_NOTIFICATION_PERMISSION_REQUESTED, false)
    }

    private fun recordNotificationPermissionPrompt() {
        getSharedPreferences(PERMISSION_PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .putBoolean(KEY_NOTIFICATION_PERMISSION_REQUESTED, true)
            .apply()
    }

    private fun openAppNotificationSettings() {
        val intent = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
            data = Uri.parse("package:$packageName")
        }
        startActivity(intent)
    }

    private fun buildUnavailableTunnelController(error: Throwable): TunnelController {
        val message = error.message ?: error::class.java.simpleName
        return object : TunnelController {
            private val snapshot = TunnelSnapshot(
                isRunning = false,
                status = TunnelStatus.ERROR,
                detail = message,
                lastError = message,
                recentLogs = "Startup failed: $message",
            )

            override val isRunning: Boolean = false

            override fun setListener(listener: TunnelController.Listener?) {
                listener?.onSnapshot(snapshot)
            }

            override fun snapshot(): TunnelSnapshot = snapshot

            override fun updateConfig(config: com.gotunnel.android.config.AppConfig) = Unit

            override fun start(config: com.gotunnel.android.config.AppConfig) = Unit

            override fun stop(reason: String) = Unit

            override fun restart(reason: String) = Unit

            override fun appendHostLog(
                level: String,
                eventCode: String,
                source: String,
                message: String,
                fieldsJson: String,
            ) = Unit
        }
    }

    private fun renderFatalStartupError(error: Throwable) {
        val container = ScrollView(this).apply {
            setBackgroundColor(Color.WHITE)
        }
        val content = TextView(this).apply {
            val message = buildString {
                append(getString(R.string.startup_error_title))
                append("\n\n")
                append(error.message ?: error::class.java.name)
            }
            text = message
            setTextColor(ContextCompat.getColor(context, R.color.gotunnel_text))
            textSize = 16f
            setPadding(dp(24), dp(32), dp(24), dp(32))
        }
        container.addView(content)
        setContentView(container)
        Toast.makeText(this, error.message ?: getString(R.string.startup_error_title), Toast.LENGTH_LONG).show()
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

    private fun renderPrimaryAction(status: TunnelStatus) {
        val isActive = status.isActive()
        val button = binding.actionButton
        button.text = if (isActive) {
            getString(R.string.stop_button)
        } else {
            getString(R.string.start_button)
        }
        button.icon = ContextCompat.getDrawable(
            this,
            if (isActive) R.drawable.ic_stop_circle else R.drawable.ic_play_circle,
        )
        button.iconGravity = MaterialButton.ICON_GRAVITY_TEXT_START
        button.iconPadding = dp(10)
        button.backgroundTintList = ColorStateList.valueOf(
            ContextCompat.getColor(
                this,
                if (isActive) R.color.gotunnel_surface_alt else R.color.gotunnel_primary,
            ),
        )
        button.setTextColor(
            ContextCompat.getColor(
                this,
                if (isActive) R.color.gotunnel_text else android.R.color.white,
            ),
        )
        button.strokeWidth = if (isActive) dp(1) else 0
        button.strokeColor = ColorStateList.valueOf(ContextCompat.getColor(this, R.color.gotunnel_border))
    }

    private fun buildTunnelItemView(tunnel: ActiveTunnel, addTopMargin: Boolean): View {
        val card = MaterialCardView(this).apply {
            radius = dp(18).toFloat()
            strokeWidth = dp(1)
            setCardBackgroundColor(ContextCompat.getColor(context, R.color.gotunnel_surface_alt))
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

    private fun TunnelStatus.isActive(): Boolean {
        return this == TunnelStatus.RUNNING ||
            this == TunnelStatus.STARTING ||
            this == TunnelStatus.RECONNECTING
    }

    companion object {
        private const val PERMISSION_PREFS_NAME = "gotunnel_permissions"
        private const val KEY_NOTIFICATION_PERMISSION_REQUESTED = "notification_permission_requested"
    }
}
