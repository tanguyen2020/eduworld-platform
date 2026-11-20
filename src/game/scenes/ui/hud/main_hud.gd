extends CanvasLayer
## Main Hub HUD (STUDENT_GAME_UX_CORE S03, F062 US1/US2; spec Clarifications 2026-11-16): only components with real
## data in M-C2 — PlayerBadge (nickname + grade band from the bootstrap), the "Nhà trường giao (n)" chip, quest
## journal, settings, "Về Quảng trường", the system banner, the ContextActionButton and the touch joystick. The slots
## XpBar, CurrencyChip, QuestTracker, Minimap, ChatDock and PartyChip exist in the layout but stay hidden until their
## owning features. Reads AppState only and emits intents; no business logic, no network (AGENTS R1, G-4).
## Layout follows the grade band (`g1_3`: touch targets ≥ 64 dp from UiTokens density, no minimap — F062-AC-05);
## every string is a `ui.hub.*` key, re-rendered on a locale change (F062-AC-06).

signal context_action_pressed()
signal journal_pressed()
signal settings_pressed()
## F070: Tủ đồ (CustomizationStudio).
signal wardrobe_pressed()
signal home_pressed()
signal joystick_moved(vector: Vector2)

## PlayerBadge width in touch-target units (nickname ≤ 24 chars + grade band fit at every density).
const BADGE_WIDTH_FACTOR: float = 3.0
const NO_MINIMAP_BANDS: PackedStringArray = ["g1_3"]
## Slots owned by later features (F074 XP, F075 Xu, F072 quest tracker, F071 minimap, F110 chat, F102 party).
const FUTURE_SLOTS: PackedStringArray = ["XpBar", "CurrencyChip", "QuestTracker", "Minimap", "ChatDock", "PartyChip"]
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
@onready var _home: Button = %HomeButton
@onready var _banner: PanelContainer = %SystemBanner
@onready var _banner_label: Label = %BannerLabel
@onready var _context_action: Button = %ContextActionButton
@onready var _joystick: Control = %Joystick


func _ready() -> void:
	for slot: String in FUTURE_SLOTS:
		(find_child(slot, true, false) as Control).visible = false
	_banner.visible = false
	_assignments_chip.pressed.connect(journal_pressed.emit)
	_journal.pressed.connect(journal_pressed.emit)
	_settings.pressed.connect(settings_pressed.emit)
	_wardrobe.pressed.connect(wardrobe_pressed.emit)
	_home.pressed.connect(home_pressed.emit)
	_context_action.pressed.connect(context_action_pressed.emit)
	_joystick.connect("moved", joystick_moved.emit)
	AppState.player_changed.connect(_on_player_changed)
	AppState.assignments_changed.connect(_on_assignments_changed)
	AppState.grade_band_changed.connect(apply_band)
	EventBus.locale_changed.connect(_on_locale_changed)
	apply_band(AppState.grade_band)
	_render()


func _exit_tree() -> void:
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
	for button: Button in [_assignments_chip, _journal, _settings, _wardrobe, _home, _context_action]:
		button.custom_minimum_size = Vector2(target, target)
	_badge.custom_minimum_size = Vector2(target * BADGE_WIDTH_FACTOR, target)
	_render()


## ContextActionButton for the Interactable the player stands in ("" hides it).
func set_context_action(interactable_id: String, prompt_key: String) -> void:
	_context_action.call("show_prompt", interactable_id, prompt_key)


## System banner (curfew warning, errors); `key` "" hides it.
func show_banner(key: String, args: Dictionary = {}) -> void:
	banner_key = key
	banner_args = args
	_render()


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
	_banner.visible = not banner_key.is_empty()
	_banner_label.text = tr(banner_key).format(banner_args) if not banner_key.is_empty() else ""


func _on_player_changed(_player: Dictionary) -> void:
	_render()


func _on_assignments_changed(_count: int) -> void:
	_render()


func _on_locale_changed(_locale: String) -> void:
	_render()
