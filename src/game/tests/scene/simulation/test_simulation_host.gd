extends GdUnitTestSuite
## F066-AC-21, F066-AC-23 (TC-ACT-004, TC-SIM-002; T039): the renderer loads the packaged physics.lever v1 template,
## applies the pinned parameters and seed and suspends the world; "Thả" sends the run with the action log; goals are
## ticked only from the server result ("Đạt mục tiêu!"); `SIMULATION_LOG_INVALID` asks to redo, the run limit locks the
## runs and opens the conclusion question; a missing template or a pack with another sha256 → update required, no crash.

const SCENE: String = "res://scenes/learning/activity_renderers/simulation/SimulationRenderer.tscn"

var r: SimulationRenderer = null
var runs: Array[Dictionary] = []
var missing: Array[String] = []


func _open(data: Dictionary) -> void:
	runs.clear()
	missing.clear()
	r = (load(SCENE) as PackedScene).instantiate() as SimulationRenderer
	add_child(r)
	r.run_submitted.connect(func(run: Dictionary) -> void: runs.append(run))
	r.template_load_failed.connect(func(reason: String) -> void: missing.append(reason))
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = "vi"
	r.setup(AttemptModel.from_dict(data), ctx)


func after_test() -> void:
	if r != null:
		r.teardown()
		r.free()
		r = null
	QualityManager.suspend_reasons.clear()


func _lever() -> SimulationTemplateBase:
	return r.host.template


func test_run_is_sent_and_goals_follow_the_server() -> void:
	_open(SimulationFixtures.attempt())
	assert_object(_lever()).is_not_null()
	assert_bool(QualityManager.suspend_reasons.has(SimulationHost.SUSPEND_REASON)).is_true()
	assert_int(r.host.run_seed).is_equal(4242)
	var lever: SimulationTemplateBase = _lever()
	lever.call("pick_mass", 0)
	lever.call("pick_slot", "left", 2)
	lever.call("pick_mass", 1)
	lever.call("pick_slot", "right", 4)
	assert_float(float(lever.measurements()["balanced"])).is_equal(1.0)
	var goal: CheckBox = (r.get_node("%Goals") as Control).get_child(0) as CheckBox
	assert_bool(goal.button_pressed).is_false()
	r.release()
	assert_int(runs.size()).is_equal(1)
	var run: Dictionary = runs[0]
	assert_str(String(run["template_code"])).is_equal("physics.lever")
	assert_int(int(run["seed"])).is_equal(4242)
	var sent: Array = run["action_log"] as Array
	assert_int(sent.size()).is_equal(3)
	assert_str(String(sent[2]["action"])).is_equal("release")
	assert_bool(goal.button_pressed).override_failure_message("ticked before the server answered").is_false()
	r.show_run_result(SimulationFixtures.result(true, 1))
	assert_bool(goal.button_pressed).is_true()
	assert_str((r.get_node("%Status") as Label).text).is_equal("ui.simulation.goal_reached")
	assert_str((r.get_node("%Runs") as Label).text).is_equal("1 / 3")


func test_a_network_loss_resends_the_same_run() -> void:
	_open(SimulationFixtures.attempt())
	r.release()
	r.run_refused("NETWORK_ERROR")
	r.release()
	assert_int(runs.size()).is_equal(2)
	assert_dict(runs[1]).is_equal(runs[0])


func test_log_invalid_and_run_limit() -> void:
	_open(SimulationFixtures.attempt("physics.lever", 2, 0))
	r.release()
	r.run_refused(LearningService.CODE_LOG_INVALID)
	assert_str((r.get_node("%Status") as Label).text).is_equal("ui.simulation.log_invalid")
	assert_bool((r.get_node("%Retry") as Button).disabled).is_false()
	r.retry()
	assert_int(r.host.action_log.to_payload().size()).is_equal(0)
	r.release()
	r.run_refused(LearningService.CODE_RUN_LIMIT)
	assert_bool((r.get_node("%Release") as Button).disabled).is_true()
	assert_bool((r.get_node("%Overlay") as Control).visible).is_true()
	assert_int(r.current_position()).is_equal(1)
	assert_bool(r.wants_action_bar()).is_true()


func test_missing_template_or_bad_pack_is_update_required() -> void:
	_open(SimulationFixtures.attempt("chemistry.boiling"))
	assert_array(missing).contains_exactly([SimulationTemplateLoader.REASON_MISSING])
	assert_bool((r.get_node("%Missing") as Control).visible).is_true()
	var loader: SimulationTemplateLoader = SimulationTemplateLoader.new()
	var pck: String = "user://f066_fake.pck"
	var f: FileAccess = FileAccess.open(pck, FileAccess.WRITE)
	f.store_string("not a pack")
	f.close()
	loader.packs["physics.lever@2"] = {"pck": pck, "sha256": "00", "scene": "res://x.tscn"}
	assert_object(loader.load_template("physics.lever", 2)).is_null()
	assert_str(loader.last_reason).is_equal(SimulationTemplateLoader.REASON_BAD_PACK)
	assert_object(loader.load_template("physics.lever", 3)).is_null()
	assert_str(loader.last_reason).is_equal(SimulationTemplateLoader.REASON_MISSING)
	DirAccess.remove_absolute(pck)
