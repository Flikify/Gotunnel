plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val mobileAar = file("libs/gotunnelmobile.aar")
val appVersionName = providers.gradleProperty("gotunnelVersionName").orNull ?: "0.1.0"
val appVersionCode = providers.gradleProperty("gotunnelVersionCode").orNull?.toIntOrNull()
    ?: parseVersionCode(appVersionName)

val releaseStoreFile = providers.gradleProperty("gotunnelReleaseStoreFile").orNull
val releaseStorePassword = providers.gradleProperty("gotunnelReleaseStorePassword").orNull
val releaseKeyAlias = providers.gradleProperty("gotunnelReleaseKeyAlias").orNull
val releaseKeyPassword = providers.gradleProperty("gotunnelReleaseKeyPassword").orNull
val hasReleaseSigning = !releaseStoreFile.isNullOrBlank() &&
    !releaseStorePassword.isNullOrBlank() &&
    !releaseKeyAlias.isNullOrBlank() &&
    !releaseKeyPassword.isNullOrBlank()

android {
    namespace = "com.gotunnel.android"
    compileSdk = 34

    signingConfigs {
        if (hasReleaseSigning) {
            create("release") {
                storeFile = file(requireNotNull(releaseStoreFile))
                storePassword = requireNotNull(releaseStorePassword)
                keyAlias = requireNotNull(releaseKeyAlias)
                keyPassword = requireNotNull(releaseKeyPassword)
            }
        }
    }

    defaultConfig {
        applicationId = "com.gotunnel.android"
        minSdk = 24
        targetSdk = 34
        versionCode = appVersionCode
        versionName = appVersionName
        buildConfigField("String", "GITHUB_REPO_OWNER", "\"Flikify\"")
        buildConfigField("String", "GITHUB_REPO_NAME", "\"Gotunnel\"")
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            if (hasReleaseSigning) {
                signingConfig = signingConfigs.getByName("release")
            }
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        buildConfig = true
        viewBinding = true
    }
}

dependencies {
    implementation(
        fileTree(
            mapOf(
                "dir" to "libs",
                "include" to listOf("*.jar", "*.aar"),
            ),
        ),
    )
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.activity:activity-ktx:1.9.2")
    implementation("com.google.android.material:material:1.12.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
}

val verifyMobileAar by tasks.registering {
    doLast {
        check(mobileAar.exists()) {
            "Missing android/app/libs/gotunnelmobile.aar. Run ./scripts/build.sh android or gomobile bind before building the APK."
        }
    }
}

tasks.configureEach {
    if (name in setOf("preBuild", "preDebugBuild", "preReleaseBuild", "assembleDebug", "assembleRelease")) {
        dependsOn(verifyMobileAar)
    }
}

fun parseVersionCode(versionName: String): Int {
    val numbers = versionName
        .removePrefix("v")
        .removePrefix("V")
        .split(Regex("[^0-9]+"))
        .filter { it.isNotBlank() }
        .mapNotNull { it.toIntOrNull() }

    if (numbers.isEmpty()) {
        return 1
    }

    val major = numbers.getOrElse(0) { 0 }.coerceIn(0, 99)
    val minor = numbers.getOrElse(1) { 0 }.coerceIn(0, 999)
    val patch = numbers.getOrElse(2) { 0 }.coerceIn(0, 999)
    return (major * 1_000_000 + minor * 1_000 + patch).coerceAtLeast(1)
}
