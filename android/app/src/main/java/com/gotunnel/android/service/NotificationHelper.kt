package com.gotunnel.android.service

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.app.NotificationCompat
import com.gotunnel.android.MainActivity
import com.gotunnel.android.R
import com.gotunnel.android.bridge.TunnelStatus
import com.gotunnel.android.config.AppConfig

object NotificationHelper {
    const val CHANNEL_ID = "gotunnel_tunnel"
    const val NOTIFICATION_ID = 2001

    fun ensureChannel(context: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
            return
        }

        val manager = context.getSystemService(NotificationManager::class.java) ?: return
        if (manager.getNotificationChannel(CHANNEL_ID) != null) {
            return
        }

        val channel = NotificationChannel(
            CHANNEL_ID,
            context.getString(R.string.notification_channel_name),
            NotificationManager.IMPORTANCE_LOW,
        ).apply {
            description = context.getString(R.string.notification_channel_description)
        }
        manager.createNotificationChannel(channel)
    }

    fun build(
        context: Context,
        status: TunnelStatus,
        detail: String,
        config: AppConfig,
    ): Notification {
        val baseText = when {
            detail.isNotBlank() -> detail
            config.serverAddress.isNotBlank() -> context.getString(
                R.string.notification_text_configured,
                config.serverAddress,
            )
            else -> context.getString(R.string.notification_text_unconfigured)
        }

        val contentIntent = PendingIntent.getActivity(
            context,
            0,
            Intent(context, MainActivity::class.java),
            pendingIntentFlags(),
        )

        val stopIntent = PendingIntent.getService(
            context,
            1,
            TunnelService.createStopIntent(context, "notification-stop"),
            pendingIntentFlags(),
        )

        val restartIntent = PendingIntent.getService(
            context,
            2,
            TunnelService.createRestartIntent(context, "notification-restart"),
            pendingIntentFlags(),
        )

        return NotificationCompat.Builder(context, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_gotunnel_notification)
            .setContentTitle(context.getString(R.string.notification_title, status.name))
            .setContentText(baseText)
            .setStyle(NotificationCompat.BigTextStyle().bigText(baseText))
            .setOngoing(status != TunnelStatus.STOPPED)
            .setOnlyAlertOnce(true)
            .setContentIntent(contentIntent)
            .addAction(android.R.drawable.ic_popup_sync, context.getString(R.string.notification_action_restart), restartIntent)
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, context.getString(R.string.notification_action_stop), stopIntent)
            .build()
    }

    private fun pendingIntentFlags(): Int {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        } else {
            PendingIntent.FLAG_UPDATE_CURRENT
        }
    }
}
