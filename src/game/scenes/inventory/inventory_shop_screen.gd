extends Control
## S08 — Tủ đồ · Cửa hàng · Lịch sử (F069 US2, STUDENT_GAME_UX_SOCIAL §3; WorldRoot screen `inventory`, args
## `{tab: wardrobe|shop|history, item_id?}`): the Xu wallet in the header, ONE AvatarPreview3D on the left (rendered once
## per change or drag, F069-AC-07) and on the right the page of the tab —
## - Tủ đồ: kind chips → `listInventory?kind=` in a VirtualItemGrid (visible cells + 1 buffer row) → detail bar
##   "Trang bị" through ProfileService `updateAvatar`; the preview takes the server's answer only, a `422
##   ITEM_NOT_OWNED` / `EQUIP_SLOT_INVALID` keeps the previous look and says why (F069-AC-08). "Phối đồ" opens the F070
##   CustomizationStudio (appearance, saved outfits, emote wheel).
## - Cửa hàng: `listShopItems` with Xu prices, limits, "Đã sở hữu", the calm end-of-sale date; "Xem trước" tries the
##   item on the preview; "Mua" ALWAYS asks first with the price and the balance after (no quick buy), greyed with
##   "Cần thêm {n} Xu" when the shown balance is short — the server decides (F069-AC-09…AC-11).
## - Lịch sử (hidden for `g1_3`): Xu and item ledgers with friendly reasons, duplicates "Đã quy đổi thành {n} Xu"
##   (F069-AC-12).
## Offline: what is loaded stays visible; Trang bị and Mua are locked (F069-AC-28). Xu only — no real money, no random
## reward, no trade or gift, no countdown (AGENTS R19). UI only: InventoryService does the work (AGENTS R1).

signal closed()

const TAB_WARDROBE: String = "wardrobe"
const TAB_SHOP: String = "shop"
const TAB_HISTORY: String = "history"
const NO_HISTORY_BANDS: PackedStringArray = ["g1_3"]
const RARITY_WORD_BANDS: PackedStringArray = ["g4_5", "g6_7", "g8_9"]
## Codes after which the purchase dialog shows the generic message (client bug, F069-AC-11).
const GENERIC_CODES: PackedStringArray = ["PRECONDITION_REQUIRED", "IDEMPOTENCY_KEY_REUSED", "VALIDATION_FAILED"]
## Kind filters (contract `ItemKind`): `g1_3` gets the first four (icons + words), older grades all of them.
const KIND_FILTERS: Array[Dictionary] = [
	{"code": "", "key": "ui.inventory.filter.all", "icon": "menu"},
	{"code": "outfit", "key": "ui.inventory.filter.outfit", "icon": "wardrobe"},
	{"code": "hair", "key": "ui.inventory.filter.hair", "icon": "emote"},
	{"code": "accessory", "key": "ui.inventory.filter.accessory", "icon": "star"},
	{"code": "emote", "key": "ui.inventory.filter.emote", "icon": "emote"},
	{"code": "pet", "key": "ui.inventory.filter.pet", "icon": "heart"},
	{"code": "title", "key": "ui.inventory.filter.title", "icon": "flag"},
	{"code": "frame", "key": "ui.inventory.filter.frame", "icon": "collection"},
	{"code": "badge", "key": "ui.inventory.filter.badge", "icon": "medal"},
]
const SMALL_FILTER_COUNT: int = 4
## Width / height of the preview stage (chibi character standing).
const PREVIEW_RATIO: float = 0.8

var tab: String = TAB_WARDROBE
var band: String = ""
var kind_filter: String = ""
var inventory: InventoryService = null
var profile: ProfileService = null
var selected_inventory: InventoryItemModel = null
var selected_shop: ShopItemModel = null
## Look on the preview before a try-on / an equip (restored on refusal).
var shown_config: AvatarConfigModel = null
var notice_key: String = ""
var notice_args: Dictionary = {}
var focus_item_id: String = ""
var equipping: bool = false
var purchasing: bool = false
var dialog_item: ShopItemModel = null
var history_ledger: String = LedgerEntryModel.LEDGER_CURRENCY
var history: Array[LedgerEntryModel] = []
var history_cursor: String = ""
var _ui: Dictionary = {}
var _wallet: Label = null
var _tabs: FilterChips = null
var _preview: AvatarPreview3D = null
var _filters: FilterChips = null
var _grid: VirtualItemGrid = null
var _detail: PanelContainer = null
var _detail_name: Label = null
var _detail_rarity: RarityMark = null
var _detail_rarity_word: Label = null
var _detail_info: Label = null
var _equip: Button = null
var _try_on: Button = null
var _buy: Button = null
var _missing: Label = null
var _notice: Label = null
var _grid_page: VBoxContainer = null
var _history_page: VBoxContainer = null
var _history_chips: FilterChips = null
var _history_rows: VBoxContainer = null
var _history_more: Button = null
var _dialog: Control = null
var _dialog_text: Label = null
var _dialog_balance: Label = null
var _dialog_note: Label = null
var _dialog_buy: Button = null
var _dialog_cancel: Button = null
var _studio: Button = null


func _ready() -> void:
	visible = false
	_build()
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	_bind(null)
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## WorldRoot: `ui_requested("inventory", {tab, item_id?})`.
func open(args: Dictionary = {}) -> void:
	var services: ServiceLocator = AppState.get_services()
	_bind(services.inventory)
	profile = services.profile
	band = UiBuild.band()
	focus_item_id = String(args.get("item_id", ""))
	notice_key = ""
	show()
	_apply_band()
	shown_config = profile.avatar_config() if profile != null else null
	_load_preview()
	inventory.load_wallet()
	select_tab(String(args.get("tab", TAB_WARDROBE)))


func close() -> void:
	_close_dialog()
	hide()
	closed.emit()


func select_tab(next: String) -> void:
	tab = next if next in [TAB_WARDROBE, TAB_SHOP, TAB_HISTORY] else TAB_WARDROBE
	if tab == TAB_HISTORY and band in NO_HISTORY_BANDS:
		tab = TAB_WARDROBE
	_tabs.select(tab, false)
	(_ui["title"] as Label).text = {TAB_WARDROBE: "ui.inventory.title", TAB_SHOP: "ui.shop.title",
		TAB_HISTORY: "ui.inventory.history.title"}[tab]
	(_ui["icon"] as TextureRect).texture = UiBuild.icon(
			{TAB_WARDROBE: "wardrobe", TAB_SHOP: "shop", TAB_HISTORY: "history"}[tab] as String)
	selected_inventory = null
	selected_shop = null
	_grid.selected_index = -1
	_restore_preview()
	_grid_page.visible = tab != TAB_HISTORY
	_history_page.visible = tab == TAB_HISTORY
	match tab:
		TAB_WARDROBE:
			_reload_inventory()
		TAB_SHOP:
			_reload_shop()
		TAB_HISTORY:
			load_history(true)
	_render()


## Test/UI entry: the grid entry `index` was chosen.
func select_entry(index: int) -> void:
	_grid.select(index)


## Opens the confirmation of the selected shop item (never buys without it, F069-AC-10).
func ask_purchase() -> void:
	if selected_shop == null or not _can_buy(selected_shop):
		return
	dialog_item = selected_shop
	var balance: int = inventory.wallet_store.balance()
	_dialog_text.text = tr("ui.shop.confirm_purchase").format({"item": selected_shop.display_name(),
		"price": LocaleFormat.number(selected_shop.price)})
	_dialog_balance.text = tr("ui.shop.balance_after").format(
			{"balance": LocaleFormat.number(maxi(0, balance - selected_shop.price))})
	_dialog_note.text = ""
	_dialog_note.visible = false
	_dialog.show()
	_dialog_cancel.grab_focus()
	_render_dialog()


## Coroutine: the student confirmed — one purchase intent (same key on retry, F069-AC-10).
func confirm_purchase() -> void:
	if dialog_item == null or purchasing:
		return
	purchasing = true
	_render_dialog()
	var item: ShopItemModel = dialog_item
	var result: ApiResult = await inventory.purchase(item)
	purchasing = false
	if result.ok:
		_close_dialog()
		_set_notice("ui.shop.bought", {"item": item.display_name()})
	elif result.is_transient():
		_dialog_note.text = tr("ui.shop.retry_safe")
		_dialog_note.visible = true
	else:
		_close_dialog()
		_set_notice_text(_friendly_error(result.code))
	_render_dialog()
	_render()


## Coroutine (F069-AC-08): wears the selected item; the preview follows the server's answer only.
func equip_selected() -> void:
	if selected_inventory == null or equipping or inventory.offline:
		return
	equipping = true
	_render()
	var before: AvatarConfigModel = shown_config
	var result: ApiResult = await inventory.equip(selected_inventory)
	equipping = false
	if result.ok and profile.avatar_config() != null:
		shown_config = profile.avatar_config()
		_preview.show_config(shown_config, inventory.catalog)
		_set_notice("ui.inventory.equipped", {"item": selected_inventory.display_name()})
	else:
		shown_config = before
		if before != null:
			_preview.show_config(before, inventory.catalog)
		_set_notice_text(L10n.error_text(result.code))
	_render()


## Tries the selected shop item on the preview (display only, nothing saved).
func try_on_selected() -> void:
	if selected_shop == null or shown_config == null:
		return
	var draft: InventoryItemModel = InventoryItemModel.new()
	draft.item_id = selected_shop.item_id
	draft.kind = selected_shop.kind
	_preview.show_config(InventoryService.equip_config(shown_config, draft), inventory.catalog)


## Coroutine: the first (or next) page of the chosen ledger.
func load_history(reset: bool) -> void:
	if reset:
		history.clear()
		history_cursor = ""
	elif history_cursor.is_empty():
		return
	_set_notice("ui.inventory.loading", {})
	var page: Dictionary = {}
	if history_ledger == LedgerEntryModel.LEDGER_CURRENCY:
		page = await inventory.load_wallet_ledger(history_cursor)
	else:
		page = await inventory.load_item_history(history_cursor)
	if not bool(page["ok"]):
		_set_notice_text(L10n.error_text(String(page["code"])))
		return
	history.append_array(page["entries"] as Array)
	history_cursor = String(page["next_cursor"])
	_set_notice("ui.inventory.history.empty" if history.is_empty() else "", {})
	_render_history()


func dialog_open() -> bool:
	return _dialog.visible


func dialog_texts() -> PackedStringArray:
	return PackedStringArray([_dialog_text.text, _dialog_balance.text, _dialog_note.text])


func grid() -> VirtualItemGrid:
	return _grid


func preview() -> AvatarPreview3D:
	return _preview


func notice_text() -> String:
	return _notice.text


func tab_visible(code: String) -> bool:
	var chip: Button = _tabs.chip(code)
	return chip != null and chip.visible


func buy_button() -> Button:
	return _buy


func equip_button() -> Button:
	return _equip


func missing_text() -> String:
	return _missing.text if _missing.visible else ""


func rarity_word() -> String:
	return _detail_rarity_word.text if _detail_rarity_word.visible else ""


func history_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for l: Node in _history_rows.find_children("*", "Label", true, false):
		out.append((l as Label).text)
	return out


func _unhandled_input(event: InputEvent) -> void:
	if not visible:
		return
	if event.is_action_pressed("ui_cancel"):
		if _dialog.visible:
			_close_dialog()
		else:
			close()
		get_viewport().set_input_as_handled()


# ---------- data → view ----------

func _bind(service: InventoryService) -> void:
	if inventory != null:
		for pair: Array in [[inventory.wallet_store.wallet_changed, _on_wallet_changed],
				[inventory.inventory_changed, _on_inventory_changed], [inventory.shop_changed, _on_shop_changed],
				[inventory.offline_changed, _on_offline_changed]]:
			if (pair[0] as Signal).is_connected(pair[1] as Callable):
				(pair[0] as Signal).disconnect(pair[1] as Callable)
	inventory = service
	if inventory != null:
		inventory.wallet_store.wallet_changed.connect(_on_wallet_changed)
		inventory.inventory_changed.connect(_on_inventory_changed)
		inventory.shop_changed.connect(_on_shop_changed)
		inventory.offline_changed.connect(_on_offline_changed)


func _reload_inventory() -> void:
	_set_notice("ui.inventory.loading", {})
	_fill_grid()
	var result: ApiResult = await inventory.load_inventory(kind_filter)
	if not result.ok:
		_set_notice_text(L10n.error_text(result.code))
	elif inventory.items.is_empty():
		_set_notice("ui.inventory.empty_hint", {})
	else:
		_set_notice("", {})


func _reload_shop() -> void:
	_set_notice("ui.inventory.loading", {})
	_fill_grid()
	var result: ApiResult = await inventory.load_shop(kind_filter)
	if not result.ok:
		_set_notice_text(L10n.error_text(result.code))
	elif inventory.shop.is_empty():
		_set_notice("ui.shop.empty", {})
	else:
		_set_notice("", {})


func _fill_grid() -> void:
	var list: Array[Dictionary] = []
	var focus: int = -1
	if tab == TAB_WARDROBE:
		for item: InventoryItemModel in inventory.items:
			if not kind_filter.is_empty() and item.kind != kind_filter:
				continue
			if item.item_id == focus_item_id:
				focus = list.size()
			list.append({"item_id": item.item_id, "kind": item.kind, "rarity": item.rarity, "name": item.display_name(),
				"sub": tr("ui.inventory.quantity").format({"n": LocaleFormat.number(item.quantity)}) if item.quantity > 1 else "",
				"equipped": inventory.is_equipped(item.item_id)})
	elif tab == TAB_SHOP:
		for s: ShopItemModel in inventory.shop:
			list.append({"item_id": s.item_id, "kind": s.kind, "rarity": s.rarity, "name": s.display_name(),
				"sub": tr("ui.shop.owned") if s.is_sold_out_for_me() else tr("ui.reward.currency").format(
						{"amount": LocaleFormat.number(s.price)})})
	_grid.set_entries(list, true)
	if focus >= 0:
		focus_item_id = ""
		_grid.select(focus)


func _on_entry_selected(index: int) -> void:
	if tab == TAB_WARDROBE:
		var visible_items: Array[InventoryItemModel] = []
		for item: InventoryItemModel in inventory.items:
			if kind_filter.is_empty() or item.kind == kind_filter:
				visible_items.append(item)
		selected_inventory = visible_items[index] if index < visible_items.size() else null
	elif tab == TAB_SHOP:
		selected_shop = inventory.shop[index] if index < inventory.shop.size() else null
		_restore_preview()
	_render()


func _on_filter_changed(code: String) -> void:
	kind_filter = code
	selected_inventory = null
	selected_shop = null
	_grid.selected_index = -1
	if tab == TAB_WARDROBE:
		_reload_inventory()
	elif tab == TAB_SHOP:
		_reload_shop()
	_render()


func _on_wallet_changed(_wallet_model: WalletModel) -> void:
	_render()
	EventBus.wallet_changed.emit(inventory.wallet_store.balance())


func _on_inventory_changed(_count: int) -> void:
	if visible and tab == TAB_WARDROBE:
		_fill_grid()
		_render()


func _on_shop_changed(_count: int) -> void:
	if visible and tab == TAB_SHOP:
		_fill_grid()
		if selected_shop != null:
			selected_shop = inventory.shop_item(selected_shop.shop_item_id)
		_render()


func _on_offline_changed(_offline: bool) -> void:
	_render()


func _can_buy(item: ShopItemModel) -> bool:
	return item != null and not item.is_sold_out_for_me() and not inventory.offline and not purchasing \
			and item.missing_for(inventory.wallet_store.balance()) == 0 and inventory.wallet_store.has_wallet()


func _friendly_error(code: String) -> String:
	return tr(L10n.ERROR_GENERIC) if code in GENERIC_CODES else L10n.error_text(code)


func _restore_preview() -> void:
	if shown_config != null and _preview != null:
		_preview.show_config(shown_config, inventory.catalog if inventory != null else null)


func _set_notice(key: String, args: Dictionary) -> void:
	notice_key = key
	notice_args = args
	_notice.text = tr(key).format(args) if not key.is_empty() else ""
	_notice.visible = not _notice.text.is_empty()


func _set_notice_text(text: String) -> void:
	notice_key = ""
	_notice.text = text
	_notice.visible = not text.is_empty()


func _render() -> void:
	if inventory == null:
		return
	var w: WalletModel = inventory.wallet_store.wallet
	_wallet.text = tr("ui.reward.currency").format({"amount": w.balance_text()}) if w != null else ""
	_wallet.get_parent().visible = w != null
	(_filters.get_parent() as Control).visible = tab != TAB_HISTORY
	var offline: bool = inventory.offline
	var has_item: bool = (tab == TAB_WARDROBE and selected_inventory != null) or (tab == TAB_SHOP and selected_shop != null)
	_detail.visible = has_item
	_equip.visible = tab == TAB_WARDROBE
	_try_on.visible = tab == TAB_SHOP
	_buy.visible = tab == TAB_SHOP
	_missing.visible = false
	if tab == TAB_WARDROBE and selected_inventory != null:
		_detail_name.text = selected_inventory.display_name()
		_set_rarity(selected_inventory.rarity)
		_detail_info.text = ""
		_equip.disabled = offline or equipping
		_equip.text = "ui.inventory.equipping" if equipping else "ui.inventory.equip"
	elif tab == TAB_SHOP and selected_shop != null:
		var s: ShopItemModel = selected_shop
		_detail_name.text = s.display_name()
		_set_rarity(s.rarity)
		var lines: PackedStringArray = []
		var desc: String = L10n.pick(s.description)
		# F125: on the compact phone canvas (540 tall) the item card keeps name, price and buttons (the flavour line gives
		# its room to the item grid).
		if not desc.is_empty() and not AppState.compact:
			lines.append(desc)
		lines.append(tr("ui.shop.price").format({"price": LocaleFormat.number(s.price)}))
		if s.purchase_limit > 1:
			lines.append(tr("ui.shop.limit").format({"count": LocaleFormat.number(s.purchased_count),
				"limit": LocaleFormat.number(s.purchase_limit)}))
		var until: int = ExamCountdown.iso_to_ms(s.available_to)
		if until > 0:
			lines.append(tr("ui.shop.available_to").format({"date": LocaleFormat.date(LocaleFormat.from_unix(until / 1000))}))
		if s.is_sold_out_for_me():
			lines.append(tr("ui.shop.owned"))
		_detail_info.text = "\n".join(lines)
		var missing: int = s.missing_for(inventory.wallet_store.balance())
		_buy.disabled = not _can_buy(s)
		_buy.visible = not s.is_sold_out_for_me()
		if missing > 0 and not s.is_sold_out_for_me():
			_missing.text = tr("ui.shop.need_more").format({"n": LocaleFormat.number(missing)})
			_missing.visible = true
		elif offline and not s.is_sold_out_for_me():
			_missing.text = tr("ui.inventory.offline_locked")
			_missing.visible = true
	_detail_info.visible = not _detail_info.text.is_empty()
	if offline and notice_key.is_empty() and _notice.text.is_empty():
		_set_notice("ui.inventory.offline_locked", {})


func _set_rarity(rarity: String) -> void:
	var word: bool = band in RARITY_WORD_BANDS
	_detail_rarity.set_rarity(rarity, false)
	_detail_rarity_word.text = RarityMark.key_of(rarity)
	_detail_rarity_word.visible = word


func _render_history() -> void:
	UiBuild.clear(_history_rows)
	for e: LedgerEntryModel in history:
		var card: PanelContainer = UiBuild.panel(&"QuestCard")
		var h: HBoxContainer = UiBuild.hbox()
		card.add_child(h)
		h.add_child(UiBuild.icon_rect("coin" if e.ledger == LedgerEntryModel.LEDGER_CURRENCY else "gift"))
		var label: Label = UiBuild.data_label(e.label())
		UiBuild.expand(label)
		h.add_child(label)
		h.add_child(UiBuild.data_label(e.amount_text(), &"HeadingLabel", false))
		var when: int = ExamCountdown.iso_to_ms(e.created_at)
		if when > 0:
			h.add_child(UiBuild.data_label(LocaleFormat.date(LocaleFormat.from_unix(when / 1000)), &"MutedLabel", false))
		_history_rows.add_child(card)
	_history_more.visible = not history_cursor.is_empty()


func _render_dialog() -> void:
	_dialog_buy.disabled = purchasing or inventory == null or inventory.offline and not _dialog_note.visible
	_dialog_buy.text = "ui.shop.buying" if purchasing else "ui.shop.buy"
	_dialog_cancel.disabled = purchasing


func _close_dialog() -> void:
	if _dialog != null:
		_dialog.hide()
	dialog_item = null


func _apply_band() -> void:
	var options: Array[Dictionary] = []
	for i: int in KIND_FILTERS.size():
		if band != "g1_3" or i < SMALL_FILTER_COUNT:
			options.append(KIND_FILTERS[i])
	_filters.set_options(options, kind_filter)
	var tabs: Array[Dictionary] = [{"code": TAB_WARDROBE, "key": "ui.inventory.tab.wardrobe", "icon": "wardrobe"},
		{"code": TAB_SHOP, "key": "ui.shop.tab", "icon": "shop"}]
	if not band in NO_HISTORY_BANDS:
		tabs.append({"code": TAB_HISTORY, "key": "ui.inventory.tab.history", "icon": "history"})
	_tabs.set_options(tabs, tab)
	_history_chips.set_options([{"code": LedgerEntryModel.LEDGER_CURRENCY, "key": "ui.inventory.history.currency", "icon": "coin"},
		{"code": LedgerEntryModel.LEDGER_ITEM, "key": "ui.inventory.history.items", "icon": "gift"}], history_ledger)
	UiBuild.size_buttons(self)
	_grid.relayout()


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_fill_grid()
		_render()
		_render_history()


# ---------- building ----------

func _build() -> void:
	_ui = UiBuild.window(self, "ui.inventory.title", "wardrobe")
	(_ui["close"] as Button).pressed.connect(close)
	var header: HBoxContainer = _ui["header"] as HBoxContainer
	var wallet_box: PanelContainer = UiBuild.panel(&"QuestCard")
	wallet_box.name = "Wallet"
	var wallet_row: HBoxContainer = UiBuild.hbox()
	wallet_box.add_child(wallet_row)
	wallet_row.add_child(UiBuild.icon_rect("coin"))
	_wallet = UiBuild.data_label("", &"HeadingLabel", false)
	_wallet.name = "WalletLabel"
	wallet_row.add_child(_wallet)
	header.add_child(wallet_box)
	header.move_child(wallet_box, header.get_child_count() - 2)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	_tabs = FilterChips.new()
	_tabs.name = "Tabs"
	_tabs.filter_changed.connect(select_tab)
	if AppState.compact:
		# F125: on the compact phone canvas (540 tall) the Tủ đồ / Cửa hàng / Lịch sử chips sit on the title bar, before the
		# Xu wallet, so the item grid keeps a full row above the item card.
		_tabs.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		header.add_child(_tabs)
		header.move_child(_tabs, wallet_box.get_index())
	else:
		body.add_child(_tabs)
	var columns: HBoxContainer = UiBuild.hbox()
	UiBuild.expand(columns, true, true)
	body.add_child(columns)
	var left: VBoxContainer = UiBuild.vbox()
	left.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	left.size_flags_stretch_ratio = 0.55
	columns.add_child(left)
	_preview = (load("res://scenes/character/AvatarPreview3D.tscn") as PackedScene).instantiate() as AvatarPreview3D
	_preview.name = "Preview"
	# A portrait-shaped stage whatever the window shape (a tall phone window would otherwise zoom on the face).
	var stage: AspectRatioContainer = AspectRatioContainer.new()
	stage.name = "Stage"
	stage.ratio = PREVIEW_RATIO
	stage.stretch_mode = AspectRatioContainer.STRETCH_FIT
	UiBuild.expand(stage, true, true)
	left.add_child(stage)
	stage.add_child(_preview)
	var tools: HBoxContainer = UiBuild.hbox()
	tools.alignment = BoxContainer.ALIGNMENT_CENTER
	left.add_child(tools)
	var turn_left: Button = UiBuild.button("", "rotate_left", &"SecondaryButton")
	turn_left.tooltip_text = "ui.avatar.rotate_left"
	turn_left.pressed.connect(func() -> void: _preview.turn(-AvatarPreview3D.TURN_STEP))
	tools.add_child(turn_left)
	var turn_right: Button = UiBuild.button("", "rotate_right", &"SecondaryButton")
	turn_right.tooltip_text = "ui.avatar.rotate_right"
	turn_right.pressed.connect(func() -> void: _preview.turn(AvatarPreview3D.TURN_STEP))
	tools.add_child(turn_right)
	_studio = UiBuild.button("ui.inventory.customize", "wardrobe", &"SecondaryButton")
	_studio.name = "Customize"
	_studio.tooltip_text = "ui.inventory.customize.tooltip"
	_studio.pressed.connect(func() -> void: EventBus.ui_requested.emit(&"wardrobe", {}))
	tools.add_child(_studio)
	var right: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(right, true, true)
	columns.add_child(right)
	_grid_page = UiBuild.vbox()
	_grid_page.name = "GridPage"
	UiBuild.expand(_grid_page, true, true)
	right.add_child(_grid_page)
	_filters = FilterChips.new()
	_filters.name = "Filters"
	_filters.filter_changed.connect(_on_filter_changed)
	_grid_page.add_child(UiBuild.chip_scroller(_filters))
	_grid = VirtualItemGrid.new()
	_grid.name = "Grid"
	_grid.entry_selected.connect(_on_entry_selected)
	_grid_page.add_child(_grid)
	_build_detail(_grid_page)
	_build_history(right)
	_notice = UiBuild.data_label("", &"InfoBannerLabel")
	_notice.name = "Notice"
	_notice.visible = false
	body.add_child(_notice)
	_build_dialog()


func _build_detail(parent: Control) -> void:
	_detail = UiBuild.panel(&"QuestCard")
	_detail.name = "Detail"
	parent.add_child(_detail)
	var h: HBoxContainer = UiBuild.hbox()
	_detail.add_child(h)
	var text_box: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(text_box)
	h.add_child(text_box)
	var name_row: HBoxContainer = UiBuild.hbox()
	text_box.add_child(name_row)
	_detail_rarity = RarityMark.new()
	_detail_rarity.name = "Rarity"
	name_row.add_child(_detail_rarity)
	_detail_name = UiBuild.data_label("", &"HeadingLabel", false)
	_detail_name.name = "ItemName"
	_detail_name.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	UiBuild.expand(_detail_name)
	name_row.add_child(_detail_name)
	_detail_rarity_word = UiBuild.label("", &"MutedLabel", false)
	_detail_rarity_word.name = "RarityWord"
	name_row.add_child(_detail_rarity_word)
	_detail_info = UiBuild.data_label("", &"MutedLabel")
	_detail_info.name = "Info"
	text_box.add_child(_detail_info)
	_missing = UiBuild.data_label("", &"InlineErrorLabel")
	_missing.name = "Missing"
	text_box.add_child(_missing)
	var actions: HBoxContainer = UiBuild.hbox()
	actions.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	h.add_child(actions)
	_equip = UiBuild.button("ui.inventory.equip", "check")
	_equip.name = "Equip"
	_equip.pressed.connect(equip_selected)
	actions.add_child(_equip)
	_try_on = UiBuild.button("ui.shop.try_on", "eye", &"SecondaryButton")
	_try_on.name = "TryOn"
	_try_on.pressed.connect(try_on_selected)
	actions.add_child(_try_on)
	_buy = UiBuild.button("ui.shop.buy", "coin")
	_buy.name = "Buy"
	_buy.pressed.connect(ask_purchase)
	actions.add_child(_buy)


func _build_history(parent: Control) -> void:
	_history_page = UiBuild.vbox()
	_history_page.name = "HistoryPage"
	UiBuild.expand(_history_page, true, true)
	_history_page.visible = false
	parent.add_child(_history_page)
	_history_chips = FilterChips.new()
	_history_chips.name = "HistoryChips"
	_history_chips.filter_changed.connect(func(code: String) -> void:
		history_ledger = code
		load_history(true))
	_history_page.add_child(_history_chips)
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	_history_page.add_child(scroll)
	var list: VBoxContainer = UiBuild.vbox()
	UiBuild.expand(list)
	scroll.add_child(list)
	_history_rows = UiBuild.vbox()
	_history_rows.name = "HistoryRows"
	list.add_child(_history_rows)
	_history_more = UiBuild.button("ui.journey.more", "arrow_down", &"SecondaryButton")
	_history_more.name = "HistoryMore"
	_history_more.pressed.connect(func() -> void: load_history(false))
	list.add_child(_history_more)


func _build_dialog() -> void:
	_dialog = Control.new()
	_dialog.name = "PurchaseConfirmDialog"
	_dialog.set_anchors_preset(Control.PRESET_FULL_RECT)
	_dialog.mouse_filter = Control.MOUSE_FILTER_STOP
	add_child(_dialog)
	_dialog.add_child(UiBuild.scrim(true)) # F114-AC-07: a dialog over the window adds the nested scrim
	var center: CenterContainer = CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	_dialog.add_child(center)
	var card: PanelContainer = UiBuild.panel(&"FormCard")
	center.add_child(card)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	box.custom_minimum_size.x = float(UiTokens.SIZE_LAYOUT_280) * 1.6
	card.add_child(box)
	var coin: TextureRect = UiBuild.icon_rect("coin", 2.5)
	box.add_child(coin)
	_dialog_text = UiBuild.data_label("", &"HeadingLabel")
	_dialog_text.name = "Question"
	_dialog_text.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	box.add_child(_dialog_text)
	_dialog_balance = UiBuild.data_label("", &"")
	_dialog_balance.name = "BalanceAfter"
	_dialog_balance.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	box.add_child(_dialog_balance)
	_dialog_note = UiBuild.data_label("", &"InfoBannerLabel")
	_dialog_note.name = "Note"
	box.add_child(_dialog_note)
	var actions: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	actions.alignment = BoxContainer.ALIGNMENT_CENTER
	box.add_child(actions)
	_dialog_cancel = UiBuild.button("ui.common.cancel", "arrow_left", &"SecondaryButton")
	_dialog_cancel.name = "Cancel"
	_dialog_cancel.pressed.connect(_close_dialog)
	actions.add_child(_dialog_cancel)
	_dialog_buy = UiBuild.button("ui.shop.buy", "coin")
	_dialog_buy.name = "Confirm"
	_dialog_buy.pressed.connect(confirm_purchase)
	actions.add_child(_dialog_buy)
	_dialog.hide()


## Coroutine: the catalog (display data of the looks) once per run, then the student's look on the preview.
func _load_preview() -> void:
	if not inventory.has_catalog():
		await inventory.load_catalog()
	_restore_preview()
