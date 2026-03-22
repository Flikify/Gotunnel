package com.gotunnel.android

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Bundle
import android.os.PowerManager
import android.provider.Settings
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import com.gotunnel.android.bridge.TunnelStatus
import com.gotunnel.android.config.AppConfig
import com.gotunnel.android.config.ConfigStore
import com.gotunnel.android.config.ServiceStateStore
import com.gotunnel.android.databinding.ActivityMainBinding
import com.gotunnel.android.service.TunnelService
import java.text.DateFormat
import java.util.Date

class MainActivity : AppCompatActivity() {
    private lateinit var binding: ActivityMainBinding
    private lateinit var configStore: ConfigStore
    private lateinit var stateStore: ServiceStateStore
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

        populateForm(configStore.load())
        renderState()

        binding.saveButton.setOnClickListener {
            val config = readForm()
            configStore.save(config)
            renderState()
            Toast.makeText(this, R.string.config_saved, Toast.LENGTH_SHORT).show()
        }

        binding.startButton.setOnClickListener {
            val config = readForm()
            configStore.save(config)
            renderState()
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

        binding.batteryButton.setOnClickListener {
            openBatteryOptimizationSettings()
        }
    }

    override fun onResume() {
        super.onResume()
        renderState()
    }

    private fun populateForm(config: AppConfig) {
        binding.serverAddressInput.setText(config.serverAddress)
        binding.tokenInput.setText(config.token)
        binding.autoStartSwitch.isChecked = config.autoStart
        binding.autoReconnectSwitch.isChecked = config.autoReconnect
        binding.useTlsSwitch.isChecked = config.useTls
    }

    private fun readForm(): AppConfig {
        return AppConfig(
            serverAddress = binding.serverAddressInput.text?.toString().orEmpty().trim(),
            token = binding.tokenInput.text?.toString().orEmpty().trim(),
            autoStart = binding.autoStartSwitch.isChecked,
            autoReconnect = binding.autoReconnectSwitch.isChecked,
            useTls = binding.useTlsSwitch.isChecked,
        )
    }

    private fun renderState() {
        val state = stateStore.load()
        val timestamp = if (state.updatedAt > 0L) {
            DateFormat.getDateTimeInstance().format(Date(state.updatedAt))
        } else {
            getString(R.string.state_never_updated)
        }

        binding.stateValue.text = getString(
            R.string.state_format,
            state.status.name,
            state.detail.ifBlank { getString(R.string.state_no_detail) },
        )
        binding.stateMeta.text = getString(R.string.state_meta_format, timestamp)

        val hint = when (state.status) {
            TunnelStatus.RUNNING -> R.string.state_hint_running
            TunnelStatus.STARTING -> R.string.state_hint_starting
            TunnelStatus.RECONNECTING -> R.string.state_hint_reconnecting
            TunnelStatus.ERROR -> R.string.state_hint_error
            TunnelStatus.STOPPED -> R.string.state_hint_stopped
        }
        binding.stateHint.text = getString(hint)
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

    private fun openBatteryOptimizationSettings() {
        val powerManager = getSystemService(PowerManager::class.java)
        if (powerManager != null && powerManager.isIgnoringBatteryOptimizations(packageName)) {
            Toast.makeText(this, R.string.battery_optimization_already_disabled, Toast.LENGTH_SHORT).show()
            return
        }

        val intent = Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS).apply {
            data = Uri.parse("package:$packageName")
        }
        startActivity(intent)
    }
}
