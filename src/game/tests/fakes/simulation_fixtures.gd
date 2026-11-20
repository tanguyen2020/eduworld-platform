class_name SimulationFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): `simulation` attempts for the F066 client tests, derived from the contract fixture of
## `Attempt` with the render payload of `simulation.schema.json#/$defs/render` (physics.lever v1, goals without check).


static func payload(template_code: String = "physics.lever", max_runs: Variant = 3, runs_used: int = 0) -> Dictionary:
	return {
		"simulation_kind": "physics", "template_code": template_code, "template_version": 1,
		"parameters": {"arm_length_m": 1, "slots_per_side": 4, "mass_1_kg": 2, "mass_2_kg": 1},
		"goals": [{"goal_id": "balance", "description": {"vi": "Làm cho đòn bẩy cân bằng"}}],
		"steps": [{"step_id": "place", "instruction": {"vi": "Đặt hai vật lên đòn bẩy"}, "safety_required": false}],
		"interaction": {"max_runs": max_runs, "show_measurements": true},
		"seed": 4242, "runs_used": runs_used, "questions": [{"position": 1}],
	}


static func attempt(template_code: String = "physics.lever", max_runs: Variant = 3, runs_used: int = 0) -> Dictionary:
	var a: Dictionary = LearningFixtures.fresh_attempt()
	a["activity_type"] = "simulation"
	a["items"] = [LearningFixtures.item_of("single_choice").duplicate(true)]
	var render: Dictionary = (a["render"] as Dictionary).duplicate(true)
	render["activity_type"] = "simulation"
	render["render_payload"] = payload(template_code, max_runs, runs_used)
	render["media"] = []
	a["render"] = render
	return a


static func result(goal_met: bool, runs_used: int, max_runs: Variant = 3) -> Dictionary:
	return {"run_id": "0f660000-0000-4000-8000-0000000000aa", "verified": true, "goal_met": goal_met, "verification_status": "verified",
		"goals_met": [{"goal_id": "balance", "met": goal_met}], "runs_used": runs_used, "max_runs": max_runs,
		"verified_outcome": {"balanced": 1 if goal_met else 0}}
