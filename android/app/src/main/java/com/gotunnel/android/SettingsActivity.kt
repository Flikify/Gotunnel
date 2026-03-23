package com.gotunnel.android

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.os.Build
import android.os.PowerManager
import android.provider.Settings
import android.text.format.DateUtils
import android.text.format.Formatter
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import com.gotunnel.android.config.AppConfig
import com.gotunnel.android.config.ConfigStore
import com.gotunnel.android.config.ServerEndpointParser
import com.gotunnel.android.databinding.ActivitySettingsBinding
import com.gotunnel.android.update.AndroidReleaseUpdate
import com.gotunnel.android.update.ReleaseUpdater
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import java.io.File
import java.text.DateFormat
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

class SettingsActivity : AppCompatActivity() {
    private lateinit var binding: ActivitySettingsBinding
    private lateinit var configStore: ConfigStore
    private val uiScope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var updateJob: Job? = null
    private var availableUpdate: AndroidReleaseUpdate? = null
    private var downloadedUpdateFile: File? = null

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
        renderUpdateState()

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

        binding.checkUpdateButton.setOnClickListener {
            checkForUpdates(manual = true)
        }

        binding.installUpdateButton.setOnClickListener {
            downloadAndInstallUpdate()
        }

        checkForUpdates(manual = false)
    }

    override fun onDestroy() {
        updateJob?.cancel()
        uiScope.cancel()
        super.onDestroy()
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

    private fun checkForUpdates(manual: Boolean) {
        updateJob?.cancel()
        binding.aboutUpdateStatusValue.text = getString(R.string.update_status_checking)
        binding.aboutUpdateSummaryValue.text = getString(R.string.update_summary_checking)
        binding.checkUpdateButton.isEnabled = false
        binding.installUpdateButton.isEnabled = false

        updateJob = uiScope.launch {
            val result = runCatching {
                ReleaseUpdater.checkForUpdate(BuildConfig.VERSION_NAME)
            }

            result.onSuccess { update ->
                availableUpdate = update
                downloadedUpdateFile = null
                renderUpdateState()
                if (manual) {
                    val messageId = if (update.isUpdateAvailable) {
                        R.string.update_toast_available
                    } else {
                        R.string.update_toast_latest
                    }
                    Toast.makeText(this@SettingsActivity, getString(messageId), Toast.LENGTH_SHORT).show()
                }
            }.onFailure { error ->
                availableUpdate = null
                downloadedUpdateFile = null
                binding.aboutUpdateStatusValue.text = getString(R.string.update_status_failed)
                binding.aboutUpdateSummaryValue.text = error.message ?: getString(R.string.update_summary_failed)
                binding.checkUpdateButton.isEnabled = true
                binding.installUpdateButton.visibility = android.view.View.GONE
                if (manual) {
                    Toast.makeText(
                        this@SettingsActivity,
                        error.message ?: getString(R.string.update_summary_failed),
                        Toast.LENGTH_LONG,
                    ).show()
                }
            }
        }
    }

    private fun downloadAndInstallUpdate() {
        val update = availableUpdate
        if (update == null || !update.isUpdateAvailable) {
            Toast.makeText(this, R.string.update_toast_latest, Toast.LENGTH_SHORT).show()
            return
        }
        if (!update.isInstallable || !update.hasDownloadAsset) {
            Toast.makeText(this, R.string.update_summary_no_installable_asset, Toast.LENGTH_LONG).show()
            return
        }
        if (!ReleaseUpdater.canRequestPackageInstalls(this)) {
            ReleaseUpdater.openUnknownAppsSettings(this)
            Toast.makeText(this, R.string.update_permission_needed, Toast.LENGTH_LONG).show()
            return
        }

        val existingFile = downloadedUpdateFile
        if (existingFile?.exists() == true) {
            ReleaseUpdater.installUpdate(this, existingFile)
            return
        }

        updateJob?.cancel()
        binding.aboutUpdateStatusValue.text = getString(R.string.update_status_downloading)
        binding.aboutUpdateSummaryValue.text = getString(R.string.update_summary_downloading, update.assetName)
        binding.checkUpdateButton.isEnabled = false
        binding.installUpdateButton.isEnabled = false

        updateJob = uiScope.launch {
            val result = runCatching {
                ReleaseUpdater.downloadUpdate(this@SettingsActivity, update)
            }

            result.onSuccess { apkFile ->
                downloadedUpdateFile = apkFile
                renderUpdateState()
                ReleaseUpdater.installUpdate(this@SettingsActivity, apkFile)
            }.onFailure { error ->
                renderUpdateState()
                Toast.makeText(
                    this@SettingsActivity,
                    error.message ?: getString(R.string.update_download_failed),
                    Toast.LENGTH_LONG,
                ).show()
            }
        }
    }

    private fun renderUpdateState() {
        val update = availableUpdate
        binding.checkUpdateButton.isEnabled = true

        if (update == null) {
            binding.aboutUpdateStatusValue.text = getString(R.string.update_status_unknown)
            binding.aboutUpdateSummaryValue.text = getString(R.string.update_summary_idle)
            binding.installUpdateButton.visibility = android.view.View.GONE
            return
        }

        if (!update.isUpdateAvailable) {
            binding.aboutUpdateStatusValue.text = getString(R.string.update_status_latest)
            binding.aboutUpdateSummaryValue.text = getString(
                R.string.update_summary_latest,
                update.currentVersion,
            )
            binding.installUpdateButton.visibility = android.view.View.GONE
            return
        }

        binding.aboutUpdateStatusValue.text = getString(
            R.string.update_status_available,
            update.latestVersion,
        )
        binding.aboutUpdateSummaryValue.text = buildUpdateSummary(update)
        binding.installUpdateButton.visibility = android.view.View.VISIBLE
        binding.installUpdateButton.text = if (downloadedUpdateFile?.exists() == true) {
            getString(R.string.update_install_downloaded_button)
        } else {
            getString(R.string.update_download_install_button)
        }
        binding.installUpdateButton.isEnabled = update.isInstallable && update.hasDownloadAsset
    }

    private fun buildUpdateSummary(update: AndroidReleaseUpdate): String {
        val assetLine = if (update.hasDownloadAsset) {
            getString(
                R.string.update_summary_asset,
                update.assetName,
                Formatter.formatShortFileSize(this, update.assetSize),
            )
        } else {
            getString(R.string.update_summary_asset_missing)
        }
        val publishedLine = formatPublishedAt(update.publishedAt)
        val notes = update.releaseNotes
            .lineSequence()
            .map { it.trim() }
            .firstOrNull { it.isNotBlank() }
            .orEmpty()

        val suffix = when {
            !update.isInstallable -> getString(R.string.update_summary_no_installable_asset)
            notes.isNotBlank() -> notes
            else -> getString(R.string.update_summary_ready_to_install)
        }

        return listOf(
            getString(R.string.update_summary_version, update.currentVersion, update.latestVersion),
            assetLine,
            publishedLine,
            suffix,
        ).joinToString("\n")
    }

    private fun formatPublishedAt(value: String): String {
        if (value.isBlank()) {
            return getString(R.string.update_summary_published_unknown)
        }

        return runCatching {
            val parser = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ssX", Locale.US).apply {
                timeZone = TimeZone.getTimeZone("UTC")
            }
            val publishedAt = parser.parse(value) ?: return@runCatching getString(
                R.string.update_summary_published_plain,
                value,
            )
            val absolute = DateFormat.getDateTimeInstance(DateFormat.SHORT, DateFormat.SHORT).format(publishedAt)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
                val relative = DateUtils.getRelativeTimeSpanString(
                    publishedAt.time,
                    System.currentTimeMillis(),
                    DateUtils.MINUTE_IN_MILLIS,
                )
                getString(R.string.update_summary_published_format, absolute, relative)
            } else {
                getString(R.string.update_summary_published_plain, absolute)
            }
        }.getOrElse {
            getString(R.string.update_summary_published_plain, value)
        }
    }
}
