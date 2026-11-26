class_name CurrencyChip
extends Button
## S03 `CurrencyChip` (F069-AC-01, STUDENT_GAME_UX_SOCIAL §3): coin icon + the Xu balance in the locale format
## ("1.250 Xu" / "1,250 Coins"); pressing it opens the Shop tab of S08. The balance only comes from WalletStore (server
## values); updated by its signal, never in `_process` (F069-AC-27). Hidden until the first `getWallet`.
## F098 (grade-band matrix): the chip keeps its label on one line (UiBuild.fit_wrapping) — in the right-aligned SideBar
## an autowrapping chip reported only its icon as minimum width and was laid out as a tall, icon-only column.
## F114: an information chip (tier 2) — thin ink outline, no bottom edge (Theme variant `HudChip`).

var store: WalletStore = null


func _init() -> void:
	name = "CurrencyChip"
	theme_type_variation = &"HudChip"
	icon = UiBuild.icon("coin")
	autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	visible = false


func _ready() -> void:
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	bind(null)
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


func bind(value: WalletStore) -> void:
	if store != null and store.wallet_changed.is_connected(render):
		store.wallet_changed.disconnect(render)
	store = value
	if store != null:
		store.wallet_changed.connect(render)
		render(store.wallet)


func render(wallet: WalletModel) -> void:
	visible = wallet != null
	if wallet == null:
		return
	text = TranslationServer.translate("ui.reward.currency").format({"amount": wallet.balance_text()})
	tooltip_text = TranslationServer.translate("ui.shop.open.tooltip")
	UiBuild.fit_wrapping(self, float(UiTokens.SIZE_LAYOUT_280))


func _on_locale_changed(_locale: String) -> void:
	render(store.wallet if store != null else null)
