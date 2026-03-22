package com.gotunnel.android.config

data class AppConfig(
    val serverAddress: String = "",
    val token: String = "",
    val autoStart: Boolean = true,
    val autoReconnect: Boolean = true,
    val useTls: Boolean = true,
)
