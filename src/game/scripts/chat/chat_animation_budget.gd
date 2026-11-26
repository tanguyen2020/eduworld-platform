class_name ChatAnimationBudget
extends RefCounted
## At most 3 animated stickers / GIFs play at once, only while they are inside the visible part of the list
## (F119-AC-17, TC-PERF-019; owner value STUDENT_GAME_UX_SOCIAL §7 "Hiệu năng máy tầm trung"). Bubbles ask for a slot
## when they become visible and give it back when they scroll out or are recycled; the others show a still picture.

const MAX_PLAYING: int = 3

var _playing: Array[String] = []


## True when `key` may play now (it already plays, or a slot is free and it is visible).
func request(key: String, is_visible: bool) -> bool:
	if not is_visible:
		release(key)
		return false
	if key in _playing:
		return true
	if _playing.size() >= MAX_PLAYING:
		return false
	_playing.append(key)
	return true


func release(key: String) -> void:
	_playing.erase(key)


func playing_count() -> int:
	return _playing.size()


func is_playing(key: String) -> bool:
	return key in _playing


func clear() -> void:
	_playing.clear()
