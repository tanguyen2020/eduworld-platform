class_name QualityPolicy
extends RefCounted
## Pure rules of `QualityManager` (FR-GAME-009, NFR-COMP-001, ADR-0057). Every value is the owner document's
## (code-scoped CONFIG_DEFAULTS entries): presets `quality_presets` (PERFORMANCE_BUDGET §5), renderer
## `renderer_default` (§4), frame targets `target_fps` (§3), dynamic resolution `dynamic_resolution` (§6) and
## idle/battery `battery_target` (§12). No I/O, so every rule is unit-tested.

const LOW: String = "low"
const MEDIUM: String = "medium"
const HIGH: String = "high"
const PRESET_NAMES: PackedStringArray = ["low", "medium", "high"]
## Godot rendering methods and their `RendererKind` (OpenAPI) names.
const RENDERER_MOBILE: String = "mobile"
const RENDERER_COMPATIBILITY: String = "gl_compatibility"
const RENDERER_FORWARD_PLUS: String = "forward_plus"
const RENDERER_KINDS: Dictionary = {
	"mobile": "mobile", "gl_compatibility": "compatibility", "forward_plus": "forward_plus",
}
## PERFORMANCE_BUDGET §5 (`quality_presets`): base/dynamic render scale range, FPS cap (mobile·web / PC),
## shadows, anti-aliasing (none/fxaa/msaa2), visibility range (m), `mesh_lod_threshold`.
const PRESETS: Dictionary = {
	"low": {
		"render_scale": 0.75, "scale_min": 0.6, "scale_max": 0.75, "fps_cap_mobile": 30, "fps_cap_pc": 30,
		"shadows": false, "aa": "none", "view_distance": 40.0, "lod_threshold": 4.0,
	},
	"medium": {
		"render_scale": 0.9, "scale_min": 0.7, "scale_max": 1.0, "fps_cap_mobile": 30, "fps_cap_pc": 60,
		"shadows": true, "aa": "fxaa", "view_distance": 70.0, "lod_threshold": 2.0,
	},
	"high": {
		"render_scale": 1.0, "scale_min": 1.0, "scale_max": 1.0, "fps_cap_mobile": 30, "fps_cap_pc": 60,
		"shadows": true, "aa": "msaa2", "view_distance": 120.0, "lod_threshold": 1.0,
	},
}
## `battery_target`: 3D renders at 15 FPS while the screen is idle for more than 10 s (§12).
const IDLE_FPS: int = 15
const IDLE_AFTER_SEC: float = 10.0
## Benchmark (§6): ≤ 5 s. [Proposed] mapping of the measured p10 FPS at the medium preset to a preset: ≥ 1.5 × the
## target → high (PC only), ≥ 0.9 × target (p10 ≥ 27 at a 30 FPS target, §3) → medium, else low.
const BENCHMARK_MAX_SEC: float = 5.0
const HIGH_HEADROOM: float = 1.5
const MEDIUM_HEADROOM: float = 0.9


## Device description matched against `DeviceProfile.match` (no PII: GPU and model names only, never sent).
static func device_info(platform: String) -> Dictionary:
	return {
		"platform": platform,
		"gpu_vendor": RenderingServer.get_video_adapter_vendor(),
		"gpu_model": RenderingServer.get_video_adapter_name(),
		"device_model": OS.get_model_name(),
		"os_version": OS.get_version(),
	}


static func is_valid_preset(preset: String) -> bool:
	return preset in PRESET_NAMES


static func preset_value(preset: String, key: String) -> Variant:
	var p: Dictionary = PRESETS.get(preset if is_valid_preset(preset) else MEDIUM, {}) as Dictionary
	return p.get(key)


## Mobile, Web and tablets lock 30 FPS; PC may run 60 (`target_fps`, §3).
static func is_mobile_platform(platform: String) -> bool:
	return platform in ["android", "ios", "web"]


static func fps_cap(preset: String, platform: String) -> int:
	return int(preset_value(preset, "fps_cap_mobile" if is_mobile_platform(platform) else "fps_cap_pc"))


static func target_frame_ms(fps: int) -> float:
	return 1000.0 / float(maxi(fps, 1))


## `renderer_default` (§4): Web → Compatibility; PC/Android/iOS → Mobile; Forward+ only for the high preset on PC.
static func default_renderer(platform: String, preset: String = MEDIUM) -> String:
	if platform == "web":
		return RENDERER_COMPATIBILITY
	if preset == HIGH and not is_mobile_platform(platform):
		return RENDERER_FORWARD_PLUS
	return RENDERER_MOBILE


static func renderer_kind(rendering_method: String) -> String:
	return String(RENDERER_KINDS.get(rendering_method, ""))


## F062-AC-14: preset from the benchmark p10 FPS (measured with the medium preset). Mobile/Web never pick high.
static func choose_preset(fps_p10: float, platform: String) -> String:
	var target: float = float(fps_cap(MEDIUM, platform))
	if fps_p10 >= target * HIGH_HEADROOM and not is_mobile_platform(platform):
		return HIGH
	if fps_p10 >= target * MEDIUM_HEADROOM:
		return MEDIUM
	return LOW


## Percentile (0–100) of frame times in ms, nearest-rank; 0 when empty.
static func percentile(values: PackedFloat32Array, pct: float) -> float:
	if values.is_empty():
		return 0.0
	var sorted: PackedFloat32Array = values.duplicate()
	sorted.sort()
	var rank: int = clampi(int(ceilf(pct / 100.0 * float(sorted.size()))) - 1, 0, sorted.size() - 1)
	return sorted[rank]


## FPS p10 = FPS of the 90th percentile frame time (the 10% slowest frames, GAME_ANALYTICS `fps_p10`).
static func fps_p10(frame_ms: PackedFloat32Array) -> float:
	var slow: float = percentile(frame_ms, 90.0)
	return 1000.0 / slow if slow > 0.0 else 0.0


static func fps_p50(frame_ms: PackedFloat32Array) -> float:
	var mid: float = percentile(frame_ms, 50.0)
	return 1000.0 / mid if mid > 0.0 else 0.0


## F062-AC-15: first `client-config.device_profiles` entry matching the device, or {} (`DeviceProfile.match`:
## platform, gpu_vendor, gpu_model_pattern, device_model_pattern, os_version_max — all given keys must match;
## patterns are case-insensitive regular expressions, an invalid pattern never matches).
static func match_device_profile(profiles: Array, device: Dictionary) -> Dictionary:
	for item: Variant in profiles:
		if not item is Dictionary:
			continue
		var profile: Dictionary = item as Dictionary
		var match_rule: Variant = profile.get("match")
		if not match_rule is Dictionary or not is_valid_preset(String(profile.get("preset", ""))):
			continue
		if _matches(match_rule as Dictionary, device):
			return profile
	return {}


## `RendererKind` of a profile → Godot rendering method ("" when unknown).
static func rendering_method_of(kind: String) -> String:
	for method: Variant in RENDERER_KINDS.keys():
		if String(RENDERER_KINDS[method]) == kind:
			return String(method)
	return ""


## F062-AC-17: the previous run started the renderer but never drew a frame → use Compatibility next time.
static func renderer_after_boot(saved_renderer: String, boot_pending: bool) -> String:
	if boot_pending and saved_renderer != RENDERER_COMPATIBILITY:
		return RENDERER_COMPATIBILITY
	return saved_renderer


## Effective FPS cap: the preset cap, one level lower under thermal pressure (§6), 15 FPS when idle (§12).
static func effective_fps_cap(preset_cap: int, thermal_throttled: bool, idle_sec: float) -> int:
	var cap: int = preset_cap
	if thermal_throttled and cap > 30:
		cap = 30
	if idle_sec > IDLE_AFTER_SEC:
		cap = mini(cap, IDLE_FPS)
	return cap


static func _matches(rule: Dictionary, device: Dictionary) -> bool:
	if rule.has("platform") and String(rule["platform"]).to_lower() != String(device.get("platform", "")).to_lower():
		return false
	if rule.has("gpu_vendor") and not String(device.get("gpu_vendor", "")).to_lower().contains(String(rule["gpu_vendor"]).to_lower()):
		return false
	for pair: Array in [["gpu_model_pattern", "gpu_model"], ["device_model_pattern", "device_model"]]:
		if rule.has(pair[0]) and not _regex_matches(String(rule[pair[0]]), String(device.get(pair[1], ""))):
			return false
	if rule.has("os_version_max") and _version_greater(String(device.get("os_version", "")), String(rule["os_version_max"])):
		return false
	return true


static func _regex_matches(pattern: String, value: String) -> bool:
	if pattern.is_empty():
		return true
	var re: RegEx = RegEx.new()
	if re.compile("(?i)" + pattern) != OK:
		return false
	return re.search(value) != null


## Numeric dotted comparison (`"13.1" > "13"`); a non-numeric part compares as 0.
static func _version_greater(a: String, b: String) -> bool:
	var pa: PackedStringArray = a.split(".")
	var pb: PackedStringArray = b.split(".")
	for i: int in maxi(pa.size(), pb.size()):
		var x: int = pa[i].to_int() if i < pa.size() else 0
		var y: int = pb[i].to_int() if i < pb.size() else 0
		if x != y:
			return x > y
	return false
