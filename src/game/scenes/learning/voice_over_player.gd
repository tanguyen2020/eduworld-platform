extends PanelContainer
## Voice-over and captions of the current item (STUDENT_GAME_UX_LEARNING §7, GODOT_ARCHITECTURE §6.2, F063-AC-04).
## `g1_3` plays the recorded stem audio automatically when the item appears; "🔊 Đọc to" (key `R`) replays it. When
## the device or the Voice bus is muted — or there is no audio — the caption (stem text) and a muted icon are always
## shown so the student can work without sound. Audio clips come from MediaLoader (RAM only).
## F097 (F096-3): when the renderer draws the stem on screen (`stem_on_screen`), that stem IS the caption — this panel
## never repeats it; it only plays the clip (auto-play, key `R`, the "Đọc to" button of the question card).

const BUS: StringName = &"Voice"
## Speaker icons of design/icons/game (F096-AC-15: no emoji, the Web build has no emoji font).
const ICON_SOUND: Texture2D = preload("res://resources/ui/icons/sound_on.svg")
const ICON_MUTED: Texture2D = preload("res://resources/ui/icons/sound_off.svg")

var stream: AudioStream = null
var caption: String = ""
## g1_3 (or the student's auto-read setting): the caption is always shown with the stem.
var always_caption: bool = false
## The renderer shows the stem text itself (question card): no caption panel at all (F096-3).
var stem_on_screen: bool = false

@onready var _icon: TextureRect = %Icon
@onready var _caption: Label = %Caption
@onready var _replay: Button = %Replay
@onready var _player: AudioStreamPlayer = %Player


func _ready() -> void:
	_player.bus = BUS
	_icon.custom_minimum_size = Vector2.ONE * UiTokens.COMPONENT_ICON_SIZE
	_replay.pressed.connect(replay)
	visible = false


## Shows the item's caption and plays `audio` when `auto_play` (g1_3). `audio` may be null (no recording).
func present(caption_text: String, audio: AudioStream, auto_play: bool) -> void:
	caption = caption_text
	stream = audio
	_player.stop()
	_player.stream = audio
	_caption.text = caption_text
	_render()
	if auto_play and audio != null and not is_muted():
		_player.play()


func replay() -> void:
	if stream != null and not is_muted():
		_player.stop()
		_player.play()
	_render()


func stop() -> void:
	_player.stop()


func is_playing() -> bool:
	return _player.playing


func is_caption_visible() -> bool:
	return visible and _caption.visible


## "sound_on" or "sound_off": the speaker icon shown beside the caption.
func icon_name() -> String:
	return "sound_off" if _icon.texture == ICON_MUTED else "sound_on"


## Muted when the Voice or Master bus is muted or silent.
func is_muted() -> bool:
	for bus_name: StringName in [BUS, &"Master"]:
		var index: int = AudioServer.get_bus_index(bus_name)
		if index >= 0 and (AudioServer.is_bus_mute(index) or AudioServer.get_bus_volume_db(index) <= -79.0):
			return true
	return false


func _render() -> void:
	var muted: bool = is_muted()
	var show_caption: bool = (always_caption or muted or stream == null) and not stem_on_screen
	visible = not caption.is_empty() and not stem_on_screen and (show_caption or stream != null)
	_caption.visible = show_caption
	_icon.texture = ICON_MUTED if muted or stream == null else ICON_SOUND
	_replay.visible = stream != null
