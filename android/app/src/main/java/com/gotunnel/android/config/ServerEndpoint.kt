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
        val cleanHost = host.trim()
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
}
