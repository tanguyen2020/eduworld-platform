class_name FakeAudioPlayback
extends AudioPlayback
## TEST-ONLY (AGENTS R17): scripted audio playback for the audio renderer tests — no decoding; `advance(ms)` moves the
## position while playing, `finish()` ends the play. Lives under tests/ (never exported).

var loaded: Array[String] = []
var pos: int = 0
var playing: bool = false
var length_ms: int = 60000
var speed: float = 1.0


func load_file(path: String) -> bool:
	loaded.append(path)
	return not path.is_empty()


func play() -> void:
	playing = true


func pause() -> void:
	playing = false


func seek(ms: int) -> void:
	pos = maxi(0, ms)


func position_ms() -> int:
	return pos


func duration_ms() -> int:
	return length_ms


func is_playing() -> bool:
	return playing


func set_speed(rate: float) -> void:
	speed = rate


func advance(ms: int) -> void:
	if playing:
		pos = mini(pos + ms, length_ms)


func finish() -> void:
	pos = length_ms
	playing = false
	playback_finished.emit()
