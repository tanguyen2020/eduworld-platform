class_name SeasonTrackStore
extends RefCounted
## RAM store of the "Mùa" tab (F111 US4; ADR-0064): the free season track of the current season (`null` with `missing`
## when the server has none — "Mùa mới sắp bắt đầu") and "Mùa trước". Cleared on logout / session replaced.

signal track_changed()
signal history_changed()

var track: SeasonTrackModel = null
## `getSeasonTrack` answered `404`: no season right now.
var missing: bool = false
var loaded: bool = false
var history: Array[SeasonHistoryItemModel] = []


func set_track(value: SeasonTrackModel) -> void:
	track = value
	missing = value == null
	loaded = true
	track_changed.emit()


func set_missing() -> void:
	track = null
	missing = true
	loaded = true
	track_changed.emit()


## `season.track.updated` of the current season.
func apply_update(payload: Dictionary) -> void:
	if track == null or DataRead.str_of(payload, "season_id") != track.season_id:
		return
	track.apply_update(payload)
	track_changed.emit()


## The server confirmed the claim of `tier`.
func mark_claimed(tier: int) -> void:
	var t: SeasonTierModel = track.tier_model(tier) if track != null else null
	if t != null:
		t.claimed = true
		t.reached = true
		track_changed.emit()


func set_history(list: Array[SeasonHistoryItemModel]) -> void:
	history = list
	history_changed.emit()


func clear() -> void:
	track = null
	missing = false
	loaded = false
	history = []
	track_changed.emit()
	history_changed.emit()
