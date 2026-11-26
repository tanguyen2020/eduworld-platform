class_name RewardModel
extends RefCounted
## One reward the server already granted or announces (`RewardGrantSummary` of specs/openapi, AsyncAPI `RewardList`;
## PROGRESSION_SYSTEM §4.4, INVENTORY_ECONOMY §5.2). Display only: the client never grants, claims or computes a
## reward (FR-RWD-003, ADR-0016) — there is no "Nhận" action anywhere.

const KIND_XP: String = "xp"
const KIND_CURRENCY: String = "currency"
const KIND_ITEM: String = "item"
const KIND_BADGE: String = "badge"
const KIND_TITLE: String = "title"
const KIND_COLLECTION_ITEM: String = "collection_item"
const KINDS: PackedStringArray = ["xp", "currency", "item", "badge", "title", "collection_item"]
## Contract `maxItems` of a reward list.
const MAX_ITEMS: int = 20

var kind: String = ""
var amount: int = 0
var currency_code: String = ""
var item_id: String = ""
var label: Dictionary = {}


static func from_dict(d: Variant) -> RewardModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	var kind_value: String = DataRead.str_of(m, "kind")
	if not kind_value in KINDS:
		return null
	var r: RewardModel = RewardModel.new()
	r.kind = kind_value
	r.amount = maxi(0, DataRead.int_of(m, "amount"))
	r.currency_code = DataRead.str_of(m, "currency_code")
	r.item_id = DataRead.str_of(m, "item_id")
	r.label = DataRead.text_of(m, "label")
	return r


static func list_from(v: Variant) -> Array[RewardModel]:
	var out: Array[RewardModel] = []
	if v is Array:
		for d: Variant in v as Array:
			var r: RewardModel = from_dict(d)
			if r != null and out.size() < MAX_ITEMS:
				out.append(r)
	return out


## Text of the reward chip: "40 XP", "50 Xu", or the item/badge/title name the server sent; a reward without a name
## shows the generic word of its kind (F069 edge case "phần thưởng chưa có visual": generic icon + name).
func display_text() -> String:
	match kind:
		KIND_XP:
			return TranslationServer.translate("ui.reward.xp").format({"amount": LocaleFormat.number(amount)})
		KIND_CURRENCY:
			return TranslationServer.translate("ui.reward.currency").format({"amount": LocaleFormat.number(amount)})
	var name: String = L10n.pick(label)
	if not name.is_empty():
		return name
	match kind:
		KIND_BADGE:
			return TranslationServer.translate("ui.reward.badge")
		KIND_TITLE:
			return TranslationServer.translate("ui.reward.title")
	return TranslationServer.translate("ui.reward.item")


## Icon of the chip (design/icons/game): coin for Xu, star for XP, medal for badges/titles, gift box otherwise.
func icon_path() -> String:
	match kind:
		KIND_XP:
			return "res://resources/ui/icons/star.svg"
		KIND_CURRENCY:
			return "res://resources/ui/icons/coin.svg"
		KIND_BADGE, KIND_TITLE:
			return "res://resources/ui/icons/medal.svg"
	return "res://resources/ui/icons/gift.svg"
