package com.gotunnel.android.config

data class ServerEndpoint(
    val host: String = "",
    val port: String = "",
)

object ServerEndpointParser {
    fun parse(raw: String): ServerEndpoint {
        val value = raw.trim()
        if (value.isBlank()) {
            return ServerEndpoint()
        }

        if (value.startsWith("[") && value.contains("]:")) {
            val closeIndex = value.indexOf("]:")
            if (closeIndex > 0) {
                return ServerEndpoint(
                    host = value.substring(1, closeIndex).trim(),
                    port = value.substring(closeIndex + 2).trim(),
                )
            }
        }

        if (value.count { it == ':' } == 1) {
            val separator = value.lastIndexOf(':')
            return ServerEndpoint(
                host = value.substring(0, separator).trim(),
                port = value.substring(separator + 1).trim(),
            )
        }

        return ServerEndpoint(host = value)
    }

    fun compose(host: String, port: String): String {
        val cleanHost = normalizeHost(host)
        val cleanPort = port.trim()
        if (cleanHost.isBlank()) {
            return ""
        }
        if (cleanPort.isBlank()) {
            return cleanHost
        }

        val normalizedHost = if (cleanHost.contains(':') && !cleanHost.startsWith("[") && !cleanHost.endsWith("]")) {
            "[$cleanHost]"
        } else {
            cleanHost
        }
        return "$normalizedHost:$cleanPort"
    }

    private fun normalizeHost(host: String): String {
        var normalized = host.trim()
        if (normalized.contains("://")) {
            normalized = normalized.substringAfter("://")
        }
        normalized = normalized.substringBefore("/").substringBefore("?").substringBefore("#").trim()
        if (normalized.startsWith("[") && normalized.endsWith("]") && normalized.length > 2) {
            normalized = normalized.substring(1, normalized.length - 1).trim()
        }
        return normalized
    }
}
