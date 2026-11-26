class_name WalletModel
extends RefCounted
## Xu balance of the student (`Wallet`, INVENTORY_ECONOMY §4). Xu are earned in the game only; there is no real-money
## price anywhere in the client (FR-RWD-002, AGENTS R19). The balance only ever comes from the server: `getWallet` or
## `balance_after` of a purchase (data-model §3 — never decremented locally before a response).

var currency_code: String = ""
var balance: int = 0
var daily_earn_remaining: int = -1


static func from_dict(d: Variant) -> WalletModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	if not m.has("balance"):
		return null
	var w: WalletModel = WalletModel.new()
	w.currency_code = DataRead.str_of(m, "currency_code")
	w.balance = maxi(0, DataRead.int_of(m, "balance"))
	w.daily_earn_remaining = DataRead.nullable_int(m, "daily_earn_remaining")
	return w


## Balance formatted for the current locale ("1.250" vi / "1,250" en).
func balance_text() -> String:
	return LocaleFormat.number(balance)
