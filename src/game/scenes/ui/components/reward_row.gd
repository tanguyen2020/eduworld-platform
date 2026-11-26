class_name RewardRow
extends HFlowContainer
## Rewards the server granted or announces, as chips: icon + text ("40 XP", "50 Xu", the item name), wrapping on small
## screens (S07 `RewardCarousel`, S06 `RewardPreview`, S09 `RewardChips`). `large` (g1_3) shows bigger icons — young
## students read the picture. Never a "Nhận" button: rewards are already in the bag (FR-RWD-003, AC-UX-0702).

var large: bool = false


func set_rewards(rewards: Array[RewardModel], big: bool = false) -> void:
	large = big
	UiBuild.clear(self)
	for r: RewardModel in rewards:
		add_child(_chip(r))


func chip_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for chip: Node in get_children():
		var l: Label = chip.find_child("Text", true, false) as Label
		if l != null:
			out.append(l.text)
	return out


func _chip(r: RewardModel) -> Control:
	var p: PanelContainer = UiBuild.panel(&"QuestCard")
	var h: HBoxContainer = UiBuild.hbox()
	p.add_child(h)
	var icon: TextureRect = TextureRect.new()
	icon.texture = load(r.icon_path()) as Texture2D
	icon.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	icon.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	var side: float = float(UiTokens.SIZE_ICON_40 if large else UiTokens.COMPONENT_ICON_SIZE)
	icon.custom_minimum_size = Vector2(side, side)
	h.add_child(icon)
	var text: Label = UiBuild.data_label(r.display_text(), &"", false)
	text.name = "Text"
	text.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	h.add_child(text)
	return p
