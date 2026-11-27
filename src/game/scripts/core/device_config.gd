class_name DeviceConfig
extends RefCounted
## `device.cfg` of a device (F133 T036, data-model §1, F133-AC-18/19/20, ADR-0078, ADR-0086, PERFORMANCE_BUDGET §10.1).
## ConfigFile, section `[device]`, written by the school's MDM (Android managed config) or the school APK kit:
##   shared_device (bool, default per platform — CONFIG_DEFAULTS `shared_device_default`),
##   lan_cache_url (http(s) URL of the school's LAN cache, ≤ 2048, no query/credentials — otherwise ignored with the
##     warning `lan_cache_url_invalid`), low_data_mode (bool, default false),
##   preinstalled_bundles_dir (≤ 512, `res://`/`user://`/absolute; default res://preinstalled — where the school kit
##     puts its packs), grade_bands (optional subset of g1_3…g8_9 the kit carries; empty = every band).
## The Web build has no device.cfg (ADR-0101): it is always a shared device without LAN cache or preinstalled packs.
## Domain class: parsing only, no Node. Warnings name the key, never its value.

const SECTION: String = "device"
const URL_MAX: int = 2048
const DIR_MAX: int = 512
const DEFAULT_PREINSTALLED_DIR: String = "res://preinstalled"
const GRADE_BANDS: PackedStringArray = ["g1_3", "g4_5", "g6_7", "g8_9"]
const WARN_LAN: String = "lan_cache_url_invalid"
const WARN_DIR: String = "preinstalled_bundles_dir_invalid"

var platform: String = "other"
var shared_device: bool = true
var lan_cache_url: String = ""
var low_data_mode: bool = false
var preinstalled_bundles_dir: String = DEFAULT_PREINSTALLED_DIR
var grade_bands: PackedStringArray = []
var warnings: PackedStringArray = []


## Parses the text of a device.cfg for `platform` (Config keys: android, ios, web, other). Empty text = defaults.
static func parse(text: String, for_platform: String, shared_default: bool = true) -> DeviceConfig:
	var d: DeviceConfig = DeviceConfig.new()
	d.platform = for_platform
	d.shared_device = shared_default
	var cfg: ConfigFile = ConfigFile.new()
	if not text.strip_edges().is_empty() and cfg.parse(text) == OK:
		d._read(cfg)
	if for_platform == "web":
		d.shared_device = true
		d.lan_cache_url = ""
		d.preinstalled_bundles_dir = ""
	return d


## Reads `path` (missing file = defaults).
static func load_file(path: String, for_platform: String, shared_default: bool = true) -> DeviceConfig:
	var text: String = FileAccess.get_file_as_string(path) if FileAccess.file_exists(path) else ""
	return parse(text, for_platform, shared_default)


## A LAN cache base URL is `http(s)://host[:port][/path]`, ≤ 2048, without query, fragment or credentials.
static func valid_lan_cache_url(url: String) -> bool:
	if url.length() > URL_MAX:
		return false
	var re: RegEx = RegEx.create_from_string("^https?://[A-Za-z0-9.-]+(:[0-9]{1,5})?(/[A-Za-z0-9._~/-]*)?$")
	return re.search(url) != null


func _read(cfg: ConfigFile) -> void:
	var shared: Variant = cfg.get_value(SECTION, "shared_device", null)
	if shared is bool:
		shared_device = shared as bool
	var low: Variant = cfg.get_value(SECTION, "low_data_mode", null)
	if low is bool:
		low_data_mode = low as bool
	var lan: Variant = cfg.get_value(SECTION, "lan_cache_url", "")
	if lan is String and not (lan as String).is_empty():
		var url: String = (lan as String).strip_edges()
		if valid_lan_cache_url(url):
			lan_cache_url = url.trim_suffix("/")
		else:
			warnings.append(WARN_LAN)
	var dir: Variant = cfg.get_value(SECTION, "preinstalled_bundles_dir", "")
	if dir is String and not (dir as String).is_empty():
		var p: String = dir as String
		if p.length() <= DIR_MAX and not p.contains("..") \
				and (p.begins_with("res://") or p.begins_with("user://") or p.is_absolute_path()):
			preinstalled_bundles_dir = p.trim_suffix("/")
		else:
			warnings.append(WARN_DIR)
	var bands: Variant = cfg.get_value(SECTION, "grade_bands", [])
	if bands is Array or bands is PackedStringArray:
		for band: Variant in bands:
			if String(band) in GRADE_BANDS and not String(band) in grade_bands:
				grade_bands.append(String(band))
