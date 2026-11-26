extends Control
## S09 — Thành tựu · Bộ sưu tập · Huy hiệu trưng bày (F069 US4, STUDENT_GAME_UX_SOCIAL §4; WorldRoot screen
## `achievements`, args `{tab}`):
## - Thành tựu: category chips, rows with the tier and a progress bar "7/10"; a hidden achievement not unlocked shows
##   ONLY "?" and the vague hint, whatever the response carried (F069-AC-21); an unlocked one says "Đã nhận" with the
##   rewards the server granted — there is no claim button (F069-AC-22).
## - Bộ sưu tập: the collections and their progress; a collection page draws owned items in colour and missing ones as
##   the same picture greyed with "Chưa có" and "Cách nhận" (F069-AC-23); `g1_3` gets a sticker book of big pictures.
## - Huy hiệu trưng bày: only owned badges, as many slots as the server allows, saved with `updateAvatar`
##   (F069-AC-24).
## UI only: AchievementService does the work (AGENTS R1).

signal closed()

const TAB_ACHIEVEMENTS: String = "achievements"
const TAB_COLLECTIONS: String = "collections"
const TAB_SHOWCASE: String = "showcase"
const TIER_BANDS: PackedStringArray = ["g4_5", "g6_7", "g8_9"]
const STICKER_BANDS: PackedStringArray = ["g1_3"]
## Picture of a collection card by collection type (contract `CollectionType`).
const COLLECTION_KINDS: Dictionary = {"pet": "pet", "knowledge_card": "knowledge_card", "cosmetic": "outfit",
	"badge": "badge", "exploration": "decoration", "event": "decoration"}

var tab: String = TAB_ACHIEVEMENTS
var band: String = ""
var category: String = ""
var service: AchievementService = null
var collection: CollectionModel = null
var showcase_draft: PackedStringArray = []
var _ui: Dictionary = {}
var _tabs: FilterChips = null
var _categories: FilterChips = null
var _list: VBoxContainer = null
var _state: Label = null
var _retry: Button = null
var _scroll: ScrollContainer = null
var _back: Button = null
var _save: Button = null


func _ready() -> void:
	visible = false
	_ui = UiBuild.window(self, "ui.achievement.title", "medal")
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	_tabs = FilterChips.new()
	_tabs.name = "Tabs"
	_tabs.filter_changed.connect(select_tab)
	body.add_child(_tabs)
	_categories = FilterChips.new()
	_categories.name = "Categories"
	_categories.filter_changed.connect(func(code: String) -> void:
		category = code
		_render())
	body.add_child(UiBuild.chip_scroller(_categories))
	var state_row: HBoxContainer = UiBuild.hbox()
	body.add_child(state_row)
	_back = UiBuild.button("ui.collection.back", "arrow_left", &"SecondaryButton")
	_back.name = "Back"
	_back.pressed.connect(func() -> void:
		collection = null
		_render())
	state_row.add_child(_back)
	_state = UiBuild.label("", &"MutedLabel")
	_state.name = "StateLabel"
	UiBuild.expand(_state)
	state_row.add_child(_state)
	_retry = UiBuild.button("ui.common.retry", "rotate_right", &"SecondaryButton")
	_retry.name = "Retry"
	_retry.pressed.connect(func() -> void: select_tab(tab))
	state_row.add_child(_retry)
	_save = UiBuild.button("ui.achievement.showcase_save", "check")
	_save.name = "SaveShowcase"
	_save.pressed.connect(save_showcase)
	state_row.add_child(_save)
	_scroll = ScrollContainer.new()
	_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(_scroll, true, true)
	body.add_child(_scroll)
	_list = UiBuild.vbox()
	_list.name = "List"
	UiBuild.expand(_list)
	_scroll.add_child(_list)
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## WorldRoot: `ui_requested("achievements", {tab?})`.
func open(args: Dictionary = {}) -> void:
	if service == null:
		service = AppState.get_services().achievements
	band = UiBuild.band()
	collection = null
	show()
	var tabs: Array[Dictionary] = [{"code": TAB_ACHIEVEMENTS, "key": "ui.achievement.tab.achievements", "icon": "medal"},
		{"code": TAB_COLLECTIONS, "key": "ui.collection.tab", "icon": "collection"}]
	# STUDENT_GAME_UX_SOCIAL §4: g1_3 has the sticker book and the short achievement list only.
	if not band in STICKER_BANDS:
		tabs.append({"code": TAB_SHOWCASE, "key": "ui.achievement.tab.showcase", "icon": "star"})
	_tabs.set_options(tabs, tab)
	UiBuild.size_buttons(self)
	select_tab(String(args.get("tab", TAB_ACHIEVEMENTS)))


func close() -> void:
	hide()
	closed.emit()


## Coroutine: loads and shows the tab.
func select_tab(next: String) -> void:
	tab = next if next in [TAB_ACHIEVEMENTS, TAB_COLLECTIONS, TAB_SHOWCASE] else TAB_ACHIEVEMENTS
	if tab == TAB_SHOWCASE and band in STICKER_BANDS:
		tab = TAB_COLLECTIONS
	_tabs.select(tab, false)
	collection = null
	_set_state("ui.achievement.loading", false)
	_render()
	var result: ApiResult = null
	match tab:
		TAB_ACHIEVEMENTS:
			result = await service.load_achievements()
			var options: Array[Dictionary] = [{"code": "", "key": "ui.inventory.filter.all"}]
			for c: String in service.categories():
				options.append({"code": c, "key": "ui.achievement.category." + c})
			category = ""
			_categories.set_options(options, "")
		TAB_COLLECTIONS:
			result = await service.load_collections()
		TAB_SHOWCASE:
			result = await service.load_badges()
			showcase_draft = service.valid_showcase(service.showcase())
	if result != null and not result.ok:
		_set_state("ui.achievement.load_failed", true)
	else:
		_set_state("", false)
	UiBuild.size_buttons(self)
	_render()


## Coroutine: opens the page of one collection (404 → the list is refreshed with a notice).
func open_collection(collection_id: String) -> void:
	_set_state("ui.achievement.loading", false)
	var detail: CollectionModel = await service.load_collection(collection_id)
	if detail == null:
		_set_state("ui.collection.gone" if service.last_error_code == "NOT_FOUND" else "ui.achievement.load_failed",
				service.last_error_code != "NOT_FOUND")
		collection = null
	else:
		_set_state("", false)
		collection = detail
	_render()


## Adds `badge` to the first free slot, or removes it when already on the showcase (only owned badges).
func toggle_badge(badge: String) -> void:
	if badge in showcase_draft:
		showcase_draft.remove_at(showcase_draft.find(badge))
	elif showcase_draft.size() < service.badge_slots():
		showcase_draft.append(badge)
	showcase_draft = service.valid_showcase(showcase_draft)
	_render()


## Coroutine: saves the showcase through ProfileService `updateAvatar`.
func save_showcase() -> void:
	_save.disabled = true
	var result: ApiResult = await service.save_showcase(showcase_draft)
	_save.disabled = false
	_set_state("ui.achievement.showcase_saved" if result.ok else "", false)
	if not result.ok:
		_state.text = L10n.error_text(result.code)
		_state.visible = true


func row_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in _list.find_children("*", "Label", true, false):
		out.append((node as Label).text)
	return out


func list_root() -> VBoxContainer:
	return _list


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		close()
		get_viewport().set_input_as_handled()


func _set_state(key: String, error: bool) -> void:
	_state.text = key
	_state.visible = not key.is_empty()
	_retry.visible = error


func _render() -> void:
	UiBuild.clear(_list)
	(_categories.get_parent() as Control).visible = tab == TAB_ACHIEVEMENTS and band in TIER_BANDS
	_back.visible = tab == TAB_COLLECTIONS and collection != null
	_save.visible = tab == TAB_SHOWCASE
	if service == null:
		return
	match tab:
		TAB_ACHIEVEMENTS:
			for a: AchievementModel in service.achievements:
				if category.is_empty() or a.category == category:
					_list.add_child(_achievement_row(a))
			if service.achievements.is_empty() and not _state.visible:
				_set_state("ui.achievement.empty", false)
		TAB_COLLECTIONS:
			if collection != null:
				_render_collection(collection)
			else:
				for c: CollectionModel in service.collections:
					_list.add_child(_collection_row(c))
				if service.collections.is_empty() and not _state.visible:
					_set_state("ui.collection.empty", false)
		TAB_SHOWCASE:
			_render_showcase()


func _achievement_row(a: AchievementModel) -> Control:
	var card: PanelContainer = UiBuild.panel(&"QuestCard")
	card.name = "Achievement_%s" % a.achievement_id.left(8)
	var h: HBoxContainer = UiBuild.hbox()
	card.add_child(h)
	h.add_child(UiBuild.icon_rect("help" if a.hidden else ("medal" if a.unlocked else "star"), 1.4))
	var text_box: VBoxContainer = UiBuild.vbox(&"FieldBox")
	UiBuild.expand(text_box)
	h.add_child(text_box)
	var title: Label = UiBuild.data_label(a.display_title(), &"HeadingLabel")
	title.name = "Title"
	text_box.add_child(title)
	if a.hidden:
		text_box.add_child(UiBuild.label("ui.achievement.hidden_hint", &"MutedLabel"))
		return card
	var desc: String = L10n.pick(a.description)
	if not desc.is_empty() and band != "g1_3":
		text_box.add_child(UiBuild.data_label(desc, &"MutedLabel"))
	var progress_row: HBoxContainer = UiBuild.hbox()
	text_box.add_child(progress_row)
	var bar: ProgressBar = UiBuild.progress_bar(a.fraction())
	bar.name = "Progress"
	UiBuild.expand(bar)
	progress_row.add_child(bar)
	var progress: String = a.progress_text()
	if not progress.is_empty():
		var p: Label = UiBuild.data_label(progress, &"", false)
		p.tooltip_text = tr("ui.achievement.progress_spoken").format({"current": progress.get_slice("/", 0),
			"target": progress.get_slice("/", 1)})
		progress_row.add_child(p)
	var side: VBoxContainer = UiBuild.vbox(&"FieldBox")
	h.add_child(side)
	var tier_key: String = a.tier_key()
	if not tier_key.is_empty() and band in TIER_BANDS:
		side.add_child(UiBuild.label(tier_key, &"MutedLabel", false))
	if a.unlocked:
		var got: HBoxContainer = UiBuild.hbox()
		got.add_child(UiBuild.icon_rect("verdict_correct"))
		got.add_child(UiBuild.label("ui.achievement.received", &"", false))
		side.add_child(got)
	var rewards: RewardRow = RewardRow.new()
	side.add_child(rewards)
	rewards.set_rewards(a.rewards, band == "g1_3")
	return card


## A collection card: a picture of its kind, the title and type, the progress bar "3/10" and an arrow — the whole
## card is one big touch target.
func _collection_row(c: CollectionModel) -> Control:
	var b: Button = UiBuild.button("", "", &"SecondaryButton")
	b.name = "Collection_%s" % c.collection_id.left(8)
	b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	b.tooltip_text = "%s · %s" % [L10n.pick(c.title), tr("ui.collection.progress").format({"progress": c.progress_text()})]
	var t: float = UiBuild.target()
	b.custom_minimum_size = Vector2(t, t * 1.9)
	b.pressed.connect(open_collection.bind(c.collection_id))
	var row: HBoxContainer = UiBuild.hbox()
	row.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	row.offset_left = float(UiTokens.SIZE_SPACE_12)
	row.offset_right = -float(UiTokens.SIZE_SPACE_12)
	row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	b.add_child(row)
	var thumb: ItemThumb = ItemThumb.new()
	thumb.custom_minimum_size = Vector2(t * 1.5, t * 1.5)
	thumb.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	thumb.set_item(COLLECTION_KINDS.get(c.type, "decoration") as String, "", "rare")
	row.add_child(thumb)
	var text: VBoxContainer = UiBuild.vbox(&"FieldBox")
	text.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	text.mouse_filter = Control.MOUSE_FILTER_IGNORE
	UiBuild.expand(text)
	row.add_child(text)
	var title: Label = UiBuild.data_label(L10n.pick(c.title), &"HeadingLabel", false)
	title.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	text.add_child(title)
	var line: HBoxContainer = UiBuild.hbox()
	text.add_child(line)
	var bar: ProgressBar = UiBuild.progress_bar(c.fraction())
	bar.mouse_filter = Control.MOUSE_FILTER_IGNORE
	UiBuild.expand(bar)
	line.add_child(bar)
	line.add_child(UiBuild.data_label(tr("ui.collection.progress").format({"progress": c.progress_text()}), &"", false))
	var arrow: TextureRect = UiBuild.icon_rect("arrow_right")
	arrow.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(arrow)
	for node: Node in row.find_children("*", "Control", true, false):
		(node as Control).mouse_filter = Control.MOUSE_FILTER_IGNORE
	return b


func _render_collection(c: CollectionModel) -> void:
	var head: HBoxContainer = UiBuild.hbox()
	_list.add_child(head)
	var title: Label = UiBuild.data_label(L10n.pick(c.title), &"HeadingLabel")
	UiBuild.expand(title)
	head.add_child(title)
	head.add_child(UiBuild.data_label(c.progress_text(), &"HeadingLabel", false))
	var bar: ProgressBar = UiBuild.progress_bar(c.fraction())
	_list.add_child(bar)
	if not c.completion_rewards.is_empty():
		_list.add_child(UiBuild.label("ui.collection.completion_reward", &"MutedLabel"))
		var rewards: RewardRow = RewardRow.new()
		_list.add_child(rewards)
		rewards.set_rewards(c.completion_rewards, band == "g1_3")
	var sticker: bool = band in STICKER_BANDS
	var grid: GridContainer = GridContainer.new()
	grid.name = "Items"
	var cell_w: float = UiBuild.target() * (3.2 if sticker else 3.6)
	grid.columns = maxi(1, int((_scroll.size.x if _scroll.size.x > 0.0 else size.x * 0.8) / (cell_w + 12.0)))
	_list.add_child(grid)
	for it: Dictionary in c.items:
		grid.add_child(_collection_item(it, sticker, cell_w))


func _collection_item(it: Dictionary, sticker: bool, cell_w: float) -> Control:
	var owned: bool = bool(it["owned"])
	var card: PanelContainer = UiBuild.panel(&"QuestCard")
	card.name = "Item_%s" % String(it["item_id"])
	card.custom_minimum_size = Vector2(cell_w, 0.0)
	var box: VBoxContainer = UiBuild.vbox(&"FieldBox")
	card.add_child(box)
	var thumb: ItemThumb = ItemThumb.new()
	thumb.name = "Thumb"
	var side: float = UiBuild.target() * (2.6 if sticker else 1.8)
	thumb.custom_minimum_size = Vector2(side, side)
	thumb.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
	thumb.set_item(String(it["kind"]) if not String(it["kind"]).is_empty() else "pet", String(it["item_id"]),
			String(it["rarity"]), not owned)
	box.add_child(thumb)
	var name_text: String = L10n.pick(it["name"] as Dictionary)
	if not name_text.is_empty():
		var n: Label = UiBuild.data_label(name_text, &"", true)
		n.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		box.add_child(n)
	var status: HBoxContainer = UiBuild.hbox()
	status.alignment = BoxContainer.ALIGNMENT_CENTER
	status.add_child(UiBuild.icon_rect("verdict_correct" if owned else "lock", 0.75))
	var status_label: Label = UiBuild.label("ui.collection.owned" if owned else "ui.collection.missing", &"MutedLabel", false)
	status_label.name = "Status"
	status.add_child(status_label)
	box.add_child(status)
	var how: String = L10n.pick(it["how_to_obtain"] as Dictionary)
	if not owned and not how.is_empty():
		var how_label: Label = UiBuild.data_label(tr("ui.collection.how_to_obtain_line").format({"how": how}), &"MutedLabel")
		how_label.name = "HowToObtain"
		box.add_child(how_label)
	return card


func _render_showcase() -> void:
	_list.add_child(UiBuild.label("ui.achievement.showcase_hint", &"MutedLabel"))
	var slots: HBoxContainer = UiBuild.hbox()
	slots.name = "Slots"
	_list.add_child(slots)
	for i: int in service.badge_slots():
		var code: String = showcase_draft[i] if i < showcase_draft.size() else ""
		var b: Button = UiBuild.button("", "" if not code.is_empty() else "star", &"PictureTile" if not code.is_empty()
				else &"SecondaryButton")
		b.name = "Slot%d" % (i + 1)
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		var t: float = UiBuild.target()
		b.custom_minimum_size = Vector2(t * 2.4, t * 2.4)
		if code.is_empty():
			b.text = tr("ui.achievement.showcase_empty")
		else:
			var thumb: ItemThumb = ItemThumb.new()
			thumb.set_anchors_preset(Control.PRESET_FULL_RECT)
			thumb.set_item("badge", code, _badge_rarity(code))
			b.add_child(thumb)
			b.tooltip_text = _badge_name(code)
			b.pressed.connect(toggle_badge.bind(code))
		slots.add_child(b)
	_list.add_child(UiBuild.label("ui.achievement.showcase_owned", &"HeadingLabel"))
	var owned: HFlowContainer = HFlowContainer.new()
	owned.name = "OwnedBadges"
	_list.add_child(owned)
	if service.badges.is_empty():
		_list.add_child(UiBuild.label("ui.achievement.no_badges", &"MutedLabel"))
	for badge: InventoryItemModel in service.badges:
		var b: Button = Button.new()
		b.name = "Badge_%s" % badge.item_id
		b.theme_type_variation = &"PictureTile"
		b.toggle_mode = true
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		b.text = badge.display_name()
		b.set_pressed_no_signal(badge.item_id in showcase_draft)
		var t2: float = UiBuild.target()
		b.custom_minimum_size = Vector2(t2 * 2.4, t2 * 2.6)
		var thumb2: ItemThumb = ItemThumb.new()
		b.add_child(thumb2)
		thumb2.position = Vector2(8.0, 8.0)
		thumb2.size = Vector2(t2 * 2.4 - 16.0, t2 * 1.4)
		thumb2.set_item("badge", badge.item_id, badge.rarity)
		b.pressed.connect(toggle_badge.bind(badge.item_id))
		owned.add_child(b)


func _badge_rarity(code: String) -> String:
	for b: InventoryItemModel in service.badges:
		if b.item_id == code:
			return b.rarity
	return "common"


func _badge_name(code: String) -> String:
	for b: InventoryItemModel in service.badges:
		if b.item_id == code:
			return b.display_name()
	return ""


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render()
