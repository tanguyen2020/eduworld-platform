class_name DynamicResolution
extends RefCounted
## Render-scale controller of `QualityManager` (CONFIG_DEFAULTS `dynamic_resolution`, PERFORMANCE_BUDGET §6,
## F062-AC-16): averages the frame time over 2 s windows; above the target by 10 % → scale −0.05; below the target
## by 20 % for 10 s → scale +0.05; always inside the preset range. It scales the 3D viewport only (never the UI) and
## never changes the preset: at the minimum scale and still slow for 30 s it only raises `lower_preset_suggested`.

signal scale_changed(value: float)
signal lower_preset_suggested()

const STEP: float = 0.05
const WINDOW_SEC: float = 2.0
const SLOW_RATIO: float = 1.10
const FAST_RATIO: float = 0.80
const FAST_HOLD_SEC: float = 10.0
const SUGGEST_AFTER_SEC: float = 30.0

var scale: float = 1.0
var scale_min: float = 1.0
var scale_max: float = 1.0
var target_ms: float = 33.333
var _window_sec: float = 0.0
var _window_ms: float = 0.0
var _window_frames: int = 0
var _fast_sec: float = 0.0
var _slow_at_min_sec: float = 0.0
var _suggested: bool = false


## Resets the controller for a preset range and frame target.
func configure(base_scale: float, min_scale: float, max_scale: float, frame_target_ms: float) -> void:
	scale_min = minf(min_scale, max_scale)
	scale_max = maxf(min_scale, max_scale)
	scale = clampf(base_scale, scale_min, scale_max)
	target_ms = frame_target_ms
	_window_sec = 0.0
	_window_ms = 0.0
	_window_frames = 0
	_fast_sec = 0.0
	_slow_at_min_sec = 0.0
	_suggested = false


## Feeds one frame (`frame_ms` = its duration). Returns the (possibly new) scale.
func push_frame(frame_ms: float) -> float:
	_window_ms += frame_ms
	_window_frames += 1
	_window_sec += frame_ms / 1000.0
	if _window_sec < WINDOW_SEC:
		return scale
	var avg: float = _window_ms / float(maxi(_window_frames, 1))
	var elapsed: float = _window_sec
	_window_sec = 0.0
	_window_ms = 0.0
	_window_frames = 0
	if avg > target_ms * SLOW_RATIO:
		_fast_sec = 0.0
		if scale > scale_min + 0.0001:
			_set_scale(scale - STEP)
			_slow_at_min_sec = 0.0
		else:
			_slow_at_min_sec += elapsed
			if _slow_at_min_sec >= SUGGEST_AFTER_SEC and not _suggested:
				_suggested = true
				lower_preset_suggested.emit()
	elif avg < target_ms * FAST_RATIO:
		_slow_at_min_sec = 0.0
		_fast_sec += elapsed
		if _fast_sec >= FAST_HOLD_SEC and scale < scale_max - 0.0001:
			_fast_sec = 0.0
			_set_scale(scale + STEP)
	else:
		_fast_sec = 0.0
		_slow_at_min_sec = 0.0
	return scale


func _set_scale(value: float) -> void:
	var next: float = clampf(snappedf(value, 0.01), scale_min, scale_max)
	if is_equal_approx(next, scale):
		return
	scale = next
	scale_changed.emit(scale)
