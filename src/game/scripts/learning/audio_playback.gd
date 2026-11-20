class_name AudioPlayback
extends RefCounted
## Playback of an Ogg Vorbis listening file for the audio renderer (F066-AC-18): load, play, pause, seek, speed and
## position in ms on the Voice bus. The production one wraps an `AudioStreamPlayer` of the renderer scene; tests use a
## scripted fake (tests/fakes, TEST-ONLY). There is no recording path (ADR-0059, ADR-0072).

signal playback_finished()

const BUS: StringName = &"Voice"

var player: AudioStreamPlayer = null
var _pos_ms: int = 0


func _init(audio_player: AudioStreamPlayer = null) -> void:
	player = audio_player
	if player != null:
		if AudioServer.get_bus_index(BUS) >= 0:
			player.bus = BUS
		player.finished.connect(func() -> void: playback_finished.emit())


## Loads a local `.ogg` file; false when it cannot be opened or decoded.
func load_file(path: String) -> bool:
	if player == null or path.is_empty() or not FileAccess.file_exists(path):
		return false
	var stream: AudioStreamOggVorbis = AudioStreamOggVorbis.load_from_file(path)
	if stream == null:
		return false
	player.stream = stream
	return true


func play() -> void:
	if player == null:
		return
	if player.stream_paused:
		player.stream_paused = false
	elif not player.playing:
		player.play(float(_pos_ms) / 1000.0)


func pause() -> void:
	if player != null and player.playing:
		_pos_ms = position_ms()
		player.stream_paused = true


func seek(ms: int) -> void:
	_pos_ms = maxi(0, ms)
	if player != null and player.playing:
		player.seek(float(_pos_ms) / 1000.0)


func position_ms() -> int:
	if player != null and player.playing:
		return int(player.get_playback_position() * 1000.0)
	return _pos_ms


func duration_ms() -> int:
	return int(player.stream.get_length() * 1000.0) if player != null and player.stream != null else 0


func is_playing() -> bool:
	return player != null and player.playing and not player.stream_paused


func set_speed(rate: float) -> void:
	if player != null:
		player.pitch_scale = clampf(rate, 0.5, 2.0)
