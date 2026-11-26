class_name LedgerEntryModel
extends RefCounted
## One line of the "Lịch sử" tab (F069-AC-12): a Xu movement (`CurrencyLedgerEntry`) or an item movement
## (`ItemLedgerEntry`), read-only. The label is `ui.inventory.reason.<reason_code>`; a duplicate converted into Xu says
## so honestly ("Đã quy đổi thành {amount} Xu", INVENTORY_ECONOMY §6, AGENTS R19).

const LEDGER_CURRENCY: String = "currency"
const LEDGER_ITEM: String = "item"
const REASON_DUPLICATE: String = "duplicate_conversion"
## Translation keys of the contract reason codes, listed literally for tools/extract_pot.gd.
const REASON_KEYS: PackedStringArray = [
	"ui.inventory.reason.reward", "ui.inventory.reason.shop_purchase", "ui.inventory.reason.collection",
	"ui.inventory.reason.adjustment", "ui.inventory.reason.refund", "ui.inventory.reason.duplicate_conversion",
]

var ledger: String = ""
var delta: int = 0
var balance_after: int = -1
var reason_code: String = ""
var item_id: String = ""
var item_name: Dictionary = {}
var kind: String = ""
var source_label: Dictionary = {}
var created_at: String = ""


static func from_dict(d: Variant, ledger_kind: String) -> LedgerEntryModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	if not m.has("delta"):
		return null
	var e: LedgerEntryModel = LedgerEntryModel.new()
	e.ledger = ledger_kind
	e.delta = DataRead.int_of(m, "delta")
	e.balance_after = DataRead.nullable_int(m, "balance_after")
	e.reason_code = DataRead.str_of(m, "reason_code")
	e.item_id = DataRead.str_of(m, "item_id")
	e.item_name = DataRead.text_of(m, "item_name")
	e.kind = DataRead.str_of(m, "kind")
	e.source_label = DataRead.text_of(m, "source_label")
	e.created_at = DataRead.str_of(m, "created_at")
	return e


static func list_from(v: Variant, ledger_kind: String) -> Array[LedgerEntryModel]:
	var out: Array[LedgerEntryModel] = []
	if v is Array:
		for d: Variant in v as Array:
			var e: LedgerEntryModel = from_dict(d, ledger_kind)
			if e != null:
				out.append(e)
	return out


## Main text of the line: the friendly reason, with the item name for item lines.
func label() -> String:
	var key: String = "ui.inventory.reason." + reason_code
	var reason: String = TranslationServer.translate(key) if L10n.has_key(key) \
			else TranslationServer.translate("ui.inventory.reason.other")
	if reason_code == REASON_DUPLICATE and ledger == LEDGER_CURRENCY:
		return TranslationServer.translate("ui.collection.duplicate_converted").format(
				{"amount": LocaleFormat.number(absi(delta))})
	var name: String = L10n.pick(item_name)
	return reason if name.is_empty() else "%s · %s" % [reason, name]


## "+50 Xu" / "−120 Xu" for Xu lines, "+1" / "−1" for items (sign as a word-free symbol, not colour only).
func amount_text() -> String:
	var sign: String = "+" if delta >= 0 else "−"
	if ledger == LEDGER_CURRENCY:
		return TranslationServer.translate("ui.reward.currency").format(
				{"amount": sign + LocaleFormat.number(absi(delta))})
	return sign + LocaleFormat.number(absi(delta))
