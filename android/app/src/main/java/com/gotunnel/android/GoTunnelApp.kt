package com.gotunnel.android

import android.app.Application
import com.gotunnel.android.service.NotificationHelper

class GoTunnelApp : Application() {
    override fun onCreate() {
        super.onCreate()
        NotificationHelper.ensureChannel(this)
    }
}
