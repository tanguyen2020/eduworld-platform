class_name BossEnergyBar
extends HBoxContainer
## S06 `BossEnergyBar` (F110-AC-03; STUDENT_GAME_UX_CORE S06 §12 "thanh năng lượng có số %"): the boss energy the server
## sent (`energy_remaining_pct`) as a bar AND a number in words ("Năng lượng 52%") — never computed, never raised by the
## client (F110-AC-05, AGENTS R5). F110 compact polish: on the compact phone canvas the bar sits in the first HUD row
## and says only "52%" beside the fire icon (`ui.boss.energy_short`); the words stay in its tooltip. UI only.

var value: float = 100.0
var compact: bool = false
var _bar: ProgressBar = null
var _label: Label = null


func _init() -> void:
	name = "BossEnergyBar"
	theme_type_variation = &"ToolbarBox"
	var icon: TextureRect = UiBuild.icon_rect("fire", 0.9)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	add_child(icon)
	_bar = UiBuild.progress_bar(1.0)
	_bar.name = "Bar"
	UiBuild.expand(_bar)
	add_child(_bar)
	_label = UiBuild.data_label("", &"HudLabel", false)
	_label.name = "Percent"
	_label.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	add_child(_label)


## The server value (0–100).
func show_energy(pct: float) -> void:
	value = clampf(pct, 0.0, 100.0)
	_bar.value = value / 100.0
	var args: Dictionary = {"pct": LocaleFormat.number(roundf(value), "", 0)}
	_label.text = tr("ui.boss.energy_short" if compact else "ui.boss.energy").format(args)
	tooltip_text = tr("ui.boss.energy").format(args)
	_bar.tooltip_text = tooltip_text


## Compact: the number only ("52%") and a shorter bar (three `SIZE_SPACE_32`, `PhaseTimer.COMPACT_BAR`), so the
## first HUD row keeps "Chat nhóm" and "Rời" on its line.
func set_compact(on: bool) -> void:
	compact = on
	_bar.custom_minimum_size.x = PhaseTimer.COMPACT_BAR if on else float(UiTokens.SIZE_LAYOUT_120)
	show_energy(value)


func text() -> String:
	return _label.text


func bar_value() -> float:
	return _bar.value
