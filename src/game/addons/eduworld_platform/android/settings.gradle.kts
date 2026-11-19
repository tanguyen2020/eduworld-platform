// EduWorld platform Android plugin (F060 T052, ADR-0086). Built in CI (godot.yml job "platform-android"):
//   gradle -p src/game/addons/eduworld_platform/android assemble
pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
    }
}
rootProject.name = "eduworld-platform"
