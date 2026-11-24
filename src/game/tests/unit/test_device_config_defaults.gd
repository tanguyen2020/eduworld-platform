extends GdUnitTestSuite
## F060-AC-27 (ADR-0078, ADR-0086, ADR-0101): without `shared_device` in device.cfg, operating systems that are not
## released (platform key `other`: Windows, macOS, Linux editor/debug runs) are shared (MemoryOnly); Android and iOS
## default to personal devices; Web is always shared.

const ConfigScript := preload("res://scripts/core/config.gd")
const DEVICE_PATH: String = "user://test_device.cfg"

var _config: Node = null


func before_test() -> void:
	DirAccess.remove_absolute(DEVICE_PATH)
	_config = auto_free(ConfigScript.new()) as Node


func after_test() -> void:
	DirAccess.remove_absolute(DEVICE_PATH)


func _write_device(values: Dictionary) -> void:
	var cfg: ConfigFile = ConfigFile.new()
	for key: String in values.keys():
		cfg.set_value("device", key, values[key])
	cfg.save(DEVICE_PATH)


func test_unreleased_desktop_maps_to_other() -> void:
	for os_name: String in ["Windows", "macOS", "Linux"]:
		assert_str(ConfigScript.platform_key(os_name, false)).is_equal("other")
	assert_str(ConfigScript.platform_key("Windows", true)).is_equal("web")


func test_other_without_device_cfg_is_shared() -> void:
	_config.call("load_device_config", DEVICE_PATH, "other")
	assert_bool(bool(_config.get("shared_device"))).is_true()
	assert_str(SecureStorageFactory.create(bool(_config.get("shared_device")), "other").adapter_name()).is_equal("memory_only")


func test_other_without_the_key_is_shared() -> void:
	_write_device({"lan_cache_url": "http://cache.school.lan"})
	_config.call("load_device_config", DEVICE_PATH, "other")
	assert_bool(bool(_config.get("shared_device"))).is_true()
	assert_str(String(_config.get("lan_cache_url"))).is_equal("http://cache.school.lan")


func test_other_personal_setting_still_has_no_native_store() -> void:
	_write_device({"shared_device": false})
	_config.call("load_device_config", DEVICE_PATH, "other")
	assert_bool(bool(_config.get("shared_device"))).is_false()
	assert_str(SecureStorageFactory.create(false, "other").adapter_name()).is_equal("memory_only")


func test_android_and_ios_default_to_personal() -> void:
	for platform: String in ["android", "ios"]:
		_config.call("load_device_config", DEVICE_PATH, platform)
		assert_bool(bool(_config.get("shared_device"))).is_false()


func test_web_is_always_shared() -> void:
	_write_device({"shared_device": false})
	_config.call("load_device_config", DEVICE_PATH, "web")
	assert_bool(bool(_config.get("shared_device"))).is_true()


func test_defaults_come_from_project_settings() -> void:
	assert_bool(ConfigScript.shared_device_default("android")).is_equal(
		bool(ProjectSettings.get_setting("eduworld/device/shared_device_default/android")))
	assert_bool(ProjectSettings.has_setting("eduworld/device/shared_device_default/windows")).is_false()
	assert_bool(ConfigScript.shared_device_default("unknown_platform")).is_true()
