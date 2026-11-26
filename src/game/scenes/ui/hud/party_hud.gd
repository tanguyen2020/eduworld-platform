class_name PartyHud
extends VBoxContainer
## PartyHud — the `PartyChip` slot of the Main Hub HUD (F102-AC-01, AC-12, AC-13, AC-14; STUDENT_GAME_UX_SOCIAL §5):
## the "Nhóm" button (group icon + word, also the `P` key) opens the Party screen. While the student is in a party, a
## small strip shows each member: the 2D portrait, the crown of the leader and the state as a shape — a tick when
## Ready, the wifi-off sign and the faded frame while the member reconnects (the nickname and the state in words in the
## tooltip / screen reader). "Trưởng nhóm đang kết nối lại…" shows under it during the leader grace. Minimising the
## lobby keeps the party and this strip (AC-12). Only nicknames (AGENTS R16). Reads PartyStore; UI only (AGENTS R1).

signal party_pressed()

const MINI_SCALE: float = 0.9
const BADGE_SCALE: float = 0.55
const MINI_KEYS: Dictionary = {
	"ready": "ui.party.hud.member_ready", "waiting": "ui.party.hud.member_waiting",
	"away": "ui.party.hud.member_away",
}

var store: PartyStore = null
## Number of strip renders (tests).
var renders: int = 0
var _button: Button = null
var _strip_panel: PanelContainer = null
var _strip: HBoxContainer = null
var _banner: PanelContainer = null
var _banner_label: Label = null


func _init() -> void:
	name = "PartyHud"
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_build()


func _ready() -> void:
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	bind(null)
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## Follows `party_store` (null unbinds: previews, HUD-only tests).
func bind(party_store: PartyStore) -> void:
	if store != null:
		for pair: Array in [[store.party_updated, _on_party_updated], [store.member_updated, _on_member_updated]]:
			if (pair[0] as Signal).is_connected(pair[1] as Callable):
				(pair[0] as Signal).disconnect(pair[1] as Callable)
	store = party_store
	if store != null:
		store.party_updated.connect(_on_party_updated)
		store.member_updated.connect(_on_member_updated)
	render()


func button() -> Button:
	return _button


func member_count() -> int:
	return _strip.get_child_count() if _strip_panel.visible else 0


## The mini frame of `student_id` is faded (member reconnecting).
func is_faded(student_id: String) -> bool:
	var mini: Control = _strip.get_node_or_null(NodePath("Mini_%s" % student_id.right(12))) as Control
	return mini != null and mini.theme_type_variation == &"PartyMemberCardAway"


func banner_visible() -> bool:
	return _banner.visible


func apply_band() -> void:
	_button.custom_minimum_size = Vector2(UiBuild.target(), UiBuild.target())
	render()


func render() -> void:
	renders += 1
	UiBuild.clear(_strip)
	var party: PartyModel = store.party if store != null else null
	_button.text = "ui.party.hud.button_in_party" if party != null else "ui.party.hud.button"
	UiBuild.fit_wrapping(_button, float(UiTokens.SIZE_LAYOUT_280))
	_strip_panel.visible = party != null and not party.members.is_empty()
	_banner.visible = party != null and party.leader_reconnecting
	if party == null:
		return
	var side: float = UiBuild.target() * MINI_SCALE
	for m: PartyMemberModel in party.members:
		var mini: PanelContainer = UiBuild.panel(&"PartyMemberCardAway" if m.is_disconnected() else &"PartyMemberCard")
		mini.name = "Mini_%s" % m.student_id.right(12)
		mini.mouse_filter = Control.MOUSE_FILTER_PASS
		var state: String = "away" if m.is_disconnected() else ("ready" if m.ready else "waiting")
		mini.tooltip_text = tr(String(MINI_KEYS[state])).format({"nickname": m.nickname})
		var box: Control = Control.new()
		box.custom_minimum_size = Vector2(side, side)
		box.mouse_filter = Control.MOUSE_FILTER_IGNORE
		mini.add_child(box)
		var thumb: AvatarThumb = AvatarThumb.new()
		thumb.standalone = true
		thumb.set_anchors_preset(Control.PRESET_FULL_RECT)
		# Ready = the tick badge of a chosen tile (shape, not colour).
		thumb.ready_mark = state == "ready"
		var look: AvatarConfigModel = m.avatar()
		thumb.set_look(look if look != null else AvatarConfigModel.from_dict(PartyMemberCard.DEFAULT_AVATAR))
		box.add_child(thumb)
		if m.is_leader():
			box.add_child(_badge("crown", false, side))
		if state == "away":
			box.add_child(_badge("wifi_off", true, side))
		_strip.add_child(mini)


## A small icon in the top-left (crown) or bottom-right (wifi-off) corner of a mini portrait.
func _badge(icon_name: String, bottom_right: bool, side: float) -> TextureRect:
	var t: TextureRect = UiBuild.icon_rect(icon_name)
	var s: float = side * BADGE_SCALE
	t.custom_minimum_size = Vector2(s, s)
	var corner: float = 1.0 if bottom_right else 0.0
	t.anchor_left = corner
	t.anchor_right = corner
	t.anchor_top = corner
	t.anchor_bottom = corner
	t.offset_left = -s if bottom_right else 0.0
	t.offset_top = -s if bottom_right else 0.0
	t.offset_right = 0.0 if bottom_right else s
	t.offset_bottom = 0.0 if bottom_right else s
	return t


func _on_party_updated(_party: PartyModel) -> void:
	render()


func _on_member_updated(_student_id: String) -> void:
	render()


func _on_locale_changed(_locale: String) -> void:
	render()


func _build() -> void:
	_button = UiBuild.button("ui.party.hud.button", "group", &"SecondaryButton")
	_button.name = "PartyButton"
	_button.tooltip_text = "ui.party.hud.tooltip"
	_button.size_flags_horizontal = Control.SIZE_SHRINK_END
	# Like the other SideBar buttons: the label stays on one line up to the layout width, then wraps between words.
	_button.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_button.pressed.connect(party_pressed.emit)
	add_child(_button)
	_strip_panel = UiBuild.panel(&"HudPanel")
	_strip_panel.name = "Strip"
	_strip_panel.size_flags_horizontal = Control.SIZE_SHRINK_END
	_strip_panel.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_strip_panel)
	_strip = UiBuild.hbox(&"ToolbarBox")
	_strip.name = "Members"
	_strip.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_strip_panel.add_child(_strip)
	_banner = UiBuild.panel(&"WarningBannerPanel")
	_banner.name = "LeaderBanner"
	_banner.size_flags_horizontal = Control.SIZE_SHRINK_END
	_banner.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_banner)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	_banner.add_child(row)
	var icon: TextureRect = UiBuild.icon_rect("wifi_off", 0.75)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	row.add_child(icon)
	_banner_label = UiBuild.label("ui.party.leader_reconnecting", &"WarningBannerLabel", false)
	row.add_child(_banner_label)
	_strip_panel.visible = false
	_banner.visible = false
