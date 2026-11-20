class_name SimulationHost
extends Control
## Host of a packaged simulation template (F066-AC-21, AC-22, GODOT_ARCHITECTURE §6.2): loads the scene of
## `template_code`/`template_version` (build or verified PCK), applies the pinned parameters and seed, suspends the
## world while open, and records the student's actions in a SimulationActionLog with times relative to the run. It
## builds the `SimulationRunInput`; the result comes only from the server.

const SUSPEND_REASON: StringName = &"simulation"

var loader: SimulationTemplateLoader = SimulationTemplateLoader.new()
var template: SimulationTemplateBase = null
var action_log: SimulationActionLog = SimulationActionLog.new()
var template_code: String = ""
var template_version: int = 0
var parameters: Dictionary = {}
var run_seed: int = 0
var last_reason: String = ""
var _suspended: bool = false


## Opens the template of a render payload; false with `last_reason` (`update_required`, `pack_mismatch`, or the
## parameter reason of the template) when this client cannot run it.
func open(payload: Dictionary, lite: bool = false) -> bool:
	template_code = String(payload.get("template_code", ""))
	template_version = int(payload.get("template_version", 0))
	parameters = (payload.get("parameters", {}) as Dictionary) if payload.get("parameters") is Dictionary else {}
	run_seed = int(payload.get("seed", 0))
	var scene: PackedScene = loader.load_template(template_code, template_version)
	if scene == null:
		last_reason = loader.last_reason
		return false
	template = scene.instantiate() as SimulationTemplateBase
	if template == null:
		last_reason = SimulationTemplateLoader.REASON_MISSING
		return false
	(%Stage as Control).add_child(template)
	template.set_lite(lite)
	last_reason = template.setup_template(parameters, run_seed)
	if not last_reason.is_empty():
		return false
	template.action_performed.connect(_on_action)
	if not _suspended:
		QualityManager.suspend_world(SUSPEND_REASON)
		_suspended = true
	action_log = SimulationActionLog.new(template.actions())
	action_log.start()
	return true


func _on_action(action: String, args: Dictionary) -> void:
	action_log.record(action, args)


## A new run: the template back to its initial state, an empty log.
func new_run() -> void:
	if template != null:
		template.reset_run()
	action_log.start()


## "Thả": records the release and returns the run to send (the measurements are informative only).
func release() -> Dictionary:
	action_log.record("release", {})
	var shown: Dictionary = template.measurements() if template != null else {}
	return action_log.run_input(template_code, template_version, run_seed, parameters, shown)


func close() -> void:
	if _suspended:
		QualityManager.resume_world(SUSPEND_REASON)
		_suspended = false
