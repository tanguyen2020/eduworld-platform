class_name AnimDriver
extends Node
## One animation state machine for the local avatar (F062-AC-03): idle / walk / run / jump chosen from the body's
## velocity and floor contact only (no input or game logic). Builds the AnimationTree state machine in code; until
## the real avatar rig exists (F070) the clips are PLACEHOLDER bob animations of the greybox capsule (AGENTS R17).

const IDLE: StringName = &"idle"
const WALK: StringName = &"walk"
const RUN: StringName = &"run"
const JUMP: StringName = &"jump"
const STATES: Array[StringName] = [&"idle", &"walk", &"run", &"jump"]
const LIBRARY: StringName = &"avatar"
## PLACEHOLDER clips: [length s, bob height m, loop].
const PLACEHOLDER_CLIPS: Dictionary = {
	&"idle": [1.6, 0.02, true], &"walk": [0.6, 0.06, true], &"run": [0.4, 0.1, true], &"jump": [0.5, 0.15, false],
}

## Horizontal speeds (m/s) separating idle/walk and walk/run.
@export var walk_threshold: float = 0.2
@export var run_threshold: float = 3.5
@export var animation_player: AnimationPlayer = null
@export var animation_tree: AnimationTree = null
## Node animated by the placeholder clips (relative to the AnimationPlayer root).
@export var bob_track: String = "Bob:position:y"

var state: StringName = IDLE


func _ready() -> void:
	if animation_player != null and not animation_player.has_animation_library(LIBRARY):
		animation_player.add_animation_library(LIBRARY, build_placeholder_library(bob_track))
	if animation_tree != null and animation_player != null:
		animation_tree.anim_player = animation_tree.get_path_to(animation_player)
		animation_tree.tree_root = build_state_machine()
		animation_tree.active = true


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


## Called every physics frame by the controller; switches the state machine when the state changes.
func update(velocity: Vector3, on_floor: bool) -> StringName:
	var next: StringName = state_for(velocity, on_floor, walk_threshold, run_threshold)
	if next != state:
		state = next
		var machine_playback: AnimationNodeStateMachinePlayback = playback()
		if machine_playback != null:
			machine_playback.travel(next)
	return state


func playback() -> AnimationNodeStateMachinePlayback:
	if animation_tree == null or not animation_tree.active:
		return null
	return animation_tree.get("parameters/playback") as AnimationNodeStateMachinePlayback


## State machine with one node per state, every transition allowed (cross-fade), starting at idle.
static func build_state_machine() -> AnimationNodeStateMachine:
	var machine: AnimationNodeStateMachine = AnimationNodeStateMachine.new()
	var x: float = 0.0
	for s: StringName in STATES:
		var node: AnimationNodeAnimation = AnimationNodeAnimation.new()
		node.animation = StringName("%s/%s" % [LIBRARY, s])
		machine.add_node(s, node, Vector2(x, 0.0))
		x += 200.0
	for from: StringName in STATES:
		for to: StringName in STATES:
			if from != to:
				var t: AnimationNodeStateMachineTransition = AnimationNodeStateMachineTransition.new()
				t.xfade_time = 0.15
				machine.add_transition(from, to, t)
	var start: AnimationNodeStateMachineTransition = AnimationNodeStateMachineTransition.new()
	machine.add_transition(&"Start", IDLE, start)
	return machine


## PLACEHOLDER (R17): vertical bob of the greybox body per state.
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
	return library
