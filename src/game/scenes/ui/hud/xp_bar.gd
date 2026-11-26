class_name XpBar
extends VBoxContainer
## S03 `XpBar` + level/title line of the PlayerBadge (F069-AC-01, AC-02; STUDENT_GAME_UX_CORE S03 §8–§9): "Cấp 3 · Tân
## binh", a bar of `xp_in_level` over `xp_in_level + xp_to_next` with the numbers, the moon icon + tooltip
## `ui.hub.daily_cap_reached` when today's XP cap is reached, a full bar with "Cấp tối đa" at the maximum level, and the
## granted XP flying up for ≤ 0.8 s (none with "Giảm chuyển động"). Every value comes from ProgressionStore signals —
## nothing in `_process`, nothing computed (F069-AC-27, FR-GAM-007). Hidden until the first `getProgression`.

## S03 §9: the flying number lasts at most this long.
const FLY_SEC: float = 0.8

var store: ProgressionStore = null
var _level: Label = null
var _title: Label = null
var _bar: ProgressBar = null
var _numbers: Label = null
var _moon: TextureRect = null
var _fly: Label = null
var _tween: Tween = null


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_PASS
	_level = UiBuild.data_label("", &"HudLabel", false)
	_level.name = "LevelLabel"
	_level.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	add_child(_level)
	_title = UiBuild.data_label("", &"HudLabel", false)
	_title.name = "TitleLabel"
	_title.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	add_child(_title)
	var row: HBoxContainer = UiBuild.hbox()
	add_child(row)
	_bar = ProgressBar.new()
	_bar.name = "Bar"
	_bar.show_percentage = false
	_bar.max_value = 1.0
	_bar.step = 0.0
	_bar.mouse_filter = Control.MOUSE_FILTER_PASS
	_bar.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_bar.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	_bar.custom_minimum_size = Vector2(0.0, float(UiTokens.COMPONENT_PROGRESS_HEIGHT) * 0.6)
	row.add_child(_bar)
	_moon = UiBuild.icon_rect("moon", 0.75)
	_moon.name = "Moon"
	_moon.mouse_filter = Control.MOUSE_FILTER_PASS
	_moon.tooltip_text = "ui.hub.daily_cap_reached"
	row.add_child(_moon)
	_numbers = UiBuild.data_label("", &"HudLabel", false)
	_numbers.name = "Numbers"
	row.add_child(_numbers)
	_fly = UiBuild.data_label("", &"HudLabel", false)
	_fly.name = "Fly"
	_fly.visible = false
	_fly.top_level = false
	add_child(_fly)
	EventBus.locale_changed.connect(_on_locale_changed)
	visible = false


func _exit_tree() -> void:
	bind(null)
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## Follows `value` (null unbinds).
func bind(value: ProgressionStore) -> void:
	if store != null:
		if store.progression_changed.is_connected(render):
			store.progression_changed.disconnect(render)
		if store.xp_gained.is_connected(fly):
			store.xp_gained.disconnect(fly)
	store = value
	if store != null:
		store.progression_changed.connect(render)
		store.xp_gained.connect(fly)
		render(store.progression)


func render(p: ProgressionModel) -> void:
	if _bar == null:
		return
	visible = p != null
	if p == null:
		return
	var title: String = p.title_text()
	_level.text = tr("ui.progression.level").format({"level": LocaleFormat.number(p.level)})
	_title.text = title
	_title.visible = not title.is_empty()
	_level.tooltip_text = level_text()
	_title.tooltip_text = _level.tooltip_text
	_bar.value = p.fraction()
	if p.is_max_level():
		_numbers.text = tr("ui.progression.max_level")
	else:
		_numbers.text = tr("ui.progression.xp_numbers").format({"xp": LocaleFormat.number(p.xp_in_level),
			"total": LocaleFormat.number(p.span())})
	_moon.visible = p.capped
	_bar.tooltip_text = tr("ui.hub.daily_cap_reached") if p.capped else _numbers.text


## The flying "+40 XP" (S03 §9): ≤ FLY_SEC, skipped when "Giảm chuyển động" is on.
func fly(delta: int) -> void:
	if _fly == null or delta <= 0:
		return
	_fly.text = tr("ui.progression.xp_gained").format({"xp": LocaleFormat.number(delta)})
	if bool(Settings.reduce_motion):
		_fly.visible = false
		return
	_fly.visible = true
	_fly.modulate.a = 1.0
	if _tween != null:
		_tween.kill()
	_tween = create_tween()
	_tween.tween_property(_fly, "modulate:a", 0.0, FLY_SEC)
	_tween.tween_callback(func() -> void: _fly.visible = false)


## "Cấp 3 · Nhà thám hiểm nhí" (level and title as shown).
func level_text() -> String:
	if _level == null:
		return ""
	return _level.text if _title.text.is_empty() else "%s · %s" % [_level.text, _title.text]


func numbers_text() -> String:
	return _numbers.text if _numbers != null else ""


func moon_visible() -> bool:
	return _moon != null and _moon.visible


func bar_value() -> float:
	return _bar.value if _bar != null else 0.0


func _on_locale_changed(_locale: String) -> void:
	render(store.progression if store != null else null)
