class_name ClientConfig
extends RefCounted
## Public client configuration `GET /game/client-config` (`ClientConfig` in specs/openapi/eduworld-api.v1.yaml).
## F060 only parses it (contract test, F060-AC-22); F061 fetches it during VersionCheck.

var min_client_version: String = ""
var latest_client_version: String = ""
var update_required: bool = false
var maintenance_active: bool = false
var maintenance_until: String = ""
var login_url: String = ""
var world_manifest_url: String = ""
var realtime_url: String = ""
var feature_flags: Dictionary = {}
var supported_locales: PackedStringArray = []


## Parses a decoded JSON body; returns null when a required field is missing or has the wrong shape.
static func from_dict(data: Dictionary) -> ClientConfig:
	if not data.has("min_client_version") or not data.get("maintenance") is Dictionary:
		return null
	var maintenance: Dictionary = data["maintenance"] as Dictionary
	if not maintenance.get("active") is bool:
		return null
	var config: ClientConfig = ClientConfig.new()
	config.min_client_version = String(data["min_client_version"])
	config.latest_client_version = String(data.get("latest_client_version", ""))
	config.update_required = bool(data.get("update_required", false))
	config.maintenance_active = maintenance["active"] as bool
	config.maintenance_until = String(maintenance.get("until", ""))
	var login: Dictionary = data.get("login", {}) as Dictionary
	config.login_url = String(login.get("login_url", ""))
	config.world_manifest_url = String(data.get("world_manifest_url", ""))
	config.realtime_url = String(data.get("realtime_url", ""))
	var flags: Variant = data.get("feature_flags", {})
	if flags is Dictionary:
		for key: Variant in (flags as Dictionary).keys():
			config.feature_flags[String(key)] = bool((flags as Dictionary)[key])
	for locale: Variant in (data.get("supported_locales", []) as Array):
		if L10n.is_supported(String(locale), false):
			config.supported_locales.append(String(locale))
	return config
