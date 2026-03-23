package com.gotunnel.android.update

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.provider.Settings
import androidx.core.content.FileProvider
import com.gotunnel.android.BuildConfig
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.File
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

data class AndroidReleaseUpdate(
    val currentVersion: String,
    val latestVersion: String,
    val releaseNotes: String,
    val publishedAt: String,
    val assetName: String,
    val assetSize: Long,
    val downloadUrl: String,
    val isUpdateAvailable: Boolean,
    val isInstallable: Boolean,
) {
    val hasDownloadAsset: Boolean
        get() = assetName.isNotBlank() && downloadUrl.isNotBlank()
}

object ReleaseUpdater {
    private const val API_BASE_URL = "https://api.github.com"
    private const val GITHUB_API_VERSION = "2022-11-28"
    private const val USER_AGENT = "GoTunnel-Android-Updater"
    private const val CONNECT_TIMEOUT_MS = 15_000
    private const val READ_TIMEOUT_MS = 30_000

    suspend fun checkForUpdate(currentVersion: String): AndroidReleaseUpdate = withContext(Dispatchers.IO) {
        val endpoint = "$API_BASE_URL/repos/${BuildConfig.GITHUB_REPO_OWNER}/${BuildConfig.GITHUB_REPO_NAME}/releases/latest"
        val response = request(endpoint)
        val release = JSONObject(response)
        val latestVersion = release.optString("tag_name").ifBlank {
            throw IOException("GitHub release is missing tag_name")
        }
        val asset = selectAndroidAsset(release)
        val assetName = asset?.optString("name").orEmpty()
        val downloadUrl = asset?.optString("browser_download_url").orEmpty()
        val installable = assetName.isNotBlank() && !assetName.contains("unsigned", ignoreCase = true)

        AndroidReleaseUpdate(
            currentVersion = currentVersion,
            latestVersion = latestVersion,
            releaseNotes = release.optString("body").trim(),
            publishedAt = release.optString("published_at"),
            assetName = assetName,
            assetSize = asset?.optLong("size") ?: 0L,
            downloadUrl = downloadUrl,
            isUpdateAvailable = compareVersions(currentVersion, latestVersion) < 0,
            isInstallable = installable,
        )
    }

    suspend fun downloadUpdate(context: Context, update: AndroidReleaseUpdate): File = withContext(Dispatchers.IO) {
        require(update.hasDownloadAsset) { "Latest release does not contain an Android APK asset" }

        val downloadsDir = context.getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS) ?: context.cacheDir
        val targetDir = File(downloadsDir, "updates").apply { mkdirs() }
        val targetFile = File(targetDir, sanitizeFileName(update.assetName))
        if (targetFile.exists() && targetFile.length() > 0L) {
            return@withContext targetFile
        }

        val tempFile = File(targetDir, "${targetFile.name}.part")
        request(update.downloadUrl, acceptJson = false) { connection ->
            connection.inputStream.use { input ->
                tempFile.outputStream().use { output ->
                    input.copyTo(output)
                }
            }
        }

        if (!tempFile.renameTo(targetFile)) {
            tempFile.copyTo(targetFile, overwrite = true)
            tempFile.delete()
        }

        targetFile
    }

    fun canRequestPackageInstalls(context: Context): Boolean {
        return Build.VERSION.SDK_INT < Build.VERSION_CODES.O || context.packageManager.canRequestPackageInstalls()
    }

    fun openUnknownAppsSettings(context: Context) {
        val intent = Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES).apply {
            data = Uri.parse("package:${context.packageName}")
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        context.startActivity(intent)
    }

    fun installUpdate(context: Context, apkFile: File) {
        val authority = "${context.packageName}.fileprovider"
        val contentUri = FileProvider.getUriForFile(context, authority, apkFile)
        val intent = Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(contentUri, "application/vnd.android.package-archive")
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        context.startActivity(intent)
    }

    private fun selectAndroidAsset(release: JSONObject): JSONObject? {
        val assets = release.optJSONArray("assets") ?: return null
        var fallback: JSONObject? = null

        for (index in 0 until assets.length()) {
            val asset = assets.optJSONObject(index) ?: continue
            val name = asset.optString("name")
            if (!name.endsWith(".apk", ignoreCase = true)) {
                continue
            }
            if (!name.contains("android", ignoreCase = true)) {
                continue
            }

            if (!name.contains("unsigned", ignoreCase = true)) {
                return asset
            }
            if (fallback == null) {
                fallback = asset
            }
        }

        return fallback
    }

    private fun request(
        url: String,
        acceptJson: Boolean = true,
        block: ((HttpURLConnection) -> Unit)? = null,
    ): String {
        val connection = openConnection(url, acceptJson)
        return connection.use {
            val statusCode = it.responseCode
            if (statusCode !in 200..299) {
                val errorBody = it.errorStream?.bufferedReader()?.use { reader -> reader.readText() }.orEmpty()
                throw IOException("GitHub request failed: HTTP $statusCode ${errorBody.take(200)}")
            }

            if (block != null) {
                block(it)
                ""
            } else {
                it.inputStream.bufferedReader().use { reader -> reader.readText() }
            }
        }
    }

    private fun openConnection(url: String, acceptJson: Boolean): HttpURLConnection {
        return (URL(url).openConnection() as HttpURLConnection).apply {
            requestMethod = "GET"
            connectTimeout = CONNECT_TIMEOUT_MS
            readTimeout = READ_TIMEOUT_MS
            instanceFollowRedirects = true
            setRequestProperty("User-Agent", USER_AGENT)
            setRequestProperty("X-GitHub-Api-Version", GITHUB_API_VERSION)
            if (acceptJson) {
                setRequestProperty("Accept", "application/vnd.github+json")
            }
        }
    }

    private fun sanitizeFileName(name: String): String {
        return name.replace(Regex("[^A-Za-z0-9._-]"), "_")
    }

    private fun compareVersions(current: String, latest: String): Int {
        val currentParts = parseVersionParts(current)
        val latestParts = parseVersionParts(latest)
        val maxSize = maxOf(currentParts.size, latestParts.size)

        for (index in 0 until maxSize) {
            val left = currentParts.getOrElse(index) { 0 }
            val right = latestParts.getOrElse(index) { 0 }
            if (left != right) {
                return left.compareTo(right)
            }
        }

        return 0
    }

    private fun parseVersionParts(version: String): List<Int> {
        val normalized = version.trim().removePrefix("v").removePrefix("V")
        return normalized
            .split(Regex("[^0-9]+"))
            .filter { it.isNotBlank() }
            .mapNotNull { it.toIntOrNull() }
    }

    private inline fun <T : HttpURLConnection, R> T.use(block: (T) -> R): R {
        return try {
            block(this)
        } finally {
            disconnect()
        }
    }
}
