class_name AnimDriver
extends Node
## One animation state machine for an avatar (F062-AC-03, CHARACTER_SYSTEM §4): idle / walk / run / jump chosen from
## the body's velocity and floor contact only (no input or game logic), plus the one-shot Emote state (F070-AC-20):
## `play_emote()` plays the clip once, then the driver returns to Locomotion when the clip ends or the body moves.
## Builds the AnimationTree state machine in code. On the F068 `kid_base` rig (KidSkeleton, ADR-0103 "3D Animated
## Cartoon") the clips are the baked idle breathing / walk / bouncy run / jump with squash & stretch
## (tools/art/character_anim.gd), and each state shows a facial expression on the AvatarBuilder (EXPRESSION_FOR_STATE;
## an emote shows its ItemVisual.expression, default happy). A body without that rig keeps the PLACEHOLDER bob clips
## (AGENTS R17). Used by the Player, remote players and the avatar previews (S02, Wardrobe).

signal emote_started(emote_id: String)
signal emote_finished(emote_id: String)

const IDLE: StringName = &"idle"
const WALK: StringName = &"walk"
const RUN: StringName = &"run"
const JUMP: StringName = &"jump"
const EMOTE: StringName = &"emote"
## Locomotion states (the Emote node is added next to them).
const STATES: Array[StringName] = [&"idle", &"walk", &"run", &"jump"]
const LIBRARY: StringName = &"avatar"
## F068: facial expression per state (display only) and of an emote without its own.
const EXPRESSION_FOR_STATE: Dictionary = {
	&"idle": "neutral", &"walk": "neutral", &"run": "trying", &"jump": "surprised", &"emote": "happy",
}
const EMOTE_EXPRESSION: String = "happy"
## PLACEHOLDER clips: [length s, bob height m, loop].
const PLACEHOLDER_CLIPS: Dictionary = {
	&"idle": [1.6, 0.02, true], &"walk": [0.6, 0.06, true], &"run": [0.4, 0.1, true], &"jump": [0.5, 0.15, false],
}
## PLACEHOLDER emote clip (R17): three hops of the greybox body until the OQ-16 emote set ships.
const PLACEHOLDER_EMOTE_LENGTH: float = 1.2
const PLACEHOLDER_EMOTE_HOPS: int = 3
const PLACEHOLDER_EMOTE_HEIGHT: float = 0.12

## Horizontal speeds (m/s) separating idle/walk and walk/run.
@export var walk_threshold: float = 0.2
@export var run_threshold: float = 3.5
@export var animation_player: AnimationPlayer = null
@export var animation_tree: AnimationTree = null
## Node animated by the placeholder clips (relative to the AnimationPlayer root).
@export var bob_track: String = "Bob:position:y"

var state: StringName = IDLE
## Emote being played ("" when in Locomotion) and the time left of its clip.
var emote_id: String = ""
var emote_time_left: float = 0.0
## Avatar whose face follows the state (the AnimationPlayer root when it is an AvatarBuilder).
var face: AvatarBuilder = null
## True when the clips are the baked kid_base clips (not the placeholders).
var rig_clips: bool = false


func _ready() -> void:
	if animation_player != null:
		face = animation_player.get_node_or_null(animation_player.root_node) as AvatarBuilder
	if animation_player != null and not animation_player.has_animation_library(LIBRARY):
		var rig: Node = animation_player.get_node_or_null(animation_player.root_node)
		var library: AnimationLibrary = null
		if rig != null and rig.get_node_or_null(KidSkeleton.SKELETON_PATH) is Skeleton3D:
			library = build_rig_library(bob_track)
		rig_clips = library != null
		animation_player.add_animation_library(LIBRARY, library if library != null else build_placeholder_library(bob_track))
	if animation_tree != null and animation_player != null:
		animation_tree.anim_player = animation_tree.get_path_to(animation_player)
		animation_tree.tree_root = build_state_machine()
		animation_tree.active = true


func _process(delta: float) -> void:
	advance_emote(delta)


## F062-AC-03: pure mapping velocity → state.
static func state_for(velocity: Vector3, on_floor: bool, walk_speed: float, run_speed: float) -> StringName:
	if not on_floor:
		return JUMP
	var speed: float = Vector2(velocity.x, velocity.z).length()
	if speed >= run_speed:
		return RUN
	if speed >= walk_speed:
		return WALK
	return IDLE


## Called every physics frame by the controller; switches the state machine when the state changes. An emote keeps
## playing while the body stands still and is interrupted as soon as it moves or leaves the floor.
func update(velocity: Vector3, on_floor: bool) -> StringName:
	var next: StringName = state_for(velocity, on_floor, walk_threshold, run_threshold)
	if state == EMOTE:
		if next == IDLE:
			return state
		_end_emote(next)
		return state
	if next != state:
		state = next
		var machine_playback: AnimationNodeStateMachinePlayback = playback()
		if machine_playback != null:
			machine_playback.travel(next)
		_show_expression(String(EXPRESSION_FOR_STATE.get(next, "neutral")))
	return state


## F092: a remote player's locomotion state as sent by the server (`idle`, `walk`, `run`, `jump`; anything else →
## idle). An emote keeps playing until it ends or the player moves, as for the local player.
func set_locomotion(anim_name: String) -> StringName:
	var next: StringName = StringName(anim_name) if StringName(anim_name) in STATES else IDLE
	if state == EMOTE:
		if next != IDLE:
			_end_emote(next)
		return state
	if next != state:
		state = next
		var machine_playback: AnimationNodeStateMachinePlayback = playback()
		if machine_playback != null:
			machine_playback.travel(next)
		_show_expression(String(EXPRESSION_FOR_STATE.get(next, "neutral")))
	return state


## F070-AC-20: plays `clip` (or the placeholder) once for `id`, then Locomotion. False for an empty id.
func play_emote(id: String, clip: Animation = null) -> bool:
	if id.is_empty():
		return false
	var anim: Animation = clip.duplicate() as Animation if clip != null else build_placeholder_emote(bob_track)
	anim.loop_mode = Animation.LOOP_NONE
	if animation_player != null and animation_player.has_animation_library(LIBRARY):
		var library: AnimationLibrary = animation_player.get_animation_library(LIBRARY)
		if library.has_animation(EMOTE):
			library.remove_animation(EMOTE)
		library.add_animation(EMOTE, anim)
	emote_id = id
	emote_time_left = maxf(anim.length, 0.01)
	state = EMOTE
	var machine_playback: AnimationNodeStateMachinePlayback = playback()
	if machine_playback != null:
		machine_playback.start(EMOTE, true)
	var visual: ItemVisual = ItemVisual.load_item(id)
	_show_expression(visual.expression if visual != null and not visual.expression.is_empty() else EMOTE_EXPRESSION)
	emote_started.emit(id)
	return true


## Advances the emote clock (called from `_process`; tests call it directly); the end returns to Locomotion.
func advance_emote(delta: float) -> void:
	if state != EMOTE:
		return
	emote_time_left -= delta
	if emote_time_left <= 0.0:
		_end_emote(IDLE)


func playback() -> AnimationNodeStateMachinePlayback:
	if animation_tree == null or not animation_tree.active:
		return null
	return animation_tree.get("parameters/playback") as AnimationNodeStateMachinePlayback


func _end_emote(next: StringName) -> void:
	var finished: String = emote_id
	emote_id = ""
	emote_time_left = 0.0
	state = next
	var machine_playback: AnimationNodeStateMachinePlayback = playback()
	if machine_playback != null:
		machine_playback.travel(next)
	_show_expression(String(EXPRESSION_FOR_STATE.get(next, "neutral")))
	emote_finished.emit(finished)


func _show_expression(expression: String) -> void:
	if face != null:
		face.set_expression(expression)


## State machine with one node per locomotion state plus Emote, every transition allowed (cross-fade), from idle.
static func build_state_machine() -> AnimationNodeStateMachine:
	var machine: AnimationNodeStateMachine = AnimationNodeStateMachine.new()
	var x: float = 0.0
	var nodes: Array[StringName] = STATES.duplicate()
	nodes.append(EMOTE)
	for s: StringName in nodes:
		var node: AnimationNodeAnimation = AnimationNodeAnimation.new()
		node.animation = StringName("%s/%s" % [LIBRARY, s])
		machine.add_node(s, node, Vector2(x, 0.0))
		x += 200.0
	for from: StringName in nodes:
		for to: StringName in nodes:
			if from != to:
				var t: AnimationNodeStateMachineTransition = AnimationNodeStateMachineTransition.new()
				t.xfade_time = 0.15
				machine.add_transition(from, to, t)
	var start: AnimationNodeStateMachineTransition = AnimationNodeStateMachineTransition.new()
	machine.add_transition(&"Start", IDLE, start)
	return machine


## F068: the baked kid_base locomotion clips (KidSkeleton.clip_path) plus the placeholder emote slot; null when a
## clip is missing on this client (the caller then keeps the placeholders).
static func build_rig_library(track_path: String) -> AnimationLibrary:
	var library: AnimationLibrary = AnimationLibrary.new()
	for s: StringName in STATES:
		var path: String = KidSkeleton.clip_path(String(s))
		if not ResourceLoader.exists(path):
			return null
		library.add_animation(s, load(path) as Animation)
	library.add_animation(EMOTE, build_placeholder_emote(track_path))
	return library


## PLACEHOLDER (R17): vertical bob of the greybox body per state, plus the placeholder emote.
static func build_placeholder_library(track_path: String) -> AnimationLibrary:
	var library: AnimationLibrary = AnimationLibrary.new()
	for s: StringName in STATES:
		var spec: Array = PLACEHOLDER_CLIPS[s] as Array
		var length: float = float(spec[0])
		var anim: Animation = Animation.new()
		anim.length = length
		anim.loop_mode = Animation.LOOP_LINEAR if bool(spec[2]) else Animation.LOOP_NONE
		var track: int = anim.add_track(Animation.TYPE_VALUE)
		anim.track_set_path(track, NodePath(track_path))
		anim.track_insert_key(track, 0.0, 0.0)
		anim.track_insert_key(track, length / 2.0, float(spec[1]))
		anim.track_insert_key(track, length, 0.0)
		library.add_animation(s, anim)
	library.add_animation(EMOTE, build_placeholder_emote(track_path))
	return library


## PLACEHOLDER (R17): one-shot hops used for every emote until its clip exists (`ItemVisual.emote_clip`).
static func build_placeholder_emote(track_path: String) -> Animation:
	var anim: Animation = Animation.new()
	anim.length = PLACEHOLDER_EMOTE_LENGTH
	anim.loop_mode = Animation.LOOP_NONE
	var track: int = anim.add_track(Animation.TYPE_VALUE)
	anim.track_set_path(track, NodePath(track_path))
	var hop: float = PLACEHOLDER_EMOTE_LENGTH / float(PLACEHOLDER_EMOTE_HOPS)
	for i: int in PLACEHOLDER_EMOTE_HOPS:
		anim.track_insert_key(track, hop * float(i), 0.0)
		anim.track_insert_key(track, hop * (float(i) + 0.5), PLACEHOLDER_EMOTE_HEIGHT)
	anim.track_insert_key(track, PLACEHOLDER_EMOTE_LENGTH, 0.0)
	return anim
