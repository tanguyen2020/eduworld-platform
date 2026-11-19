// EduWorld platform Android plugin (F060 T052, ADR-0086): AAR copied to ../bin for the export plugin.
plugins {
    id("com.android.library") version "8.7.3"
    id("org.jetbrains.kotlin.android") version "2.1.0"
}

android {
    namespace = "vn.eduworld.platform"
    compileSdk = 35
    defaultConfig {
        minSdk = 28 // Android 9+ (GODOT_ARCHITECTURE §11)
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

kotlin {
    jvmToolchain(17)
}

dependencies {
    // Godot Android library of the pinned engine version (src/game/.godot-version), provided by the export.
    compileOnly("org.godotengine:godot:4.7.2.stable")
}

val copyAar by tasks.registering(Copy::class) {
    dependsOn("assembleDebug", "assembleRelease")
    from(layout.buildDirectory.dir("outputs/aar"))
    include("*.aar")
    into(layout.projectDirectory.dir("../bin"))
    rename("android-(debug|release).aar", "eduworld-platform-$1.aar")
}

tasks.named("assemble") { finalizedBy(copyAar) }
