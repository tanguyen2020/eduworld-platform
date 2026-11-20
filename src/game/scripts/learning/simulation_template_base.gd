class_name SimulationTemplateBase
extends Control
## Contract of a packaged simulation template (F066-AC-21, SIMULATION_ENGINE §3, ADR-0052): the CMS only sets its
## parameters; the template draws the experiment and reports the student's actions (names of the descriptor actions).
## A template never decides a goal: the server checker does.

## An action of the template's action list (`place_mass`, …) with its arguments.
@warning_ignore("unused_signal")
signal action_performed(action: String, args: Dictionary)

var parameters: Dictionary = {}
var rng_seed: int = 0


## Applies the pinned parameters and seed; "" when the template can run them, else the reason.
func setup_template(params: Dictionary, run_seed: int) -> String:
	parameters = params
	rng_seed = run_seed
	return ""


## Back to the initial state for a new run.
func reset_run() -> void:
	pass


## The measurements shown to the student (display only; never sent as the result).
func measurements() -> Dictionary:
	return {}


## Lighter drawing for the Low preset and the Web export.
func set_lite(_lite: bool) -> void:
	pass


## The action names of the template descriptor (the action log accepts only these).
func actions() -> PackedStringArray:
	return PackedStringArray()
