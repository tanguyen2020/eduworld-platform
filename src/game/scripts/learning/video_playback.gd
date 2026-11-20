class_name VideoPlayback
extends RefCounted
## Playback of a Theora file for the video renderer (F065-AC-15): load, play, pause, seek and position in ms. The
## production one wraps a `VideoStreamPlayer` of the renderer scene; tests use a scripted fake (tests/fakes,
## TEST-ONLY) so cue stops, seeks and stalls are checked without decoding video.

signal playback_finished()

var player: VideoStreamPlayer = null


func _init(video_player: VideoStreamPlayer = null) -> void:
	player = video_player
	if player != null:
		player.finished.connect(func() -> void: playback_finished.emit())


## Loads a local `.ogv` file; false when it cannot be opened.
func load_file(path: String) -> bool:
	if player == null or path.is_empty() or not FileAccess.file_exists(path):
		return false
	var stream: VideoStreamTheora = VideoStreamTheora.new()
	stream.file = path
	player.stream = stream
	return true


func play() -> void:
	if player == null:
		return
	if player.is_playing():
		player.paused = false
	else:
		player.play()


func pause() -> void:
	if player != null:
		player.paused = true


func seek(ms: int) -> void:
	if player != null:
		player.stream_position = maxf(0.0, float(ms) / 1000.0)


func position_ms() -> int:
	return int(player.stream_position * 1000.0) if player != null else 0


func duration_ms() -> int:
	return int(player.get_stream_length() * 1000.0) if player != null and player.stream != null else 0


func is_playing() -> bool:
	return player != null and player.is_playing() and not player.paused


func set_speed(rate: float) -> void:
	if player != null:
		player.speed_scale = clampf(rate, 0.5, 2.0)
