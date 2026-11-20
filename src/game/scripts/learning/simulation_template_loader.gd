class_name SimulationTemplateLoader
extends RefCounted
## Finds the packaged scene of a simulation template version (F066-AC-21, GODOT_ARCHITECTURE §6.2, §8): templates
## shipped in the build, or a PCK listed in the manifest whose `sha256` must match before `load_resource_pack`. A
## missing template or a version this client does not have → `update_required` (never a crash).

const REASON_OK: String = ""
const REASON_MISSING: String = "update_required"
const REASON_BAD_PACK: String = "pack_mismatch"

## `template_code` → {template_version: scene path} of the templates in the build.
var builtin: Dictionary = {
	"physics.lever": {1: "res://scenes/learning/simulation/templates/physics_lever/v1/LeverTemplate.tscn"},
}
## `template_code@version` → {pck: path, sha256: hex, scene: res path} of downloaded packs (F133 distributes them).
var packs: Dictionary = {}
var last_reason: String = REASON_OK


func scene_path(code: String, version: int) -> String:
	var versions: Dictionary = builtin.get(code, {}) as Dictionary
	return String(versions.get(version, ""))


## The template scene, null with `last_reason` when it cannot be loaded.
func load_template(code: String, version: int) -> PackedScene:
	last_reason = REASON_OK
	var key: String = "%s@%d" % [code, version]
	if packs.has(key):
		var p: Dictionary = packs[key] as Dictionary
		var path: String = String(p.get("pck", ""))
		if not FileAccess.file_exists(path) or FileAccess.get_sha256(path) != String(p.get("sha256", "")).to_lower():
			last_reason = REASON_BAD_PACK
			return null
		if not ProjectSettings.load_resource_pack(path, false):
			last_reason = REASON_BAD_PACK
			return null
		return _load(String(p.get("scene", "")))
	return _load(scene_path(code, version))


func _load(path: String) -> PackedScene:
	if path.is_empty() or not ResourceLoader.exists(path):
		last_reason = REASON_MISSING
		return null
	var scene: PackedScene = load(path) as PackedScene
	if scene == null:
		last_reason = REASON_MISSING
	return scene
