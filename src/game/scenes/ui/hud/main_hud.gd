extends CanvasLayer
## Main Hub HUD (STUDENT_GAME_UX_CORE S03, F062 US1/US2; spec Clarifications 2026-11-16): only components with real
## data in M-C2 — PlayerBadge (nickname + grade band from the bootstrap), the "Nhà trường giao (n)" chip, quest
## journal, settings, "Về Quảng trường", the system banner, the ContextActionButton and the touch joystick. The slots
## XpBar, CurrencyChip, QuestTracker, Minimap, ChatDock and PartyChip exist in the layout but stay hidden until their
## owning features. Reads AppState only and emits intents; no business logic, no network (AGENTS R1, G-4).
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
## F064: 🔔 or a toast touched — the owner opens "Nhà trường giao" through NotificationService.
signal notifications_pressed()
signal joystick_moved(vector: Vector2)
## F096: the "Việc tiếp theo" card was pressed — the owner opens `screen_id` with `args` (EventBus.ui_requested).
signal next_step_pressed(screen_id: StringName, args: Dictionary)
## F069: the Xu chip was pressed (S08 tab Shop) / the "Thêm" menu (Cửa hàng, Thành tựu, Núi Thử Thách, Hành trình).
signal shop_pressed()
signal menu_pressed()

## PlayerBadge width in touch-target units (nickname ≤ 24 chars + grade band fit at every density).
const BADGE_WIDTH_FACTOR: float = 3.0
## "Việc tiếp theo" card width in touch-target units (a two-line label wraps inside it, F096-AC-11).
const NEXT_STEP_WIDTH_FACTOR: float = 4.0
## F069: the "Việc tiếp theo" card never goes above this share of the screen height (its F096 place).
const NEXT_STEP_MIN_TOP: float = 0.14
const NO_MINIMAP_BANDS: PackedStringArray = ["g1_3"]
## F069: XpBar (level · title, XP, moon at the daily cap) in the PlayerBadge and the Xu CurrencyChip + "Thêm" menu on
## the right (SideBar) show once their data is loaded (ProgressionStore, WalletStore signals — no polling).
## Slots owned by later features (F074 XP, F075 Xu, F072 quest tracker, F071 minimap, F110 chat, F102 party).
const FUTURE_SLOTS: PackedStringArray = ["QuestTracker", "Minimap", "ChatDock", "PartyChip"]
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

## F096: the current "Việc tiếp theo" suggestion (NextStepHint.for_assignments).
var next_step: Dictionary = {}

## F064: notices of new school work (AppState.services; null in previews and unit tests of the HUD alone).
var notifications: NotificationService = null


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
	_toasts.connect("toast_pressed", notifications_pressed.emit)
	_next_step_button.pressed.connect(_on_next_step_pressed)
	_badge.resized.connect(_layout_left_column)
	_xp_panel.resized.connect(_layout_left_column)
	_next_step_card.resized.connect(_layout_left_column)
	get_viewport().size_changed.connect(_layout_left_column)
	_currency.pressed.connect(shop_pressed.emit)
	_menu.pressed.connect(menu_pressed.emit)
	bind_progression(AppState.services.progression.store if AppState.services != null else null,
			AppState.services.inventory.wallet_store if AppState.services != null else null)
	_onboarding.target_root = self
	bind_notifications(AppState.services.notifications if AppState.services != null else null)
	AppState.player_changed.connect(_on_player_changed)
	AppState.assignments_changed.connect(_on_assignments_changed)
	AppState.grade_band_changed.connect(apply_band)
	EventBus.locale_changed.connect(_on_locale_changed)
	apply_band(AppState.grade_band)
	_render()


func _exit_tree() -> void:
	bind_notifications(null)
	bind_progression(null, null)
	if get_viewport() != null and get_viewport().size_changed.is_connected(_layout_left_column):
		get_viewport().size_changed.disconnect(_layout_left_column)
	for pair: Array in [[AppState.player_changed, _on_player_changed], [AppState.assignments_changed,
			_on_assignments_changed], [AppState.grade_band_changed, apply_band], [EventBus.locale_changed, _on_locale_changed]]:
		var sig: Signal = pair[0] as Signal
		if sig.is_connected(pair[1] as Callable):
			sig.disconnect(pair[1] as Callable)


## F062-AC-05: density-dependent layout. Theme density is switched by ThemeService; touch targets follow UiTokens.
func apply_band(value: String) -> void:
	band = value if ThemeService.is_valid_band(value) else ThemeService.default_band()
	minimap_allowed = not band in NO_MINIMAP_BANDS
	var target: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band])
	for button: Button in touch_buttons():
		button.custom_minimum_size = Vector2(target, target)
	_badge.custom_minimum_size = Vector2(target * BADGE_WIDTH_FACTOR, target)
	_next_step_button.custom_minimum_size.x = maxf(target * NEXT_STEP_WIDTH_FACTOR, float(UiTokens.SIZE_LAYOUT_280))
	_render()


## F064-AC-20…22: follows the badge and the toasts of `service` (null unbinds).
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


## ContextActionButton for the Interactable the player stands in ("" hides it).
func set_context_action(interactable_id: String, prompt_key: String) -> void:
	_context_action.call("show_prompt", interactable_id, prompt_key)


## System banner (curfew warning, errors); `key` "" hides it.
func show_banner(key: String, args: Dictionary = {}) -> void:
	banner_key = key
	banner_args = args
	_render()


## F092: short hub notice (join refused, emote limits) under the network banner.
func show_hub_notice(key: String) -> void:
	_network.call("show_hub_notice", key)


## F096-AC-18: every button of the HUD whose touch target follows the grade band.
func touch_buttons() -> Array[Button]:
	return [_assignments_chip, _journal, _bell, _settings, _wardrobe, _map, _home, _context_action, _next_step_button,
		_onboarding.find_child("Skip", true, false) as Button, _onboarding.find_child("Next", true, false) as Button,
		_currency, _menu]


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
	var nickname: String = LoginFormModel.nickname_of(AppState.player)
	_nickname.text = nickname if not nickname.is_empty() else tr("ui.hub.player_default")
	_nickname.tooltip_text = _nickname.text
	_grade_band.text = tr(String(BAND_KEYS.get(band, "ui.hub.grade_band.g1_3")))
	_grade_band.tooltip_text = _grade_band.text
	var count: int = AppState.assignments.size()
	_assignments_chip.text = tr("ui.hub.assignments_count").format({"count": count}) if count > 0 \
			else tr("ui.hub.assignments")
	next_step = NextStepHint.for_assignments(count)
	_next_step_button.text = tr(String(next_step["label_key"])).format(next_step["args"] as Dictionary)
	_next_step_button.icon = load(String(next_step["icon"])) as Texture2D
	_banner.visible = not banner_key.is_empty()
	_banner_label.text = tr(banner_key).format(banner_args) if not banner_key.is_empty() else ""


func _on_next_step_pressed() -> void:
	next_step_pressed.emit(next_step["screen_id"] as StringName, next_step["screen_args"] as Dictionary)


func _on_toast_requested(key: String, args: Dictionary) -> void:
	_toasts.call("push", tr(key).format(args))


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


func xp_bar() -> XpBar:
	return _xp_bar


func currency_chip() -> CurrencyChip:
	return _currency


## F069: the XpPanel (level · title, XP bar) sits under the PlayerBadge once the progression is known; the "Việc tiếp
## theo" card moves under it (signal-driven layout, no `_process`).
func _on_progression_changed(progression: ProgressionModel) -> void:
	_xp_panel.visible = progression != null
	_layout_left_column.call_deferred()


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
