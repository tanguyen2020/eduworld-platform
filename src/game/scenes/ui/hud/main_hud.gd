extends CanvasLayer
## Main Hub HUD (STUDENT_GAME_UX_CORE S03, F062 US1/US2; spec Clarifications 2026-11-16): only components with real
## data in M-C2 — PlayerBadge (nickname + grade band from the bootstrap), the "Nhà trường giao (n)" chip, quest
## journal, settings, "Về Quảng trường", the system banner, the ContextActionButton and the touch joystick. The slots
## XpBar, CurrencyChip, QuestTracker, Minimap and ChatDock exist in the layout but stay hidden until their
## owning features (F102: PartyChip = PartyHud "Nhóm" + members, InvitationToast). Reads AppState only and emits
## intents; no business logic, no network (AGENTS R1, G-4).
## Layout follows the grade band (`g1_3`: touch targets ≥ 64 dp from UiTokens density, no minimap — F062-AC-05);
## every string is a `ui.hub.*` key, re-rendered on a locale change (F062-AC-06). F096: every button carries an icon of
## design/icons/game beside its label (F096-AC-09), the "Việc tiếp theo" card always says what to do next
## (NextStepHint, F096-AC-11) and Mây's first-time guide (OnboardingHint, F096-AC-10) is hosted here.

signal context_action_pressed()
signal journal_pressed()
signal settings_pressed()
## F070: Tủ đồ (CustomizationStudio).
signal wardrobe_pressed()
## F071: World Map S04.
signal map_pressed()
signal home_pressed()
## F123: the bell was pressed — the owner opens the notification inbox.
signal notifications_pressed()
## F123: a notification toast was touched — the owner opens its screen through NotificationService (or the inbox).
signal notification_pressed(item: NotificationModel)
signal joystick_moved(vector: Vector2)
## F096: the "Việc tiếp theo" card was pressed — the owner opens `screen_id` with `args` (EventBus.ui_requested).
signal next_step_pressed(screen_id: StringName, args: Dictionary)
## F069: the Xu chip was pressed (S08 tab Shop) / the "Thêm" menu (Cửa hàng, Thành tựu, Núi Thử Thách, Hành trình).
signal shop_pressed()
signal menu_pressed()
## F102: the "Nhóm" button / P (no args) or the invitation toast (accepted, or a party switch to confirm) — the owner
## opens the Party screen with `args`.
signal party_requested(args: Dictionary)

## Smallest PlayerBadge width in touch-target units. The badge takes the room the top bar leaves next to its buttons:
## the whole nickname on one line when it fits; else it wraps between words and is never cut (F125 rule: no ellipsis on
## a student's name, no text below the band floor) — the badge grows taller and the left column follows its `resized`.
const BADGE_WIDTH_FACTOR: float = 3.0
## "Việc tiếp theo" card width in touch-target units (a two-line label wraps inside it, F096-AC-11).
const NEXT_STEP_WIDTH_FACTOR: float = 4.0
## F069: the "Việc tiếp theo" card never goes above this share of the screen height (its F096 place).
const NEXT_STEP_MIN_TOP: float = 0.14
const NO_MINIMAP_BANDS: PackedStringArray = ["g1_3"]
## F069: XpBar (level · title, XP, moon at the daily cap) in the PlayerBadge and the Xu CurrencyChip + "Thêm" menu on
## the right (SideBar) show once their data is loaded (ProgressionStore, WalletStore signals — no polling).
## Slots owned by later features (F072 quest tracker, F071 minimap). F102: PartyChip is the PartyHud. F112: the chat of the
## persistent channels is the ChatHubButton at the bottom right (before "Về Quảng trường") opening the ChatHub window;
## the old ChatDock slot stays hidden.
const FUTURE_SLOTS: PackedStringArray = ["QuestTracker", "Minimap", "ChatDock"]
const BAND_KEYS: Dictionary = {
	"g1_3": "ui.hub.grade_band.g1_3", "g4_5": "ui.hub.grade_band.g4_5", "g6_7": "ui.hub.grade_band.g6_7",
	"g8_9": "ui.hub.grade_band.g8_9",
}

var band: String = ""
## False for `g1_3` (S03 §10). The Minimap itself ships with F071.
var minimap_allowed: bool = true
var banner_key: String = ""
var banner_args: Dictionary = {}

@onready var _nickname: Label = %Nickname
@onready var _grade_band: Label = %GradeBand
@onready var _badge: PanelContainer = %PlayerBadge
@onready var _assignments_chip: Button = %AssignmentsChip
@onready var _journal: Button = %JournalButton
@onready var _settings: Button = %SettingsButton
@onready var _wardrobe: Button = %WardrobeButton
@onready var _map: Button = %MapButton
@onready var _home: Button = %HomeButton
@onready var _banner: PanelContainer = %SystemBanner
@onready var _banner_label: Label = %BannerLabel
@onready var _context_action: Button = %ContextActionButton
@onready var _joystick: Control = %Joystick
@onready var _network: PanelContainer = %NetworkBanner
@onready var _bell: Button = %NotificationBell
@onready var _toasts: VBoxContainer = %ToastStack
@onready var _next_step_button: Button = %NextStepButton
@onready var _onboarding: OnboardingHint = %OnboardingHint
@onready var _xp_bar: XpBar = %XpBar
@onready var _xp_panel: PanelContainer = %XpPanel
@onready var _next_step_card: PanelContainer = %NextStep
@onready var _currency: CurrencyChip = %CurrencyChip
@onready var _menu: Button = %MenuButton
## F102: PartyHud ("Nhóm" + member strip) and the invitation popup.
@onready var _party: PartyHud = %PartyChip
@onready var _invite_toast: InvitationToast = %InvitationToast
## F125 polish: controls of other layers on the right edge the invitation popup must not cover (WorldRoot sets the emote
## button) — the popup ends left of them and of the side bar.
var keep_clear: Array[Control] = []
## F112: the HUD chat button (ChatHub).
var _chat: ChatHubButton = null
## F111: the calendar button of the Event center (side bar, under "Thêm") with its static "new" dot.
var _events: EventCenterButton = null

## F096: the current "Việc tiếp theo" suggestion (NextStepHint.for_assignments).
var next_step: Dictionary = {}

## F123: in-game notifications (AppState.services; null in previews and unit tests of the HUD alone).
var notifications: NotificationService = null
## F102: the student's party (AppState.services; null in previews and HUD-only tests).
var party: PartyService = null


func _ready() -> void:
	for slot: String in FUTURE_SLOTS:
		(find_child(slot, true, false) as Control).visible = false
	_banner.visible = false
	_assignments_chip.pressed.connect(journal_pressed.emit)
	_journal.pressed.connect(journal_pressed.emit)
	_settings.pressed.connect(settings_pressed.emit)
	_wardrobe.pressed.connect(wardrobe_pressed.emit)
	_map.pressed.connect(map_pressed.emit)
	_home.pressed.connect(home_pressed.emit)
	_context_action.pressed.connect(context_action_pressed.emit)
	_joystick.connect("moved", joystick_moved.emit)
	_bell.pressed.connect(notifications_pressed.emit)
	_toasts.connect("toast_pressed", _on_toast_pressed)
	_next_step_button.pressed.connect(_on_next_step_pressed)
	# A Theme switch (band, compact) changes the font: the longest word of the name keeps its width.
	UiBuild.keep_words(_nickname)
	UiBuild.keep_words(_grade_band)
	_badge.resized.connect(_layout_left_column)
	(_badge.get_parent() as Control).resized.connect(_fit_badge)
	_xp_panel.resized.connect(_layout_left_column)
	_next_step_card.resized.connect(_layout_left_column)
	get_viewport().size_changed.connect(_layout_left_column)
	_currency.pressed.connect(shop_pressed.emit)
	_menu.pressed.connect(menu_pressed.emit)
	# F111: the calendar button shares the row of "Thêm" (one more side-bar row would push "Nhóm" under the emote button
	# on a phone).
	_events = EventCenterButton.new()
	var side: Node = _menu.get_parent()
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	row.name = "MenuRow"
	row.size_flags_horizontal = Control.SIZE_SHRINK_END
	row.mouse_filter = Control.MOUSE_FILTER_IGNORE
	side.add_child(row)
	side.move_child(row, _menu.get_index())
	side.remove_child(_menu)
	row.add_child(_events)
	row.add_child(_menu)
	# F112-AC-01, AC-05: the chat button (badge = unread of the clubs and DMs, "@" for mentions) at the bottom right,
	# before "Về Quảng trường" — clear of the toasts at the top and of the joystick at the bottom left.
	_chat = ChatHubButton.new()
	var bottom: Node = _home.get_parent()
	bottom.add_child(_chat)
	bottom.move_child(_chat, 0)
	# Owned by the HUD scene again so find_child / unique names still reach the moved button.
	row.owner = self
	_menu.owner = self
	_events.bind_store(AppState.services.live_events.store if AppState.services != null else null)
	_chat.bind_hub(AppState.services.chat_hub if AppState.services != null else null)
	_party.party_pressed.connect(_on_party_pressed)
	_invite_toast.open_requested.connect(party_requested.emit)
	EventBus.party_invitation_received.connect(_on_party_invitation)
	bind_party(AppState.services.party if AppState.services != null else null)
	bind_progression(AppState.services.progression.store if AppState.services != null else null,
			AppState.services.inventory.wallet_store if AppState.services != null else null)
	_onboarding.target_root = self
	bind_notifications(AppState.services.notifications if AppState.services != null else null)
	AppState.player_changed.connect(_on_player_changed)
	AppState.assignments_changed.connect(_on_assignments_changed)
	AppState.grade_band_changed.connect(apply_band)
	AppState.compact_changed.connect(_on_compact_changed)
	EventBus.locale_changed.connect(_on_locale_changed)
	apply_band(AppState.grade_band)
	_render()


func _exit_tree() -> void:
	bind_notifications(null)
	bind_party(null)
	_events.bind_store(null)
	_chat.bind_hub(null)
	if EventBus.party_invitation_received.is_connected(_on_party_invitation):
		EventBus.party_invitation_received.disconnect(_on_party_invitation)
	bind_progression(null, null)
	if get_viewport() != null and get_viewport().size_changed.is_connected(_layout_left_column):
		get_viewport().size_changed.disconnect(_layout_left_column)
	for pair: Array in [[AppState.player_changed, _on_player_changed], [AppState.assignments_changed,
			_on_assignments_changed], [AppState.grade_band_changed, apply_band], [AppState.compact_changed, _on_compact_changed],
			[EventBus.locale_changed, _on_locale_changed]]:
		var sig: Signal = pair[0] as Signal
		if sig.is_connected(pair[1] as Callable):
			sig.disconnect(pair[1] as Callable)


## F125: a phone window turned compact (or back): the touch targets follow without reloading the HUD.
func _on_compact_changed(_compact: bool) -> void:
	apply_band(band)


## F062-AC-05: density-dependent layout. Theme density is switched by ThemeService; touch targets follow UiTokens.
func apply_band(value: String) -> void:
	band = value if ThemeService.is_valid_band(value) else ThemeService.default_band()
	minimap_allowed = not band in NO_MINIMAP_BANDS
	var target: float = UiBuild.target_for(band)
	for button: Button in touch_buttons():
		button.custom_minimum_size = Vector2(target, target)
	_badge.custom_minimum_size = Vector2(target * BADGE_WIDTH_FACTOR, target)
	_next_step_button.custom_minimum_size.x = maxf(target * NEXT_STEP_WIDTH_FACTOR, float(UiTokens.SIZE_LAYOUT_280))
	# F123: the bell widens again to hold its count pill at the new touch target.
	_bell.call("set_count", notifications.badge if notifications != null else 0)
	_party.apply_band()
	_place_invite_toast()
	_render()


## F123-AC-15/16: follows the unread badge and the toasts of `service` (null unbinds).
func bind_notifications(service: NotificationService) -> void:
	if notifications != null:
		if notifications.toast_requested.is_connected(_on_toast_requested):
			notifications.toast_requested.disconnect(_on_toast_requested)
		if notifications.badge_changed.is_connected(_on_badge_changed):
			notifications.badge_changed.disconnect(_on_badge_changed)
	notifications = service
	if service != null:
		service.toast_requested.connect(_on_toast_requested)
		service.badge_changed.connect(_on_badge_changed)
	_bell.call("set_count", service.badge if service != null else 0)


## F102: follows the party of `service` (null unbinds): PartyHud strip, invitation popup.
func bind_party(service: PartyService) -> void:
	party = service
	_party.bind(service.store if service != null else null)
	if service == null:
		_invite_toast.dismiss()


## F102-AC-03: a new invitation pops the InvitationToast unless popups are off or the student is in an exam, a
## challenge or the activity player (then only the bell count changes — F123 inbox row).
func show_invitation(invitation: InvitationModel) -> bool:
	if party == null or invitation == null or not party.should_popup(invitation):
		return false
	_place_invite_toast()
	_invite_toast.present(invitation, party)
	return true


func invitation_toast() -> InvitationToast:
	return _invite_toast


func party_hud() -> PartyHud:
	return _party


## ContextActionButton for the Interactable the player stands in ("" hides it).
func set_context_action(interactable_id: String, prompt_key: String) -> void:
	_context_action.call("show_prompt", interactable_id, prompt_key)


## System banner (curfew warning, errors); `key` "" hides it.
func show_banner(key: String, args: Dictionary = {}) -> void:
	banner_key = key
	banner_args = args
	_render()


## F092: short hub notice (join refused, emote limits) under the network banner.
func show_hub_notice(key: String, args: Dictionary = {}) -> void:
	_network.call("show_hub_notice", key, args)


## F096-AC-18: every button of the HUD whose touch target follows the grade band.
func touch_buttons() -> Array[Button]:
	return [_assignments_chip, _journal, _bell, _settings, _wardrobe, _map, _home, _context_action, _next_step_button,
		_onboarding.find_child("Skip", true, false) as Button, _onboarding.find_child("Next", true, false) as Button,
		_currency, _menu, _events, _party.button(), _chat]


## F096-AC-10: opens Mây's guide (the owner decides when: first entry on the device or right after S02).
func start_onboarding() -> void:
	_onboarding.start()


func onboarding() -> OnboardingHint:
	return _onboarding


func network_banner() -> PanelContainer:
	return _network


func assignments_chip_text() -> String:
	return _assignments_chip.text


func _render() -> void:
	if _nickname == null:
		return
	# F098: the SideBar buttons keep their label whole (wrap only past a long translation, never letter by letter).
	UiBuild.fit_wrapping(_menu, float(UiTokens.SIZE_LAYOUT_280))
	UiBuild.fit_wrapping(_events, float(UiTokens.SIZE_LAYOUT_280))
	if _currency.visible:
		UiBuild.fit_wrapping(_currency, float(UiTokens.SIZE_LAYOUT_280))
	var nickname: String = LoginFormModel.nickname_of(AppState.player)
	_nickname.text = nickname if not nickname.is_empty() else tr("ui.hub.player_default")
	_nickname.tooltip_text = _nickname.text
	_grade_band.text = tr(String(BAND_KEYS.get(band, "ui.hub.grade_band.g1_3")))
	_grade_band.tooltip_text = _grade_band.text
	UiBuild.fit_words(_nickname)
	UiBuild.fit_words(_grade_band)
	_fit_badge.call_deferred()
	var count: int = AppState.assignments.size()
	_assignments_chip.text = tr("ui.hub.assignments_count").format({"count": count}) if count > 0 \
			else tr("ui.hub.assignments")
	next_step = NextStepHint.for_assignments(count)
	_next_step_button.text = tr(String(next_step["label_key"])).format(next_step["args"] as Dictionary)
	_next_step_button.icon = load(String(next_step["icon"])) as Texture2D
	_banner.visible = not banner_key.is_empty()
	_banner_label.text = tr(banner_key).format(banner_args) if not banner_key.is_empty() else ""


func _on_party_pressed() -> void:
	party_requested.emit({})


func _on_party_invitation(invitation: InvitationModel) -> void:
	show_invitation(invitation)


## The popup sits at the top right under the top bar, at most the layout width wide.
func _place_invite_toast() -> void:
	if _invite_toast == null:
		return
	var gap: float = float(UiTokens.SIZE_SPACE_8)
	var top_bar: Control = find_child("TopBar", true, false) as Control
	# F125 polish: between the left column (next step, XP) and the right edge controls (side bar, emote button) — on the
	# compact canvas the popup would otherwise hide "Vào nhóm" under the side bar / emote button. It never gets narrower
	# than the side bar would allow alone (a too narrow window keeps the old width).
	var root: Control = _invite_toast.get_parent() as Control
	var origin: float = root.get_global_rect().position.x
	var right: float = root.size.x - gap
	var avoid: Array[Control] = [find_child("SideBar", true, false) as Control]
	avoid.append_array(keep_clear)
	for c: Control in avoid:
		if c != null and c.is_visible_in_tree() and c.size.x > 0.0:
			right = minf(right, c.get_global_rect().position.x - origin - gap)
	var left: float = gap
	for c: Control in [_next_step_card, _xp_panel]:
		if c != null and c.is_visible_in_tree():
			left = maxf(left, c.get_global_rect().end.x - origin + gap)
	var width: float = minf(float(UiTokens.SIZE_LAYOUT_660), right - left)
	if width < float(UiTokens.SIZE_LAYOUT_280) * 1.5:
		right = root.size.x - gap
		width = float(UiTokens.SIZE_LAYOUT_660)
	_invite_toast.offset_right = right - root.size.x
	_invite_toast.offset_left = _invite_toast.offset_right - width
	_invite_toast.offset_top = (top_bar.size.y if top_bar != null else UiBuild.target()) + gap


func _on_next_step_pressed() -> void:
	next_step_pressed.emit(next_step["screen_id"] as StringName, next_step["screen_args"] as Dictionary)


## F123-AC-16/17: the text of the notification in the current language (message code + params, else title/body).
func _on_toast_requested(item: NotificationModel) -> void:
	_toasts.call("push", item.display_text(), item)


func _on_toast_pressed(tag: Object) -> void:
	notification_pressed.emit(tag as NotificationModel)


func _on_badge_changed(count: int) -> void:
	_bell.call("set_count", count)


func _on_player_changed(_player: Dictionary) -> void:
	_render()


func _on_assignments_changed(_count: int) -> void:
	_render()


func _on_locale_changed(_locale: String) -> void:
	_render()


## F069-AC-01: follows the progression and the wallet of the session (null unbinds; previews and HUD-only tests).
func bind_progression(progression: ProgressionStore, wallet: WalletStore) -> void:
	if _xp_bar.store != null and _xp_bar.store.progression_changed.is_connected(_on_progression_changed):
		_xp_bar.store.progression_changed.disconnect(_on_progression_changed)
	_xp_bar.bind(progression)
	_currency.bind(wallet)
	if progression != null and not progression.progression_changed.is_connected(_on_progression_changed):
		progression.progression_changed.connect(_on_progression_changed)
	_on_progression_changed(progression.progression if progression != null else null)


## F111: the Event center button of the side bar.
func event_center_button() -> EventCenterButton:
	return _events


func xp_bar() -> XpBar:
	return _xp_bar


func currency_chip() -> CurrencyChip:
	return _currency


## F069: the XpPanel (level · title, XP bar) sits under the PlayerBadge once the progression is known; the "Việc tiếp
## theo" card moves under it (signal-driven layout, no `_process`).
func _on_progression_changed(progression: ProgressionModel) -> void:
	_xp_panel.visible = progression != null
	_layout_left_column.call_deferred()


## The PlayerBadge width: its texts on one line when the top bar has the room beside its buttons; else the width of two
## balanced lines (the name wraps between words, e.g. "Cú Mèo Chăm" / "Chỉ 27"); never less than BADGE_WIDTH_FACTOR
## touch targets and never so wide that the buttons wrap (with less room the name takes more lines — never cut).
func _fit_badge() -> void:
	if _badge == null or not is_inside_tree():
		return
	var bar: Control = _badge.get_parent() as Control
	var gap: float = float(bar.get_theme_constant(&"h_separation"))
	var others: float = 0.0
	for child: Node in bar.get_children():
		var c: Control = child as Control
		if c == null or c == _badge or not c.visible or (c.size_flags_horizontal & Control.SIZE_EXPAND) != 0:
			continue
		others += c.get_combined_minimum_size().x + gap
	var base: float = UiBuild.target_for(band) * BADGE_WIDTH_FACTOR
	var want: float = base
	var font: Font = _nickname.get_theme_font(&"font")
	if font != null:
		var px: int = _nickname.get_theme_font_size(&"font_size")
		var band_w: float = font.get_string_size(_grade_band.text, HORIZONTAL_ALIGNMENT_LEFT, -1.0, px).x
		var pad: float = 0.0
		for box: StyleBox in [_badge.get_theme_stylebox(&"panel"), _nickname.get_theme_stylebox(&"normal")]:
			if box != null:
				pad += box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT)
		var one_line: float = font.get_string_size(_nickname.text, HORIZONTAL_ALIGNMENT_LEFT, -1.0, px).x
		want = ceilf(maxf(one_line, band_w) + pad) + 1.0
		var room_now: float = floorf(bar.size.x - others - gap) - 1.0
		if want > room_now:
			want = ceilf(maxf(_two_line_width(_nickname.text, font, px), band_w) + pad) + 1.0
	var room: float = floorf(bar.size.x - others - gap) - 1.0
	var width: float = clampf(want, base, maxf(base, room))
	if not is_equal_approx(_badge.custom_minimum_size.x, width):
		_badge.custom_minimum_size.x = width


## Width of `text` wrapped into two lines at the most balanced word break (one line when it has a single word).
static func _two_line_width(text: String, font: Font, px: int) -> float:
	var words: PackedStringArray = text.split(" ", false)
	var best: float = font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1.0, px).x
	for i: int in range(1, words.size()):
		var first: float = font.get_string_size(" ".join(words.slice(0, i)), HORIZONTAL_ALIGNMENT_LEFT, -1.0, px).x
		var second: float = font.get_string_size(" ".join(words.slice(i)), HORIZONTAL_ALIGNMENT_LEFT, -1.0, px).x
		best = minf(best, maxf(first, second))
	return best


func _layout_left_column() -> void:
	if _badge == null or not is_inside_tree():
		return
	var gap: float = float(UiTokens.SIZE_SPACE_8)
	var rect: Rect2 = _badge.get_global_rect()
	var y: float = rect.end.y + gap
	var width: float = maxf(_next_step_card.size.x, rect.size.x)
	if _xp_panel.visible:
		_xp_panel.position = Vector2(rect.position.x, y)
		_xp_panel.custom_minimum_size.x = width
		_xp_panel.reset_size()
		y = _xp_panel.position.y + _xp_panel.size.y + gap
	_next_step_card.anchor_top = 0.0
	_next_step_card.anchor_bottom = 0.0
	_next_step_card.offset_top = maxf(y, get_viewport().get_visible_rect().size.y * NEXT_STEP_MIN_TOP)
	_next_step_card.offset_bottom = _next_step_card.offset_top


## F069 (S06 §3): during a solo challenge run the HUD steps back — the top bar, the side bar, the XP panel and the
## "Việc tiếp theo" card hide so the ChallengeHud strip has the top of the screen; they come back when it ends.
func set_challenge_mode(on: bool) -> void:
	(find_child("TopBar", true, false) as Control).visible = not on
	(find_child("SideBar", true, false) as Control).visible = not on
	_next_step_card.visible = not on
	_xp_panel.visible = not on and _xp_bar.store != null and _xp_bar.store.progression != null
