extends GdUnitTestSuite
## F066-AC-24 (TC-SIM-003; T037): LeverModel gives exactly the result of every shared vector of physics.lever v1
## (specs/examples/simulation/physics.lever/v1/vectors.json — the file the Go checker test reads too).

const VECTORS: String = "../../specs/examples/simulation/physics.lever/v1/vectors.json"


func test_shared_vectors() -> void:
	var path: String = ProjectSettings.globalize_path("res://").path_join(VECTORS)
	assert_bool(FileAccess.file_exists(path)).override_failure_message("missing " + path).is_true()
	var doc: Dictionary = JSON.parse_string(FileAccess.get_file_as_string(path)) as Dictionary
	assert_str(String(doc["template_code"])).is_equal("physics.lever")
	assert_float(float(doc["g"])).is_equal(LeverModel.G)
	var vectors: Array = doc["vectors"] as Array
	assert_int(vectors.size()).is_greater_equal(20)
	for v: Variant in vectors:
		var vec: Dictionary = v as Dictionary
		var got: Dictionary = LeverModel.replay(vec["parameters"] as Dictionary, vec["action_log"] as Array)
		var want: Dictionary = vec["expect"] as Dictionary
		var name: String = String(vec["name"])
		assert_bool(bool(got["valid"])).override_failure_message(name + " valid").is_equal(bool(want["valid"]))
		if not bool(want["valid"]):
			assert_str(String(got["reject_code"])).override_failure_message(name).is_equal(String(want["reject_code"]))
			continue
		var outcome: Dictionary = got["outcome"] as Dictionary
		for k: Variant in (want["outcome"] as Dictionary).keys():
			assert_float(float(outcome[k])).override_failure_message("%s %s" % [name, k]).is_equal_approx(float(want["outcome"][k]), 1e-9)
