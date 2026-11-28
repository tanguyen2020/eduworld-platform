class_name SeasonTab
extends VBoxContainer
## "Mùa" of the Event center (F111-AC-16, AC-17; EVENT_SEASON_SYSTEM §7, ADR-0049): the season name, "Cấp n/max", the
## season points, the points still possible today in neutral words, "Miễn phí cho mọi bạn" and "Phần thưởng chưa nhận
## sẽ tự gửi khi hết mùa", the virtualised SeasonTrackView and "Mùa trước". No season (`404`): Mây says "Mùa mới sắp bắt
## đầu". No premium track, no price, no purchase. UI only: `claim_requested` (AGENTS R1).

signal claim_requested(tier: int)

var _name: Label = null
var _tier: Label = null
var _points: Label = null
var _daily: Label = null
var _note: PanelContainer = null
var _ends: Label = null
var _none: Control = null
var _track: SeasonTrackView = null
var _history_title: Control = null
var _history: VBoxContainer = null
var _season_box: VBoxContainer = null
var _shown_id: String = ""
var _shown_history: Array[SeasonHistoryItemModel] = []


func _init() -> void:
	name = "SeasonTab"
	theme_type_variation = &"FormBox"
	_none = FriendsUi.empty_line("ui.season.no_season", "NoSeason")
	_none.visible = false
	add_child(_none)
	_season_box = UiBuild.vbox(&"FormBox")
	_season_box.name = "Season"
	add_child(_season_box)
	var head: HFlowContainer = HFlowContainer.new()
	head.name = "Head"
	_season_box.add_child(head)
	var icon: TextureRect = UiBuild.icon_rect("medal", 1.0)
	icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(icon)
	_name = UiBuild.data_label("", &"HeadingLabel", false)
	_name.name = "SeasonName"
	_name.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(_name)
	_tier = UiBuild.data_label("", &"SectionTag", false)
	_tier.name = "Tier"
	_tier.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(_tier)
	_points = UiBuild.data_label("", &"SectionTag", false)
	_points.name = "Points"
	_points.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	head.add_child(_points)
	_daily = UiBuild.keep_words(UiBuild.data_label("", &"MutedLabel"))
	_daily.name = "DailyLeft"
	_season_box.add_child(_daily)
	_note = UiBuild.panel(&"InfoBannerPanel")
	_note.name = "Note"
	var note_box: VBoxContainer = UiBuild.vbox()
	_note.add_child(note_box)
	note_box.add_child(UiBuild.keep_words(UiBuild.label("ui.season.free", &"InfoBannerLabel")))
	note_box.add_child(UiBuild.keep_words(UiBuild.label("ui.season.auto_grant", &"InfoBannerLabel")))
	_ends = UiBuild.keep_words(UiBuild.data_label("", &"InfoBannerLabel"))
	_ends.name = "Ends"
	note_box.add_child(_ends)
	_season_box.add_child(_note)
	_track = SeasonTrackView.new()
	UiBuild.expand(_track)
	_track.claim_requested.connect(func(t: int) -> void: claim_requested.emit(t))
	_season_box.add_child(_track)
	_history_title = FriendsUi.group_title("ui.season.history", "history")
	add_child(_history_title)
	_history = UiBuild.vbox(&"FormBox")
	_history.name = "History"
	add_child(_history)


func show_track(store: SeasonTrackStore, _band: String) -> void:
	var t: SeasonTrackModel = store.track
	_none.visible = store.missing or (store.loaded and t == null)
	_season_box.visible = t != null
	if t != null:
		_name.text = t.display_name()
		_tier.text = tr("ui.season.tier").format({"tier": LocaleFormat.number(t.tier), "max": LocaleFormat.number(t.max_tier)})
		_points.text = tr("ui.season.points").format({"points": LocaleFormat.number(t.points)})
		_daily.text = tr("ui.season.daily_remaining").format({"points": LocaleFormat.number(t.daily_points_remaining)}) \
				if t.daily_points_remaining > 0 else ""
		_daily.visible = not _daily.text.is_empty()
		var ends: int = LiveOpsTime.unix_of(t.ends_at)
		_ends.text = tr("ui.season.ends").format({"date": LiveOpsTime.ict_date(ends)}) if ends > 0 else ""
		_ends.visible = ends > 0
		_track.show_tiers(t.tiers)
		# F111 compact polish: on a compact phone the strip comes right under the season line and the note after it, so
		# the tiers show when the tab opens; outside compact mode the note first.
		if AppState.compact != (_note.get_index() > _track.get_index()):
			_season_box.move_child(_note, _track.get_index())
		if _shown_id != t.season_id:
			_shown_id = t.season_id
			_track.scroll_to.call_deferred(maxi(1, t.tier))
	if store.history == _shown_history and _history.get_child_count() > 0:
		return
	_shown_history = store.history
	UiBuild.clear(_history)
	for h: SeasonHistoryItemModel in store.history:
		var row: PanelContainer = UiBuild.panel(&"PartyMemberCard")
		row.name = "Past"
		var box: VBoxContainer = UiBuild.vbox()
		row.add_child(box)
		box.add_child(UiBuild.keep_words(UiBuild.data_label(tr("ui.season.history_line").format({"season": h.display_name(),
			"tier": LocaleFormat.number(h.final_tier)}), &"InstructionLabel")))
		if not h.auto_granted_tiers.is_empty():
			var tiers: PackedStringArray = []
			for n: int in h.auto_granted_tiers:
				tiers.append(LocaleFormat.number(n))
			box.add_child(UiBuild.keep_words(UiBuild.data_label(tr("ui.season.history_auto").format(
					{"tiers": ", ".join(tiers)}), &"MutedLabel")))
		_history.add_child(row)
	_history_title.visible = not store.history.is_empty()
	_history.visible = not store.history.is_empty()


## The strip, or null when there is no season.
func track_view() -> SeasonTrackView:
	return _track if _season_box.visible else null


func set_busy(busy: bool) -> void:
	_track.set_busy(busy)


func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("*", "Label", true, false):
		if (node as Label).is_visible_in_tree():
			out.append(tr((node as Label).text))
	for node: Node in find_children("*", "Button", true, false):
		if (node as Button).is_visible_in_tree():
			out.append(tr((node as Button).text))
	return out
