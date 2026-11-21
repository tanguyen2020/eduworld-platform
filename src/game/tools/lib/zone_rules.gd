extends RefCounted
## Rules of a zone scene (WORLD_RENDERING §10, F071-AC-13, T023), used by `tools/zone_lint.gd` and the GdUnit tests.
## Text based (the .tscn is parsed, not instantiated) so the CLI runs headless without the autoloads: the root uses
## zone_root.gd with a known `area_id` and a default spawn; SpawnPoint / Interactable / AreaPortal ids are unique and
## well formed; a portal targets a known area; an NPC has `npc.<code>`; no inline material (materials only from the
## generated design-token presets in resources/art/generated, ADR-0047, R32); the automated part of the content safety
## checklist: no node or id naming a weapon, violence or an unsuitable symbol (the visual review stays manual).

const ZONES_DIR: String = "res://scenes/world/zones"
const ZONE_SCRIPT: String = "res://scripts/world/zone_root.gd"
const MATERIAL_PREFIX: String = "res://resources/art/generated/"
const MATERIAL_TYPES: PackedStringArray = ["StandardMaterial3D", "ORMMaterial3D", "ShaderMaterial", "CanvasItemMaterial"]
const ID_PATTERN: String = "^[a-z0-9_.-]{1,64}$"
## Words a zone may never name (vi/en, ASCII forms): automated part of the content safety checklist.
const UNSAFE_WORDS: PackedStringArray = ["weapon", "sword", "gun", "pistol", "rifle", "knife", "blade", "bomb", "blood", "kill",
	"skull", "grenade", "vukhi", "sung", "dao_gam", "kiem"]


## Zone scene paths (one Zone.tscn per area directory).
static func zone_paths() -> PackedStringArray:
	var out: PackedStringArray = []
	var dir: DirAccess = DirAccess.open(ZONES_DIR)
	if dir == null:
		return out
	for sub: String in dir.get_directories():
		var path: String = "%s/%s/Zone.tscn" % [ZONES_DIR, sub]
		if FileAccess.file_exists(path):
			out.append(path)
	return out


static func _value(line: String) -> String:
	var v: String = line.substr(line.find("=") + 1).strip_edges()
	return v.trim_prefix("\"").trim_suffix("\"")


## Violations of the zone at `path`; `known_areas` = valid area ids (data of the server, e.g. the area fixture).
static func check(path: String, known_areas: PackedStringArray) -> PackedStringArray:
	var errors: PackedStringArray = []
	var text: String = FileAccess.get_file_as_string(path)
	if text.is_empty():
		return PackedStringArray(["%s: unreadable" % path])
	var ext: Dictionary = {}
	var re_ext: RegEx = RegEx.create_from_string("\\[ext_resource [^\\]]*path=\"([^\"]+)\"[^\\]]*id=\"([^\"]+)\"")
	for m: RegExMatch in re_ext.search_all(text):
		ext[m.get_string(2)] = m.get_string(1)
	var re_id: RegEx = RegEx.create_from_string(ID_PATTERN)
	var re_name: RegEx = RegEx.create_from_string("name=\"([^\"]+)\"")
	var seen: Dictionary = {}
	var area_id: String = ""
	var default_spawn: String = ""
	var spawns: PackedStringArray = []
	var root_script: String = ""
	var in_root: bool = false
	for raw: String in text.split("\n"):
		var line: String = raw.strip_edges()
		if line.begins_with("[node "):
			in_root = not line.contains(" parent=")
			var name_match: RegExMatch = re_name.search(line)
			if name_match != null:
				errors.append_array(_unsafe(path, name_match.get_string(1)))
			continue
		if line.begins_with("[sub_resource ") or line.begins_with("[ext_resource "):
			in_root = false
			for t: String in MATERIAL_TYPES:
				if not line.contains("type=\"%s\"" % t):
					continue
				var mp: RegExMatch = re_ext.search(line)
				if line.begins_with("[sub_resource "):
					errors.append("%s: inline %s (materials come from the token presets)" % [path, t])
				elif mp != null and not mp.get_string(1).begins_with(MATERIAL_PREFIX):
					errors.append("%s: material %s outside the token presets" % [path, mp.get_string(1)])
			continue
		if in_root and line.begins_with("script = ExtResource("):
			root_script = String(ext.get(line.get_slice("\"", 1), ""))
		elif in_root and line.begins_with("area_id ="):
			area_id = _value(line)
		elif in_root and line.begins_with("default_spawn ="):
			default_spawn = _value(line)
		elif line.begins_with("spawn_point_id =") or line.begins_with("interactable_id ="):
			var id: String = _value(line)
			if line.begins_with("spawn_point_id ="):
				spawns.append(id)
			if re_id.search(id) == null:
				errors.append("%s: invalid id %s" % [path, id])
			elif seen.has(id):
				errors.append("%s: duplicated id %s" % [path, id])
			seen[id] = true
			errors.append_array(_unsafe(path, id))
		elif line.begins_with("target_area_id =") and not _value(line) in known_areas:
			errors.append("%s: portal targets unknown area %s" % [path, _value(line)])
		elif line.begins_with("npc_id =") and not _value(line).begins_with("npc."):
			errors.append("%s: npc_id %s must be npc.<code>" % [path, _value(line)])
	if root_script != ZONE_SCRIPT:
		errors.append("%s: root is not a ZoneRoot" % path)
	if not area_id in known_areas:
		errors.append("%s: unknown area_id %s" % [path, area_id])
	if default_spawn.is_empty():
		default_spawn = area_id + ".default"
	if not default_spawn in spawns:
		errors.append("%s: no default spawn point %s" % [path, default_spawn])
	return errors


static func _unsafe(path: String, text: String) -> PackedStringArray:
	var lower: String = text.to_lower()
	for word: String in UNSAFE_WORDS:
		if RegEx.create_from_string("(^|[^a-z])%s([^a-z]|$)" % word).search(lower) != null:
			return PackedStringArray(["%s: unsafe content word '%s' in %s" % [path, word, text]])
	return PackedStringArray()
