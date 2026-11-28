class_name BossSummary
extends RefCounted
## One boss of `listBosses` (OpenAPI `BossSummary`; F110 data-model §1): title (`LocalizedName`), phase count, the modes
## the server allows (`solo`, `party`), party size bounds and whether the student already beat it. Unknown fields are
## ignored; a boss without a valid id, code or title is dropped (GAME_NETWORKING §9).

const MODE_SOLO: String = "solo"
const MODE_PARTY: String = "party"

var boss_id: String = ""
var code: String = ""
var title: Dictionary = {}
var area_id: String = ""
var phase_count: int = 1
var modes: PackedStringArray = []
var party_size_min: int = 1
var party_size_max: int = 1
var defeated_before: bool = false


static func from_dict(data: Variant) -> BossSummary:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "boss_id")
	var t: Dictionary = DataRead.text_of(d, "title")
	if not Uuid.is_valid(id) or DataRead.str_of(d, "code").is_empty() or t.is_empty():
		return null
	var b: BossSummary = BossSummary.new()
	b.boss_id = id
	b.code = DataRead.str_of(d, "code").left(64)
	b.title = t
	b.area_id = DataRead.str_of(d, "area_id").left(64)
	b.phase_count = clampi(DataRead.int_of(d, "phase_count", 1), 1, 10)
	for m: String in DataRead.strings_of(d, "modes", 2):
		if m in [MODE_SOLO, MODE_PARTY] and not m in b.modes:
			b.modes.append(m)
	b.party_size_min = maxi(1, DataRead.int_of(d, "party_size_min", 1))
	b.party_size_max = clampi(DataRead.int_of(d, "party_size_max", b.party_size_min), b.party_size_min, 8)
	b.defeated_before = DataRead.bool_of(d, "defeated_before")
	return b


static func list_from(data: Variant) -> Array[BossSummary]:
	var out: Array[BossSummary] = []
	if data is Array:
		for raw: Variant in data as Array:
			var b: BossSummary = from_dict(raw)
			if b != null:
				out.append(b)
	return out


func allows_solo() -> bool:
	return MODE_SOLO in modes


func allows_party() -> bool:
	return MODE_PARTY in modes


func display_title(locale: String = "") -> String:
	return L10n.pick(title, locale)
