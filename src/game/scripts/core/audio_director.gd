extends Node
## Autoload #10 `AudioDirector` (GODOT_ARCHITECTURE §4) — F060 frame: maps Settings volumes (0–1) onto the audio
## buses of `res://resources/audio/bus_layout.tres`. Music/zone playback arrives with F062.

## Settings volume key → bus name in bus_layout.tres.
const BUS_BY_KEY: Dictionary = {"music": "Music", "sfx": "SFX", "voice": "Voice"}


func _ready() -> void:
	# Settings is autoload #11 (after this one): bind once every autoload is in the tree.
	_bind_settings.call_deferred()


func _bind_settings() -> void:
	var settings: Node = get_node_or_null("/root/Settings")
	if settings == null:
		return
	if not settings.is_connected("volume_changed", apply_volume):
		settings.connect("volume_changed", apply_volume)
	for key: String in BUS_BY_KEY.keys():
		apply_volume(key, float(settings.call("get_volume", key)))


func apply_volume(key: String, value: float) -> void:
	if not BUS_BY_KEY.has(key):
		return
	var index: int = AudioServer.get_bus_index(String(BUS_BY_KEY[key]))
	if index < 0:
		return
	var linear: float = clampf(value, 0.0, 1.0)
	AudioServer.set_bus_mute(index, linear <= 0.0)
	AudioServer.set_bus_volume_db(index, linear_to_db(maxf(linear, 0.0001)))


func bus_volume(key: String) -> float:
	var index: int = AudioServer.get_bus_index(String(BUS_BY_KEY.get(key, "")))
	if index < 0:
		return -1.0
	return db_to_linear(AudioServer.get_bus_volume_db(index))
