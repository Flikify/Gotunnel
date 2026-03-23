package com.gotunnel.android.bridge

import android.content.Context

object GoTunnelBridge {
    @Volatile
    private var controller: TunnelController? = null

    fun create(context: Context): TunnelController {
        return controller ?: synchronized(this) {
            controller ?: NativeTunnelController(context.applicationContext).also { controller = it }
        }
    }
}
