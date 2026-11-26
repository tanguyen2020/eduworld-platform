class_name CollectionModel
extends RefCounted
## A collection and its items (`CollectionProgress`, `CollectionDetail`; INVENTORY_ECONOMY §6): owned items are drawn
## in colour, missing ones with the same picture greyed plus the words "Chưa có" and the public, deterministic way to
## get them (`how_to_obtain` — never a paid random source, AGENTS R19). Display data only.

## Translation keys of the contract collection types, listed literally for tools/extract_pot.gd.
const TYPE_KEYS: PackedStringArray = [
	"ui.collection.type.pet", "ui.collection.type.knowledge_card", "ui.collection.type.cosmetic",
	"ui.collection.type.badge", "ui.collection.type.exploration", "ui.collection.type.event",
]

var collection_id: String = ""
var code: String = ""
var title: Dictionary = {}
var type: String = ""
var owned_count: int = 0
var total_count: int = 0
var completed_at: String = ""
## Items of the detail: `{item_id, name: Dictionary, rarity, kind, owned, how_to_obtain: Dictionary}`.
var items: Array[Dictionary] = []
var completion_rewards: Array[RewardModel] = []


static func from_dict(d: Variant) -> CollectionModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	var c: CollectionModel = CollectionModel.new()
	c.collection_id = DataRead.str_of(m, "collection_id")
	if c.collection_id.is_empty():
		return null
	c.code = DataRead.str_of(m, "code")
	c.title = DataRead.text_of(m, "title")
	c.type = DataRead.str_of(m, "type")
	c.owned_count = maxi(0, DataRead.int_of(m, "owned_count"))
	c.total_count = maxi(0, DataRead.int_of(m, "total_count"))
	c.completed_at = DataRead.str_of(m, "completed_at")
	for raw: Variant in DataRead.arr_of(m, "items"):
		if not raw is Dictionary:
			continue
		var it: Dictionary = raw as Dictionary
		var item_id: String = DataRead.str_of(it, "item_id")
		if not ItemVisual.is_valid_code(item_id):
			continue
		c.items.append({"item_id": item_id, "name": DataRead.text_of(it, "name"),
			"rarity": DataRead.str_of(it, "rarity", "common"), "kind": DataRead.str_of(it, "kind"),
			"owned": DataRead.bool_of(it, "owned"), "how_to_obtain": DataRead.text_of(it, "how_to_obtain")})
	c.completion_rewards = RewardModel.list_from(m.get("completion_rewards"))
	return c


static func list_from(v: Variant) -> Array[CollectionModel]:
	var out: Array[CollectionModel] = []
	if v is Array:
		for d: Variant in v as Array:
			var c: CollectionModel = from_dict(d)
			if c != null:
				out.append(c)
	return out


func fraction() -> float:
	return clampf(float(owned_count) / float(total_count), 0.0, 1.0) if total_count > 0 else 0.0


func progress_text() -> String:
	return "%s/%s" % [LocaleFormat.number(owned_count), LocaleFormat.number(total_count)]
