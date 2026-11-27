class_name TierBadge
extends PanelContainer
## The student's tier (F109-AC-01, AC-15; COMPETITION_SYSTEM §8.2): a medal icon and the tier name from the server
## (`getMyRatings`), or "Đang xếp hạng (x/N)" while provisional — never a rating number, for every grade.

var _label: Label = null


func _init() -> void:
	name = "TierBadge"
	theme_type_variation = &"HudChip"
	size_flags_vertical = Control.SIZE_SHRINK_CENTER
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("medal")
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	_label = UiBuild.data_label("", &"", false)
	_label.name = "TierLabel"
	_label.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(_label)
	visible = false


## Shows `tier` (null hides the badge: no tier yet this season).
func show_tier(tier: TierView) -> void:
	visible = tier != null
	if tier == null:
		return
	_label.text = TranslationServer.translate("ui.arena.tier.label").format({"tier": tier.label()})
	tooltip_text = _label.text


func text() -> String:
	return _label.text if visible else ""
