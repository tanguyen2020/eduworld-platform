@tool
extends EditorExportPlugin
## Adds the EduWorldPlatform Android plugin AAR (built by android/build.gradle.kts into bin/) to Android gradle
## exports. Without the AAR the export still works and SecureStorage falls back to MemoryOnly on Android.

const AAR_DIR: String = "eduworld_platform/bin"


func _get_name() -> String:
	return "EduWorldPlatform"


func _supports_platform(platform: EditorExportPlatform) -> bool:
	return platform is EditorExportPlatformAndroid


func _get_android_libraries(_platform: EditorExportPlatform, debug: bool) -> PackedStringArray:
	var file_name: String = "eduworld-platform-%s.aar" % ("debug" if debug else "release")
	var relative: String = AAR_DIR.path_join(file_name)
	if FileAccess.file_exists("res://addons".path_join(relative)):
		return PackedStringArray([relative])
	return PackedStringArray()
