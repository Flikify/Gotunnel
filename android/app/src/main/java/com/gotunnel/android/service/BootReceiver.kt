package com.gotunnel.android.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import androidx.core.content.ContextCompat
import com.gotunnel.android.config.ConfigStore

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val action = intent.action ?: return
        if (action != Intent.ACTION_BOOT_COMPLETED && action != Intent.ACTION_MY_PACKAGE_REPLACED) {
            return
        }

        val config = ConfigStore(context).load()
        if (!config.autoStart) {
            return
        }

        ContextCompat.startForegroundService(
            context,
            TunnelService.createStartIntent(context, action.lowercase()),
        )
    }
}
