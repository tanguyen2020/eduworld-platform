class_name WalletStore
extends RefCounted
## RAM store of the Xu balance (ADR-0064; F069-AC-28). Only server values replace it: `getWallet` or the
## `balance_after` of a purchase — never a local subtraction (data-model §3). The HUD CurrencyChip and S08 follow
## `wallet_changed` (no polling, F069-AC-27).

signal wallet_changed(wallet: WalletModel)

var wallet: WalletModel = null


func set_wallet(value: WalletModel) -> void:
	if value == null:
		return
	wallet = value
	wallet_changed.emit(wallet)


## `PurchaseResult.balance_after`.
func set_balance(balance: int) -> void:
	if wallet == null:
		wallet = WalletModel.new()
	wallet.balance = maxi(0, balance)
	wallet_changed.emit(wallet)


func balance() -> int:
	return wallet.balance if wallet != null else 0


func has_wallet() -> bool:
	return wallet != null


func clear() -> void:
	wallet = null
	wallet_changed.emit(null)
