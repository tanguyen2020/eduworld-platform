extends RefCounted
## Art rules of the game project (F067; ADR-0102 asset licences; AGENTS R32 / DESIGN_SYSTEM §5.3 art palette).
## Text based so the CLI (tools/lint_art.gd) runs headless without autoloads:
## - licence: every graphic asset file has a row in res://assets/LICENSES.md (path glob, source, author, licence,
##   date) whose licence is allowed (MIT, CC0-1.0, Public-Domain; OFL-1.1 only for fonts) and never share-alike,
##   non-commercial, no-derivatives, GPL family or proprietary;
## - palette: 3D art sources carry no colour literal — colours come from the generated tokens (art palette names,
##   material presets, Environment), so `Color(`, `Color8(`, `Color.html(` and hex colours are violations there
##   (a line marked `art-lint: not a colour` packs data in a Color, e.g. smoothed normals).

const LICENSES_PATH: String = "res://assets/LICENSES.md"
const ALLOWED_LICENSES: PackedStringArray = ["MIT", "CC0-1.0", "Public-Domain"]
const FONT_LICENSES: PackedStringArray = ["OFL-1.1"]
const FONT_EXTENSIONS: PackedStringArray = ["ttf", "otf", "woff", "woff2"]
## Asset extensions checked anywhere in the project (outside SKIPPED_PREFIXES), plus every file under res://assets.
const ASSET_EXTENSIONS: PackedStringArray = [
	"glb", "gltf", "fbx", "obj", "blend", "png", "jpg", "jpeg", "webp", "svg", "ktx", "ogg", "wav", "mp3", "ttf", "otf",
	"woff", "woff2", "gdshader",
]
const SKIPPED_PREFIXES: PackedStringArray = ["res://tests/", "res://addons/", "res://.godot/", "res://reports/", "res://build/"]
## Words of a licence string that are never allowed (case-insensitive, matched as whole tokens).
const FORBIDDEN_TOKENS: PackedStringArray = [
	"SA", "NC", "ND", "GPL", "LGPL", "AGPL", "GPL-2.0", "GPL-3.0", "PROPRIETARY", "COMMERCIAL", "PAID", "STORE",
	"EDITORIAL", "RESERVED",
]
## 3D art sources where colour literals are forbidden (prefix match).
const PALETTE_SCOPES: PackedStringArray = [
	"res://scenes/world/", "res://scenes/npc/", "res://scenes/character/AvatarRig.tscn", "res://resources/art/",
	"res://resources/items/", "res://assets/", "res://tools/art/",
]
const PALETTE_SKIP: PackedStringArray = ["res://resources/art/generated/"]
## Marks a line whose Color is data, not a visible colour (e.g. smoothed normals packed in the COLOR array).
const ALLOW_MARK: String = "art-lint: not a colour"
const COLOR_PATTERN: String = "(\\bColor8?\\s*\\(|\\bColor\\.html\\s*\\(|\"#[0-9A-Fa-f]{6}([0-9A-Fa-f]{2})?\")"


## Rows of LICENSES.md: [{glob, source, author, license, date, line}].
static func parse_licenses(text: String) -> Array[Dictionary]:
	var rows: Array[Dictionary] = []
	var lines: PackedStringArray = text.split("\n")
	for i: int in lines.size():
		var line: String = lines[i].strip_edges()
		if not line.begins_with("|") or line.begins_with("|---") or line.begins_with("| Path"):
			continue
		var cells: PackedStringArray = line.trim_prefix("|").trim_suffix("|").split("|")
		if cells.size() < 5:
			continue
		rows.append({
			"glob": cells[0].strip_edges().trim_prefix("`").trim_suffix("`"), "source": cells[1].strip_edges(),
			"author": cells[2].strip_edges(), "license": cells[3].strip_edges(), "date": cells[4].strip_edges(),
			"line": i + 1,
		})
	return rows


## Problems of one licence string for a file of extension `ext` (empty = allowed).
static func license_problem(license: String, ext: String) -> String:
	var spdx: String = license.strip_edges()
	for token: String in _tokens(spdx):
		if token in FORBIDDEN_TOKENS:
			return "licence '%s' is forbidden (%s; ADR-0102)" % [spdx, token]
	if spdx in ALLOWED_LICENSES:
		return ""
	if spdx in FONT_LICENSES:
		return "" if ext in FONT_EXTENSIONS or ext == "txt" else "licence '%s' is allowed for fonts only" % spdx
	return "licence '%s' is not in the allow list %s (ADR-0102)" % [spdx, ", ".join(ALLOWED_LICENSES)]


static func _tokens(spdx: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var normal: String = spdx.to_upper()
	for sep: String in [" ", "/", ",", "(", ")", "+"]:
		normal = normal.replace(sep, "|")
	for part: String in normal.split("|", false):
		out.append(part)
		for piece: String in part.split("-", false):
			out.append(piece)
	return out


## Glob match of a project-relative path (`assets/models/**`, `resources/x/*.png`).
static func glob_matches(glob: String, rel_path: String) -> bool:
	var pattern: String = glob.replace("**", "\u0001").replace("*", "\u0002")
	var re: String = "^"
	for c: String in pattern:
		match c:
			"\u0001":
				re += ".*"
			"\u0002":
				re += "[^/]*"
			".", "(", ")", "[", "]", "+", "?", "^", "$", "|", "{", "}", "\\":
				re += "\\" + c
			_:
				re += c
	return RegEx.create_from_string(re + "$").search(rel_path) != null


## Asset files that need a licence row, from a list of `res://` paths.
static func asset_files(paths: PackedStringArray) -> PackedStringArray:
	var out: PackedStringArray = []
	for path: String in paths:
		var skipped: bool = false
		for prefix: String in SKIPPED_PREFIXES:
			if path.begins_with(prefix):
				skipped = true
		if skipped or path == LICENSES_PATH or path.ends_with(".import") or path.ends_with(".uid"):
			continue
		if path.begins_with("res://assets/") or path.get_extension() in ASSET_EXTENSIONS:
			out.append(path)
	return out


## Licence violations of `files` against the LICENSES.md `rows`.
static func check_licenses(files: PackedStringArray, rows: Array[Dictionary]) -> PackedStringArray:
	var out: PackedStringArray = []
	for row: Dictionary in rows:
		for field: String in ["source", "author", "license", "date"]:
			if String(row[field]).is_empty():
				out.append("%s:%d row '%s' has no %s" % [LICENSES_PATH, int(row["line"]), row["glob"], field])
	for path: String in files:
		var rel: String = path.trim_prefix("res://")
		var row: Dictionary = {}
		for candidate: Dictionary in rows:
			if glob_matches(String(candidate["glob"]), rel):
				row = candidate
				break
		if row.is_empty():
			out.append("%s has no licence row in %s (ADR-0102)" % [path, LICENSES_PATH])
			continue
		var problem: String = license_problem(String(row["license"]), path.get_extension())
		if not problem.is_empty():
			out.append("%s: %s (%s:%d)" % [path, problem, LICENSES_PATH, int(row["line"])])
	return out


static func in_palette_scope(path: String) -> bool:
	for prefix: String in PALETTE_SKIP:
		if path.begins_with(prefix):
			return false
	for prefix: String in PALETTE_SCOPES:
		if path.begins_with(prefix):
			return true
	return false


## Colour literals in a 3D art source (comments of .gd/.gdshader and .tscn/.tres `;` lines ignored).
static func check_palette(path: String, text: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var re: RegEx = RegEx.create_from_string(COLOR_PATTERN)
	var lines: PackedStringArray = text.split("\n")
	for i: int in lines.size():
		var line: String = lines[i]
		if line.contains(ALLOW_MARK):
			continue
		var code: String = line
		match path.get_extension():
			"gd":
				var cut: int = line.find("#")
				code = line.substr(0, cut) if cut >= 0 and not line.contains("\"#") else line
			"gdshader":
				var slash: int = line.find("//")
				code = line.substr(0, slash) if slash >= 0 else line
			_:
				if line.strip_edges().begins_with(";"):
					continue
		if re.search(code) != null:
			out.append("%s:%d colour literal (use the art palette / generated presets, R32)" % [path, i + 1])
	return out
