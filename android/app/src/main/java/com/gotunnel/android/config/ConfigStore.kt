package com.gotunnel.android.config

import android.content.Context

class ConfigStore(context: Context) {
    private val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    fun load(): AppConfig {
        return AppConfig(
            serverAddress = prefs.getString(KEY_SERVER_ADDRESS, "") ?: "",
            token = prefs.getString(KEY_TOKEN, "") ?: "",
            autoStart = prefs.getBoolean(KEY_AUTO_START, true),
            autoReconnect = prefs.getBoolean(KEY_AUTO_RECONNECT, true),
        )
    }

    fun save(config: AppConfig) {
        prefs.edit()
            .putString(KEY_SERVER_ADDRESS, config.serverAddress)
            .putString(KEY_TOKEN, config.token)
            .putBoolean(KEY_AUTO_START, config.autoStart)
            .putBoolean(KEY_AUTO_RECONNECT, config.autoReconnect)
            .remove(KEY_USE_TLS)
            .apply()
    }

    companion object {
        private const val PREFS_NAME = "gotunnel_config"
        private const val KEY_SERVER_ADDRESS = "server_address"
        private const val KEY_TOKEN = "token"
        private const val KEY_AUTO_START = "auto_start"
        private const val KEY_AUTO_RECONNECT = "auto_reconnect"
        private const val KEY_USE_TLS = "use_tls"
    }
}
