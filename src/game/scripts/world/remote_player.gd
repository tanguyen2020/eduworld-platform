class_name RemotePlayer
extends Node3D
## Another student in the hub (GODOT_ARCHITECTURE §3 `RemotePlayer.tscn`; F092-AC-13, AC-15, AC-16, AC-21): the avatar
## built by AvatarBuilder from `PublicPlayer.avatar_config`, the nickname above the head and the emotes it plays.
## Only nickname and avatar are shown — no real name, school or class (R16). Position, yaw and animation come from
## the Interpolator (display only, R5). Pooled by RemotePlayerSync; `fade_out()` hides it in 0.3 s before reuse.
## Level of detail (PERFORMANCE_BUDGET §5, §13): `full` draws the avatar and animates every frame; beyond 30 m the
## animation advances every few frames; `impostor` replaces the avatar by a light stand-in.

signal fade_completed(eid: int)

## F092-AC-13 / GAME_NETWORKING §5.2: fade-out of a player who left.
const FADE_SEC: float = 0.3
## Frames between two animation updates of a far player (> ANIM_LOD_DISTANCE).
const FAR_ANIM_EVERY: int = 4

enum Lod { FULL, FAR, IMPOSTOR }

var eid: int = -1
var player: PublicPlayerModel = null
var lod: Lod = Lod.FULL
var fading: bool = false
var _fade_left: float = 0.0
var _anim_frame: int = 0
var _far_delta: float = 0.0

@onready var avatar: AvatarBuilder = $AvatarRig as AvatarBuilder
@onready var anim_driver: AnimDriver = $AnimDriver as AnimDriver
@onready var nickname_label: Label3D = $Nickname as Label3D
@onready var impostor: MeshInstance3D = $Impostor as MeshInstance3D
@onready var animation_tree: AnimationTree = $AnimationTree as AnimationTree


func _ready() -> void:
	# A nickname is never a translation key.
	nickname_label.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	# Font, size and colours from the generated Theme/tokens (R32), never literals in the scene.
	var theme: Theme = load(UiTokens.BASE_THEME_PATH) as Theme
	if theme != null and theme.default_font != null:
		nickname_label.font = theme.default_font
	nickname_label.font_size = UiTokens.SIZE_FONT_36
	nickname_label.outline_size = UiTokens.SIZE_BORDER_4
	nickname_label.modulate = UiTokens.COLOR_TEXT_ON_ACCENT
	nickname_label.outline_modulate = UiTokens.COLOR_SURFACE_HUD
	impostor.visible = false


## Shows `who` (pool reuse): avatar, nickname, visible, opaque.
func setup(entity_id: int, who: PublicPlayerModel, catalog: CharacterCatalogModel) -> void:
	eid = entity_id
	player = who
	name = "Remote_%d" % entity_id
	fading = false
	_fade_left = 0.0
	_set_transparency(0.0)
	nickname_label.text = who.nickname
	apply_avatar(catalog)
	visible = true


## `hub.player.appearance_changed` / first build: the avatar from the public config (default look without one).
func apply_avatar(catalog: CharacterCatalogModel) -> void:
	var config: AvatarConfigModel = player.avatar() if player != null else null
	if config != null:
		avatar.apply(config, catalog)


## Interpolated state of this frame (`Interpolator.sample_at`).
func apply_state(state: Dictionary) -> void:
	if state.is_empty() or fading:
		return
	position = state["p"] as Vector3
	avatar.rotation.y = float(state["yaw"])
	if lod != Lod.IMPOSTOR:
		anim_driver.set_locomotion(String(state["anim"]))


func play_emote(emote_id: String) -> void:
	if lod == Lod.IMPOSTOR:
		return
	var visual: ItemVisual = ItemVisual.load_item(emote_id)
	anim_driver.play_emote(emote_id, visual.emote_clip if visual != null else null)


func set_lod(next: Lod) -> void:
	if next == lod:
		return
	lod = next
	var full_avatar: bool = next != Lod.IMPOSTOR
	avatar.visible = full_avatar
	impostor.visible = not full_avatar
	animation_tree.active = full_avatar
	animation_tree.callback_mode_process = AnimationMixer.ANIMATION_CALLBACK_MODE_PROCESS_MANUAL if next == Lod.FAR \
			else AnimationMixer.ANIMATION_CALLBACK_MODE_PROCESS_IDLE


func fade_out() -> void:
	fading = true
	_fade_left = FADE_SEC


func _process(delta: float) -> void:
	if lod == Lod.FAR and animation_tree.active:
		_far_delta += delta
		_anim_frame += 1
		if _anim_frame >= FAR_ANIM_EVERY:
			animation_tree.advance(_far_delta)
			_anim_frame = 0
			_far_delta = 0.0
	if not fading:
		return
	_fade_left -= delta
	_set_transparency(clampf(1.0 - _fade_left / FADE_SEC, 0.0, 1.0))
	if _fade_left <= 0.0:
		fading = false
		visible = false
		fade_completed.emit(eid)


func _set_transparency(value: float) -> void:
	for node: Node in find_children("*", "GeometryInstance3D", true, false):
		(node as GeometryInstance3D).transparency = value
	nickname_label.transparency = value
