package com.gotunnel.android.bridge

import android.content.Context
import android.os.Build
import com.gotunnel.android.config.AppConfig
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import org.json.JSONArray
import java.io.File
import java.lang.reflect.Method

class NativeTunnelController(
    private val context: Context,
) : TunnelController {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var listener: TunnelController.Listener? = null
    private var config: AppConfig = AppConfig()
    private var bridge: MobileBinding? = null
    private var pollJob: Job? = null
    private var lastStatus: TunnelStatus? = null
    private var lastDetail: String = ""
    private var publishedLogs: List<String> = emptyList()

    override val isRunning: Boolean
        get() = bridge?.snapshot()?.isRunning == true

    override fun setListener(listener: TunnelController.Listener?) {
        this.listener = listener
    }

    override fun updateConfig(config: AppConfig) {
        this.config = config
    }

    override fun start(config: AppConfig) {
        updateConfig(config)

        val binding = runCatching { bridge ?: MobileBinding.load(context).also { bridge = it } }
            .getOrElse { error ->
                emitStatus(TunnelStatus.ERROR, error.message ?: "Failed to load native Go binding")
                listener?.onLog("Native Go binding unavailable: ${error.message ?: error::class.java.simpleName}")
                return
            }

        if (binding.snapshot().isRunning) {
            listener?.onLog("Native tunnel already running")
            startPolling(binding)
            return
        }

        emitStatus(TunnelStatus.STARTING, "Starting native Go core")
        listener?.onLog("Preparing native tunnel for ${config.serverAddress}")

        val error = binding.configureAndStart(config, dataDir = File(context.filesDir, "gotunnel"))
        if (!error.isNullOrBlank()) {
            emitStatus(TunnelStatus.ERROR, error)
            listener?.onLog("Native Go core start failed: $error")
            return
        }

        startPolling(binding)
    }

    override fun stop(reason: String) {
        pollJob?.cancel()
        pollJob = null
        bridge?.stop()
        listener?.onLog("Native tunnel stop requested: $reason")
        emitStatus(TunnelStatus.STOPPED, reason)
    }

    override fun restart(reason: String) {
        emitStatus(TunnelStatus.RECONNECTING, reason)
        listener?.onLog("Native tunnel restart requested: $reason")

        val binding = runCatching { bridge ?: MobileBinding.load(context).also { bridge = it } }
            .getOrElse { error ->
                emitStatus(TunnelStatus.ERROR, error.message ?: "Failed to load native Go binding")
                listener?.onLog("Native Go binding unavailable: ${error.message ?: error::class.java.simpleName}")
                return
            }

        pollJob?.cancel()
        pollJob = null
        binding.stop()

        val result = binding.configureAndStart(config, dataDir = File(context.filesDir, "gotunnel"))
        if (!result.isNullOrBlank()) {
            emitStatus(TunnelStatus.ERROR, result)
            listener?.onLog("Native Go core restart failed: $result")
            return
        }

        startPolling(binding)
    }

    override fun getActiveTunnels(): List<ActiveTunnel> {
        return bridge?.activeTunnels().orEmpty()
    }

    private fun startPolling(binding: MobileBinding) {
        if (pollJob?.isActive == true) {
            return
        }

        pollJob = scope.launch {
            while (isActive) {
                publishSnapshot(binding.snapshot())
                delay(POLL_INTERVAL_MS)
            }
        }
    }

    private fun publishSnapshot(snapshot: MobileBinding.Snapshot) {
        val status = mapStatus(snapshot.status, snapshot.isRunning)
        val detail = snapshot.detail.ifBlank {
            snapshot.lastError.ifBlank {
                when (status) {
                    TunnelStatus.RUNNING -> "Native Go core is running"
                    TunnelStatus.STARTING -> "Starting native Go core"
                    TunnelStatus.RECONNECTING -> "Reconnecting"
                    TunnelStatus.ERROR -> "Native Go core reported an error"
                    TunnelStatus.STOPPED -> "Stopped"
                }
            }
        }

        emitStatus(status, detail)
        publishNewLogs(snapshot.recentLogs)
    }

    private fun publishNewLogs(renderedLogs: String) {
        val lines = renderedLogs.lines().map { it.trimEnd() }.filter { it.isNotBlank() }
        if (lines.isEmpty()) {
            publishedLogs = emptyList()
            return
        }

        val newLines = if (lines.size >= publishedLogs.size && lines.take(publishedLogs.size) == publishedLogs) {
            lines.drop(publishedLogs.size)
        } else {
            lines
        }
        newLines.forEach { listener?.onLog(it) }
        publishedLogs = lines
    }

    private fun emitStatus(status: TunnelStatus, detail: String) {
        if (lastStatus == status && lastDetail == detail) {
            return
        }
        lastStatus = status
        lastDetail = detail
        listener?.onStatusChanged(status, detail)
    }

    private fun mapStatus(status: String, isRunning: Boolean): TunnelStatus {
        return when (status.lowercase()) {
            "running" -> TunnelStatus.RUNNING
            "starting", "connecting" -> TunnelStatus.STARTING
            "reconnecting" -> TunnelStatus.RECONNECTING
            "error" -> TunnelStatus.ERROR
            "stopped" -> if (isRunning) TunnelStatus.STARTING else TunnelStatus.STOPPED
            else -> if (isRunning) TunnelStatus.RUNNING else TunnelStatus.STOPPED
        }
    }

    private class MobileBinding private constructor(
        private val service: Any,
        private val configureMethod: Method,
        private val startMethod: Method,
        private val stopMethod: Method,
        private val isRunningMethod: Method,
        private val statusMethod: Method,
        private val detailMethod: Method,
        private val lastErrorMethod: Method,
        private val recentLogsMethod: Method,
        private val activeTunnelsJSONMethod: Method?,
    ) {
        fun configureAndStart(config: AppConfig, dataDir: File): String? {
            dataDir.mkdirs()
            configureMethod.invoke(
                service,
                config.serverAddress,
                config.token,
                dataDir.absolutePath,
                defaultClientName(),
                "",
                false,
            )
            return start()
        }

        fun start(): String? = startMethod.invoke(service) as? String

        fun stop(): String? = stopMethod.invoke(service) as? String

        fun snapshot(): Snapshot {
            return Snapshot(
                isRunning = isRunningMethod.invoke(service) as? Boolean ?: false,
                status = statusMethod.invoke(service) as? String ?: "",
                detail = detailMethod.invoke(service) as? String ?: "",
                lastError = lastErrorMethod.invoke(service) as? String ?: "",
                recentLogs = recentLogsMethod.invoke(service) as? String ?: "",
            )
        }

        fun activeTunnels(): List<ActiveTunnel> {
            val method = activeTunnelsJSONMethod ?: return emptyList()
            val payload = method.invoke(service) as? String ?: return emptyList()
            if (payload.isBlank()) {
                return emptyList()
            }

            return runCatching {
                val array = JSONArray(payload)
                buildList {
                    for (index in 0 until array.length()) {
                        val item = array.optJSONObject(index) ?: continue
                        add(
                            ActiveTunnel(
                                name = item.optString("name"),
                                type = item.optString("type"),
                                remotePort = item.optInt("remote_port"),
                                localIP = item.optString("local_ip"),
                                localPort = item.optInt("local_port"),
                                status = item.optString("status"),
                                connectedAt = item.optLong("connected_at"),
                            ),
                        )
                    }
                }
            }.getOrDefault(emptyList())
        }

        data class Snapshot(
            val isRunning: Boolean,
            val status: String,
            val detail: String,
            val lastError: String,
            val recentLogs: String,
        )

        companion object {
            private val packageClassCandidates = listOf(
                "com.gotunnel.mobilebind.Gotunnelmobile",
                "com.gotunnel.mobilebind.GoTunnelmobile",
                "go.gotunnelmobile.Gotunnelmobile",
            )
            private val serviceClassCandidates = listOf(
                "com.gotunnel.mobilebind.Service",
                "go.gotunnelmobile.Service",
            )

            fun load(context: Context): MobileBinding {
                val loader = context.classLoader
                val packageClass = packageClassCandidates.firstNotNullOfOrNull { name ->
                    runCatching { Class.forName(name, true, loader) }.getOrNull()
                }
                val serviceClass = serviceClassCandidates.firstNotNullOfOrNull { name ->
                    runCatching { Class.forName(name, true, loader) }.getOrNull()
                }

                val service = createServiceInstance(packageClass, serviceClass)
                val klass = service.javaClass
                return MobileBinding(
                    service = service,
                    configureMethod = findMethod(klass, "configure", "Configure", String::class.java, String::class.java, String::class.java, String::class.java, String::class.java, Boolean::class.javaPrimitiveType!!),
                    startMethod = findMethod(klass, "start", "Start"),
                    stopMethod = findMethod(klass, "stop", "Stop"),
                    isRunningMethod = findMethod(klass, "isRunning", "IsRunning"),
                    statusMethod = findMethod(klass, "status", "Status"),
                    detailMethod = findMethod(klass, "detail", "Detail"),
                    lastErrorMethod = findMethod(klass, "lastError", "LastError"),
                    recentLogsMethod = findMethod(klass, "recentLogs", "RecentLogs"),
                    activeTunnelsJSONMethod = findMethodOrNull(klass, "activeTunnelsJSON", "ActiveTunnelsJSON"),
                )
            }

            private fun createServiceInstance(packageClass: Class<*>?, serviceClass: Class<*>?): Any {
                if (packageClass != null && serviceClass != null) {
                    val packageSimpleName = packageClass.simpleName
                    val serviceSimpleName = serviceClass.simpleName
                    val factoryCandidates = listOf(
                        "new$serviceSimpleName",
                        "New$serviceSimpleName",
                        "new${packageSimpleName}",
                        "New${packageSimpleName}",
                        "newService",
                        "NewService",
                    )

                    for (methodName in factoryCandidates) {
                        val method = runCatching { packageClass.getMethod(methodName) }.getOrNull() ?: continue
                        return method.invoke(null)
                    }
                }

                if (serviceClass != null) {
                    return serviceClass.getDeclaredConstructor().newInstance()
                }

                error("gomobile binding classes not found in APK; build the AAR and bundle android/app/libs/gotunnelmobile.aar")
            }

            private fun findMethod(target: Class<*>, vararg namesAndTypes: Any): Method {
                val parameterTypes = namesAndTypes.dropWhile { it is String }.map { it as Class<*> }.toTypedArray()
                val names = namesAndTypes.takeWhile { it is String }.map { it as String }
                for (name in names) {
                    val method = runCatching { target.getMethod(name, *parameterTypes) }.getOrNull()
                    if (method != null) {
                        return method
                    }
                }
                error("Required gomobile method not found on ${target.name}: ${names.joinToString("/")}")
            }

            private fun findMethodOrNull(target: Class<*>, vararg names: String): Method? {
                for (name in names) {
                    val method = runCatching { target.getMethod(name) }.getOrNull()
                    if (method != null) {
                        return method
                    }
                }
                return null
            }

            private fun defaultClientName(): String {
                val manufacturer = Build.MANUFACTURER.orEmpty().trim()
                val model = Build.MODEL.orEmpty().trim()
                return listOf(manufacturer, model).filter { it.isNotBlank() }.joinToString(" ").ifBlank {
                    "android-device"
                }
            }
        }
    }

    companion object {
        private const val POLL_INTERVAL_MS = 1_000L
    }
}
