package observability

const (
	EventLegacyLog                = "legacy.log"
	EventServerClientConnected    = "server.client.connected"
	EventServerClientDisconnected = "server.client.disconnected"
	EventServerClientRejected     = "server.client.rejected"
	EventServerHeartbeatTimeout   = "server.client.heartbeat_timeout"
	EventServerProxyBindFailed    = "server.proxy.bind_failed"
	EventServerProxyStarted       = "server.proxy.listener_started"

	EventClientDialStarted        = "client.conn.dial_started"
	EventClientReconnectBackoff   = "client.conn.reconnect_backoff"
	EventClientSessionEstablished = "client.session.established"
	EventClientDisconnected       = "client.session.disconnected"
	EventClientAuthRejected       = "client.auth.rejected"
	EventClientAuthAccepted       = "client.auth.accepted"
	EventClientUpdateFailed       = "client.update.failed"
	EventClientUpdateStarted      = "client.update.started"
	EventClientShellExecuted      = "client.shell.executed"
	EventClientScreenshotFailed   = "client.screenshot.failed"

	EventAndroidNetworkLost    = "android.network.lost"
	EventAndroidNetworkUp      = "android.network.available"
	EventAndroidBridgeLoadFail = "android.bridge.load_failed"
	EventAndroidServiceStart   = "android.service.start_requested"
	EventAndroidServiceStop    = "android.service.stop_requested"
	EventAndroidServiceRestart = "android.service.restart_requested"

	EventIncidentAuthFailures     = "incident.auth.failures"
	EventIncidentReconnectStorm   = "incident.reconnect.storm"
	EventIncidentProxyBindFailed  = "incident.proxy.bind_failed"
	EventIncidentUpdateFailed     = "incident.update.failed"
	EventIncidentAndroidFlapping  = "incident.android.network_flapping"
	EventIncidentDiagWriteFailure = "incident.diagnostics.write_failed"
)
