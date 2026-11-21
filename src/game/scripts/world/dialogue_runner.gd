class_name DialogueRunner
extends RefCounted
## Runs an NPC dialogue graph served by `getNpcDialogue` (specs/schemas/npc-dialogue.schema.json, F071 US4) for the
## DialogueBubble UI. It walks the nodes and choices and turns every action into an intent for `action_handler`
## (`give_quest`/`complete_step` → quest API of F072, `open_activity`/`open_minigame` → activity host F063/F073,
## `open_ui`, `play_emote`): the runner never changes quest or reward state itself and shows only what the handler (the
## server) returns (F071-AC-19, ADR-0016). Conditions are evaluated only to choose what to show, with the facts the
## client knows (grade, level); a quest condition the client cannot know is treated as not met.

signal node_changed(node: Dictionary)
signal action_requested(action: Dictionary)
signal finished()

const BUSINESS_ACTIONS: PackedStringArray = ["give_quest", "complete_step", "open_activity", "open_minigame"]

## `(action: Dictionary) -> Variant` (may be a coroutine): performs the action through a service and returns its
## result (an ApiResult for the business actions). Null handler = actions are only emitted.
var action_handler: Callable = Callable()
var grade_level: int = 0
var player_level: int = 1
var graph: Dictionary = {}
var current: Dictionary = {}
var last_results: Array = []
var _nodes: Dictionary = {}


func load_graph(dialogue: Dictionary) -> bool:
	graph = dialogue
	_nodes.clear()
	for n: Variant in dialogue.get("nodes", []) as Array:
		if n is Dictionary and (n as Dictionary).get("node_id") is String:
			_nodes[String((n as Dictionary)["node_id"])] = n
	return not _nodes.is_empty()


## Starts at the first entry point whose condition holds, else `start_node_id`.
func start() -> Dictionary:
	var start_id: String = String(graph.get("start_node_id", ""))
	for ep: Variant in graph.get("entry_points", []) as Array:
		if ep is Dictionary and condition_met((ep as Dictionary).get("condition")):
			start_id = String((ep as Dictionary).get("node_id", start_id))
			break
	return await _enter(start_id)


## Choices of the current node whose (display) condition holds.
func visible_choices() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for c: Variant in current.get("choices", []) as Array:
		if c is Dictionary and condition_met((c as Dictionary).get("condition")):
			out.append(c as Dictionary)
	return out


## Picks a visible choice: runs its actions, then moves to its `next`.
func choose(index: int) -> Dictionary:
	var choices: Array[Dictionary] = visible_choices()
	if index < 0 or index >= choices.size():
		return current
	var choice: Dictionary = choices[index]
	await _run_actions(choice.get("actions", []))
	return await _enter(String(choice.get("next", "")))


## Moves past a node without choices (its `next`, or the end).
func advance() -> Dictionary:
	if not (current.get("choices", []) as Array).is_empty():
		return current
	return await _enter(String(current.get("next", "")))


func is_finished() -> bool:
	return current.is_empty()


## Text of a node or choice in `locale` (vi fallback, ADR-0056).
static func text_of(item: Dictionary, locale: String = "") -> String:
	return L10n.pick(item.get("text", {}) as Dictionary if item.get("text") is Dictionary else {}, locale)


## Display-only condition (`all`, `any`, `grade_level_in`, `min_level`; `quest_status` unknown to the client → false).
func condition_met(condition: Variant) -> bool:
	if not condition is Dictionary:
		return true
	var c: Dictionary = condition as Dictionary
	if c.has("all"):
		for sub: Variant in c["all"] as Array:
			if not condition_met(sub):
				return false
	if c.has("any"):
		var any_met: bool = false
		for sub: Variant in c["any"] as Array:
			any_met = any_met or condition_met(sub)
		if not any_met:
			return false
	if c.has("grade_level_in") and not grade_level in (c["grade_level_in"] as Array):
		return false
	if c.has("min_level") and player_level < int(c["min_level"]):
		return false
	if c.has("quest_status"):
		return false
	return true


func _enter(node_id: String) -> Dictionary:
	current = _nodes.get(node_id, {}) as Dictionary
	if current.is_empty():
		finished.emit()
		return current
	node_changed.emit(current)
	await _run_actions(current.get("actions", []))
	return current


func _run_actions(actions: Variant) -> void:
	if not actions is Array:
		return
	for a: Variant in actions as Array:
		if not a is Dictionary:
			continue
		var action: Dictionary = a as Dictionary
		if String(action.get("type", "")) == "end":
			continue
		action_requested.emit(action)
		if action_handler.is_valid():
			var result: Variant = await action_handler.call(action)
			last_results.append(result)
