class_name UpdateService
extends RefCounted
## "Cần cập nhật" decision (F133 T023, F133-AC-12, NFR-COMP-002, TC-AUTH-066, research R1, ADR-0101): when the server
## refuses this client (`426 CLIENT_VERSION_UNSUPPORTED`) or `getClientConfig` says `update_required` / a higher
## `min_client_version`, emit `update_required(action, url)` with the way to update on this platform:
##   - Android/iOS: open the store page — the server `store_url` of the 426, else the store link of the build config
##     (`Config.store_urls`, written by gen-config.mjs from the GitHub Environment; never typed in code — R3);
##   - a shared Android device (school tablet, `device.cfg` `shared_device`): ask the teacher — the school installs the
##     signed school kit through MDM/USB, a student cannot install apps there;
##   - Web: reload the page (`index.html` of the CDN points to the current version — DEPLOYMENT §9);
##   - no https link, or an unreleased OS (editor/debug): ask the teacher. No Windows self-update (ADR-0101).
## Domain/application class without scene; UpdateRequired.tscn only shows the result and calls `perform` (R1).

signal update_required(action: int, url: String)

enum Action { OPEN_STORE, RELOAD_PAGE, ASK_TEACHER }

const CODE_UNSUPPORTED: String = "CLIENT_VERSION_UNSUPPORTED"
const URL_MAX: int = 2048
const STORE_PLATFORMS: PackedStringArray = ["android", "ios"]

var platform: String = "other"
var shared_device: bool = true
var store_urls: Dictionary = {}
var min_client_version: String = ""
var server_store_url: String = ""
## `(url: String) -> void` opening an external page (default OS.shell_open; tests replace it).
var opener: Callable = Callable()
## `() -> void` reloading the Web page (default JavaScriptBridge; tests replace it).
var reloader: Callable = Callable()


func _init(for_platform: String = "other", shared: bool = true, stores: Dictionary = {}) -> void:
	platform = for_platform
	shared_device = shared
	store_urls = stores


## Service of this process from the Config autoload (platform, shared device, store links).
static func from_config(config: Node) -> UpdateService:
	if config == null:
		return UpdateService.new()
	var key: String = Config.platform_key(OS.get_name(), OS.has_feature("web"))
	return UpdateService.new(key, bool(config.get("shared_device")), config.get("store_urls") as Dictionary)


static func is_safe_link(url: String) -> bool:
	return url.begins_with("https://") and url.length() <= URL_MAX and not url.contains(" ")


## {action, url} for a platform (pure).
static func resolve(for_platform: String, shared: bool, server_url: String, stores: Dictionary) -> Dictionary:
	if for_platform == "web":
		return {"action": Action.RELOAD_PAGE, "url": ""}
	if not for_platform in STORE_PLATFORMS or (shared and for_platform == "android"):
		return {"action": Action.ASK_TEACHER, "url": ""}
	for url: String in [server_url, String(stores.get(for_platform, ""))]:
		if is_safe_link(url):
			return {"action": Action.OPEN_STORE, "url": url}
	return {"action": Action.ASK_TEACHER, "url": ""}


func current() -> Dictionary:
	return resolve(platform, shared_device, server_store_url, store_urls)


## ClientConfig of VersionCheck: emits and returns true when this version must be updated.
func check_client_config(cfg: ClientConfig, client_version: String) -> bool:
	if cfg == null or VersionCheck.evaluate(cfg, client_version) != VersionCheck.Outcome.FORCE_UPDATE:
		return false
	min_client_version = cfg.min_client_version
	_emit()
	return true


## Any API result: `426 CLIENT_VERSION_UNSUPPORTED` emits (with the server store link) and returns true.
func check_result(result: ApiResult) -> bool:
	if result == null or (result.status != 426 and result.code != CODE_UNSUPPORTED):
		return false
	if result.problem != null:
		min_client_version = result.problem.min_client_version
		server_store_url = result.problem.store_url
	_emit()
	return true


## Runs the action of the button; false when nothing could be done (no safe link, not on Web, ask the teacher).
func perform(action: int, url: String = "") -> bool:
	match action:
		Action.OPEN_STORE:
			if not is_safe_link(url):
				return false
			if opener.is_valid():
				opener.call(url)
			else:
				OS.shell_open(url)
			return true
		Action.RELOAD_PAGE:
			if reloader.is_valid():
				reloader.call()
				return true
			if OS.has_feature("web"):
				JavaScriptBridge.eval("window.location.reload()")
				return true
	return false


func _emit() -> void:
	var r: Dictionary = current()
	update_required.emit(int(r["action"]), String(r["url"]))
