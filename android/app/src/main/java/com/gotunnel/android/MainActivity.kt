package com.gotunnel.android

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Bundle
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import com.gotunnel.android.bridge.TunnelStatus
import com.gotunnel.android.config.ConfigStore
import com.gotunnel.android.config.LogStore
import com.gotunnel.android.config.ServiceStateStore
import com.gotunnel.android.databinding.ActivityMainBinding
import com.gotunnel.android.service.TunnelService
import java.text.DateFormat
import java.util.Date

class MainActivity : AppCompatActivity() {
    private lateinit var binding: ActivityMainBinding
    private lateinit var configStore: ConfigStore
    private lateinit var stateStore: ServiceStateStore
    private lateinit var logStore: LogStore

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

        binding.topToolbar.setNavigationOnClickListener {
            startActivity(Intent(this, SettingsActivity::class.java))
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
        binding.statusMeta.text = getString(R.string.state_meta_format, timestamp)
        binding.stateHint.text = getStateHint(state.status)
        binding.serverSummary.text = if (config.serverAddress.isBlank()) {
            getString(R.string.status_server_unconfigured)
        } else {
            getString(R.string.status_server_configured, config.serverAddress)
        }
        binding.logValue.text = logStore.render()
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
}
