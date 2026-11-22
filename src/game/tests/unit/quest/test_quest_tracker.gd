extends GdUnitTestSuite
## F072 Quest Tracker (T025): shows the tracked active quest and its current step with the server count, hides itself
## without one and follows `quest.progress.updated` through QuestService (no REST call).

const TRACKER: PackedScene = preload("res://scenes/quest/QuestTracker.tscn")
const QUEST: String = "0193a5f4-6c1e-7b2a-9d1f-000000072001"


func _service() -> QuestService:
	var s: QuestService = QuestService.new()
	var q: QuestModel = QuestModel.from_dict({"quest_id": QUEST, "status": "active", "name": {"vi": "Phụ giúp phiên chợ", "en": "Market Helper"},
		"steps": [{"step_id": "baskets", "order": 1, "step_type": "collect", "title": {"vi": "Nhặt giỏ", "en": "Baskets"},
			"interactable_ids": ["quest.math.apple_basket_1", "quest.math.apple_basket_2"], "count": 2}],
		"progress": {"quest_progress_id": "p", "status": "active", "step_states": {"baskets": {"status": "active", "count": 0, "target": 2}}}})
	s.quests.append(q)
	return s


func test_tracker_follows_the_tracked_quest() -> void:
	var service: QuestService = _service()
	var tracker: Control = TRACKER.instantiate() as Control
	tracker.set("service", service)
	add_child(tracker)
	await get_tree().process_frame
	assert_bool(tracker.visible).is_false()
	service.track(QUEST)
	assert_bool(tracker.visible).is_true()
	var step: Label = tracker.get_node("%Step") as Label
	assert_str(step.text).contains("0/2")
	service.call("_on_message", "quest.progress.updated", {"quest_id": QUEST, "status": "active", "step_id": "baskets", "step_status": "active", "count": 1})
	assert_str(step.text).contains("1/2")
	service.call("_on_message", "quest.progress.updated", {"quest_id": QUEST, "status": "completed", "step_id": "baskets", "step_status": "completed", "count": 2})
	assert_bool(tracker.visible).is_false()
	tracker.free()


func test_model_matches_world_interactions_to_active_steps() -> void:
	var q: QuestModel = _service().quests[0]
	assert_str(String(q.step_for("learning_zone.math", "quest.math.apple_basket_2", "").get("step_id", ""))).is_equal("baskets")
	assert_bool(q.step_for("learning_zone.math", "quest.math.fruit_stall", "").is_empty()).is_true()
	q.apply_update({"quest_id": QUEST, "status": "active", "step_id": "baskets", "step_status": "completed"})
	assert_bool(q.step_for("learning_zone.math", "quest.math.apple_basket_2", "").is_empty()).is_true()
