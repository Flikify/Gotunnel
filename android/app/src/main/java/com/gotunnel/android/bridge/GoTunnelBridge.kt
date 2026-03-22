package com.gotunnel.android.bridge

import android.content.Context

object GoTunnelBridge {
    fun create(context: Context): TunnelController {
        // Stub bridge for the Android shell. Replace with a native Go binding later.
        return StubTunnelController(context.applicationContext)
    }
}
