class_name TierView
extends RefCounted
## The student's competition tier for one mode × subject × season (`Rating` of `getMyRatings`,
## `competition.tier.updated`; F109-AC-15, COMPETITION_SYSTEM §8.2, ADR-0018). Only the tier is kept: a raw `rating`,
## `rd` or `rating_delta` a server bug might add is never copied, so no screen can show it (FR-RNK-003, every grade).
## The tier name is server data (Ranking Rules `tiers`, Accept-Language), never a client table.

const NAME_MAX_LENGTH: int = 128
## Fields that must never reach a screen (contract scan, test_competition_fixtures).
const FORBIDDEN_KEYS: PackedStringArray = ["rating", "rd", "rating_delta", "volatility", "mmr"]

var mode: String = ""
var subject_id: String = ""
var season_id: String = ""
var tier_code: String = ""
var tier_name: String = ""
var provisional: bool = false
var provisional_games_required: int = 1
var games_played: int = 0


static func from_dict(data: Variant) -> TierView:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var t: TierView = TierView.new()
	t.mode = DataRead.str_of(d, "mode")
	t.subject_id = DataRead.str_of(d, "subject_id")
	if t.mode.is_empty() or not Uuid.is_valid(t.subject_id) or not d.get("provisional") is bool:
		return null
	t.season_id = DataRead.str_of(d, "season_id")
	t.tier_code = DataRead.str_of(d, "tier_code").left(64)
	t.tier_name = DataRead.str_of(d, "tier_name").left(NAME_MAX_LENGTH)
	t.provisional = bool(d["provisional"])
	t.provisional_games_required = maxi(1, DataRead.int_of(d, "provisional_games_required", 1))
	t.games_played = maxi(0, DataRead.int_of(d, "games_played"))
	return t


static func list_from(raw: Variant) -> Array[TierView]:
	var out: Array[TierView] = []
	if raw is Array:
		for item: Variant in raw as Array:
			var t: TierView = from_dict(item)
			if t != null:
				out.append(t)
	return out


## JSON paths of every FORBIDDEN_KEYS entry in `value` (contract scan).
static func find_raw_rating(value: Variant, path: String = "$") -> PackedStringArray:
	var found: PackedStringArray = []
	if value is Dictionary:
		for k: Variant in (value as Dictionary).keys():
			if String(k) in FORBIDDEN_KEYS:
				found.append("%s.%s" % [path, String(k)])
			found.append_array(find_raw_rating((value as Dictionary)[k], "%s.%s" % [path, String(k)]))
	elif value is Array:
		for i: int in (value as Array).size():
			found.append_array(find_raw_rating((value as Array)[i], "%s[%d]" % [path, i]))
	return found


## Words for the badge: the server tier name, or "Đang xếp hạng (x/N)" while provisional (no number of rating).
func label() -> String:
	if provisional:
		return TranslationServer.translate("ui.arena.tier.provisional").format({
			"played": mini(games_played, provisional_games_required), "required": provisional_games_required})
	return tier_name if not tier_name.is_empty() else TranslationServer.translate("ui.arena.tier.updating")
