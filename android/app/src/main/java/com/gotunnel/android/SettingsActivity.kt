package com.gotunnel.android

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.os.PowerManager
import android.provider.Settings
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import com.gotunnel.android.config.AppConfig
import com.gotunnel.android.config.ConfigStore
import com.gotunnel.android.config.ServerEndpointParser
import com.gotunnel.android.databinding.ActivitySettingsBinding

class SettingsActivity : AppCompatActivity() {
    private lateinit var binding: ActivitySettingsBinding
    private lateinit var configStore: ConfigStore

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        binding = ActivitySettingsBinding.inflate(layoutInflater)
        setContentView(binding.root)

        configStore = ConfigStore(this)
        populateForm(configStore.load())

        binding.topToolbar.setNavigationOnClickListener {
            finish()
        }

        binding.aboutVersionValue.text = getString(
            R.string.about_version_format,
            BuildConfig.VERSION_NAME,
            BuildConfig.VERSION_CODE,
        )
        binding.aboutPackageValue.text = getString(
            R.string.about_package_format,
            packageName,
        )

        binding.saveButton.setOnClickListener {
            val validationError = validateForm()
            if (validationError != null) {
                Toast.makeText(this, validationError, Toast.LENGTH_SHORT).show()
                return@setOnClickListener
            }
            configStore.save(readForm())
            Toast.makeText(this, R.string.config_saved, Toast.LENGTH_SHORT).show()
            finish()
        }

        binding.batteryButton.setOnClickListener {
            openBatteryOptimizationSettings()
        }
    }

    private fun populateForm(config: AppConfig) {
        val endpoint = ServerEndpointParser.parse(config.serverAddress)
        binding.serverHostInput.setText(endpoint.host)
        binding.serverPortInput.setText(endpoint.port)
        binding.tokenInput.setText(config.token)
        binding.autoStartSwitch.isChecked = config.autoStart
        binding.autoReconnectSwitch.isChecked = config.autoReconnect
    }

    private fun readForm(): AppConfig {
        return AppConfig(
            serverAddress = ServerEndpointParser.compose(
                host = binding.serverHostInput.text?.toString().orEmpty(),
                port = binding.serverPortInput.text?.toString().orEmpty(),
            ),
            token = binding.tokenInput.text?.toString().orEmpty().trim(),
            autoStart = binding.autoStartSwitch.isChecked,
            autoReconnect = binding.autoReconnectSwitch.isChecked,
        )
    }

    private fun validateForm(): String? {
        val host = binding.serverHostInput.text?.toString().orEmpty().trim()
        if (host.isBlank()) {
            return getString(R.string.server_host_required)
        }

        val portText = binding.serverPortInput.text?.toString().orEmpty().trim()
        if (portText.isBlank()) {
            return getString(R.string.server_port_required)
        }

        val port = portText.toIntOrNull() ?: return getString(R.string.server_port_invalid)
        if (port !in 1..65535) {
            return getString(R.string.server_port_invalid)
        }

        if (binding.tokenInput.text?.toString().orEmpty().trim().isBlank()) {
            return getString(R.string.server_token_required)
        }

        return null
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
