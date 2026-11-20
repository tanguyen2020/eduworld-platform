class_name FakeVideoPlayback
extends VideoPlayback
## TEST-ONLY (AGENTS R17): scripted video playback for the video renderer tests — no decoding; `advance(ms)` moves the
## position while playing, `stall` freezes it, `finish()` ends the video. Lives under tests/ (never exported).

var loaded: Array[String] = []
var pos: int = 0
var playing: bool = false
var length_ms: int = 100000
var stall: bool = false


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


func advance(ms: int) -> void:
	if playing and not stall:
		pos = mini(pos + ms, length_ms)


func finish() -> void:
	pos = length_ms
	playing = false
	playback_finished.emit()
