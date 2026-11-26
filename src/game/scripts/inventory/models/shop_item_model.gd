class_name ShopItemModel
extends RefCounted
## A cosmetic on sale for Xu (`ShopItem`, INVENTORY_ECONOMY §5): price in Xu only (no real money), the purchase limit,
## owned flag and the end of sale (shown calmly, never as a countdown — AGENTS R19). Whether the student can afford
## it is only a display hint; the server decides (`INSUFFICIENT_BALANCE`, F069-AC-09).

var shop_item_id: String = ""
var item_id: String = ""
var name: Dictionary = {}
var description: Dictionary = {}
var kind: String = ""
var rarity: String = ""
var price: int = 0
var currency_code: String = ""
var purchase_limit: int = 0
var purchased_count: int = 0
var owned: bool = false
var available_to: String = ""


static func from_dict(d: Variant) -> ShopItemModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	var s: ShopItemModel = ShopItemModel.new()
	s.shop_item_id = DataRead.str_of(m, "shop_item_id")
	s.item_id = DataRead.str_of(m, "item_id")
	s.price = DataRead.int_of(m, "price")
	if not Uuid.is_valid(s.shop_item_id) or not ItemVisual.is_valid_code(s.item_id) or s.price < 1:
		return null
	s.name = DataRead.text_of(m, "name")
	s.description = DataRead.text_of(m, "description")
	s.kind = DataRead.str_of(m, "kind")
	s.rarity = DataRead.str_of(m, "rarity", "common")
	s.currency_code = DataRead.str_of(m, "currency_code")
	s.purchase_limit = maxi(0, DataRead.int_of(m, "purchase_limit"))
	s.purchased_count = maxi(0, DataRead.int_of(m, "purchased_count"))
	s.owned = DataRead.bool_of(m, "owned")
	s.available_to = DataRead.str_of(m, "available_to")
	return s


static func list_from(v: Variant) -> Array[ShopItemModel]:
	var out: Array[ShopItemModel] = []
	if v is Array:
		for d: Variant in v as Array:
			var s: ShopItemModel = from_dict(d)
			if s != null:
				out.append(s)
	return out


func display_name() -> String:
	var n: String = L10n.pick(name)
	return n if not n.is_empty() else TranslationServer.translate("ui.reward.item")


## Owned or the purchase limit is reached: the card says "Đã sở hữu" and offers no purchase.
func is_sold_out_for_me() -> bool:
	return owned or (purchase_limit > 0 and purchased_count >= purchase_limit)


## Xu still missing for `balance` (0 when affordable) — the "Cần thêm {n} Xu" hint only.
func missing_for(balance: int) -> int:
	return maxi(0, price - balance)
