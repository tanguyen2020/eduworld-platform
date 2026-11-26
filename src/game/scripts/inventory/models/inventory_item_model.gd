class_name InventoryItemModel
extends RefCounted
## An item the student owns (`InventoryItem`, INVENTORY_ECONOMY §2–§3). Ownership and equip slots stay with the server
## (`ITEM_NOT_OWNED`, `EQUIP_SLOT_INVALID`); this is display data for the Tủ đồ grid.

var item_id: String = ""
var kind: String = ""
var rarity: String = ""
var equip_slot: String = ""
var quantity: int = 1
var acquired_at: String = ""
var name: Dictionary = {}


static func from_dict(d: Variant) -> InventoryItemModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	var id: String = DataRead.str_of(m, "item_id")
	if not ItemVisual.is_valid_code(id):
		return null
	var i: InventoryItemModel = InventoryItemModel.new()
	i.item_id = id
	i.kind = DataRead.str_of(m, "kind")
	i.rarity = DataRead.str_of(m, "rarity", "common")
	i.equip_slot = DataRead.str_of(m, "equip_slot")
	i.quantity = maxi(1, DataRead.int_of(m, "quantity", 1))
	i.acquired_at = DataRead.str_of(m, "acquired_at")
	i.name = DataRead.text_of(m, "name")
	return i


static func list_from(v: Variant) -> Array[InventoryItemModel]:
	var out: Array[InventoryItemModel] = []
	if v is Array:
		for d: Variant in v as Array:
			var i: InventoryItemModel = from_dict(d)
			if i != null:
				out.append(i)
	return out


func display_name() -> String:
	var n: String = L10n.pick(name)
	return n if not n.is_empty() else TranslationServer.translate("ui.reward.item")
