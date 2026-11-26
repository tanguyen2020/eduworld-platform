class_name XpEntryModel
extends RefCounted
## One line of "Hành trình của tôi" (`XpLedgerEntry`, PROGRESSION_SYSTEM §4.6): how many XP, from which kind of
## activity, when. No technical id is kept or shown (F069-AC-05); the label is the translation of
## `ui.journey.source.<source_code>`, else the server's friendly `source_label`.

const KEY_PREFIX: String = "ui.journey.source."
## Translation keys of the XP source catalog (PROGRESSION_SYSTEM §3.1), listed literally for tools/extract_pot.gd.
const SOURCE_KEYS: PackedStringArray = [
	"ui.journey.source.quest.main_completed", "ui.journey.source.quest.side_completed",
	"ui.journey.source.quest.daily_completed", "ui.journey.source.quest.weekly_completed",
	"ui.journey.source.activity.completed", "ui.journey.source.mastery.objective_mastered",
	"ui.journey.source.lesson.completed", "ui.journey.source.exploration.discovery",
	"ui.journey.source.exploration.area_completed", "ui.journey.source.challenge.completed",
	"ui.journey.source.boss.defeated", "ui.journey.source.coop.contribution",
	"ui.journey.source.achievement.unlocked", "ui.journey.source.event.participation",
	"ui.journey.source.competition.participation",
]

var amount: int = 0
var source_code: String = ""
var source_label: Dictionary = {}
var capped: bool = false
var created_at: String = ""


static func from_dict(d: Variant) -> XpEntryModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	if not m.has("amount"):
		return null
	var e: XpEntryModel = XpEntryModel.new()
	e.amount = DataRead.int_of(m, "amount")
	e.source_code = DataRead.str_of(m, "source_code")
	e.source_label = DataRead.text_of(m, "source_label")
	e.capped = DataRead.bool_of(m, "capped")
	e.created_at = DataRead.str_of(m, "created_at")
	return e


static func list_from(v: Variant) -> Array[XpEntryModel]:
	var out: Array[XpEntryModel] = []
	if v is Array:
		for d: Variant in v as Array:
			var e: XpEntryModel = from_dict(d)
			if e != null:
				out.append(e)
	return out


## Friendly label: the catalog key of the source code, else the server label, else a generic line.
func label() -> String:
	var key: String = KEY_PREFIX + source_code
	if not source_code.is_empty() and L10n.has_key(key):
		return TranslationServer.translate(key)
	var server: String = L10n.pick(source_label)
	return server if not server.is_empty() else TranslationServer.translate("ui.journey.source.other")
