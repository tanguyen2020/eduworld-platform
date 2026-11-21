extends GdUnitTestSuite
## F064-AC-20, F064-AC-21 (TC-LMS-002; T024): NotificationService compares each successful refresh of "Nhà trường
## giao" with the list kept in RAM for the session: the first refresh of the session gives one grouped toast with the
## number of `not_started` assignments (never one toast per assignment); later, each new `assignment_id` with
## `status = not_started` gives `ui.notification.assignment_new` and raises the 🔔 badge; more than 3 new at once give
## one grouped toast; already known or started assignments give nothing; a failed refresh changes nothing. The texts
## carry the school's assignment title only — no other student, no score. FakeHttp (TEST-ONLY).

const ROUTE: String = "GET /learning/my-assignments"

var h: AuthHarness = null
var assignments: AssignmentService = null
var notes: NotificationService = null
var toasts: Array[Array] = []
var badges: Array[int] = []


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	assignments = AssignmentService.new(h.api)
	notes = NotificationService.new()
	notes.bind(assignments)
	toasts.clear()
	badges.clear()
	notes.toast_requested.connect(func(key: String, args: Dictionary) -> void: toasts.append([key, args]))
	notes.badge_changed.connect(func(n: int) -> void: badges.append(n))


func after_test() -> void:
	h.dispose()


static func _items(count: int, status: String = "not_started", offset: int = 0) -> Array:
	var out: Array = []
	for i: int in count:
		var item: Dictionary = AssignmentFixtures.item_sample(offset + i)
		item["status"] = status
		out.append(item)
	return out


func _refresh(items: Array) -> void:
	h.http.reply(ROUTE, 200, AssignmentFixtures.page(items))
	await assignments.refresh()


func test_first_refresh_of_the_session_gives_one_grouped_toast() -> void:
	var items: Array = _items(4)
	items.append_array(_items(1, "submitted", 10))
	await _refresh(items)
	assert_int(toasts.size()).is_equal(1)
	assert_str(String(toasts[0][0])).is_equal("ui.notification.assignments_pending")
	assert_dict(toasts[0][1] as Dictionary).is_equal({"count": 4})
	assert_int(notes.badge).is_equal(4)


func test_first_refresh_without_pending_work_is_silent() -> void:
	await _refresh(_items(2, "graded"))
	assert_array(toasts).is_empty()
	assert_int(notes.badge).is_equal(0)


func test_a_new_assignment_gives_its_toast_and_raises_the_badge() -> void:
	await _refresh(_items(1))
	toasts.clear()
	var next: Array = _items(1)
	next.append_array(_items(1, "not_started", 5))
	await _refresh(next)
	assert_int(toasts.size()).is_equal(1)
	assert_str(String(toasts[0][0])).is_equal("ui.notification.assignment_new")
	assert_dict(toasts[0][1] as Dictionary).is_equal({"title": "Bài 6"})
	assert_int(notes.badge).is_equal(2)


func test_known_or_started_assignments_give_nothing() -> void:
	await _refresh(_items(2))
	toasts.clear()
	var next: Array = _items(2)
	next.append_array(_items(1, "in_progress", 7))
	await _refresh(next)
	assert_array(toasts).is_empty()
	assert_int(notes.badge).is_equal(2)


func test_more_than_three_new_at_once_are_grouped() -> void:
	await _refresh([])
	await _refresh(_items(5, "not_started", 20))
	assert_int(toasts.size()).is_equal(1)
	assert_str(String(toasts[0][0])).is_equal("ui.notification.assignments_new_count")
	assert_dict(toasts[0][1] as Dictionary).is_equal({"count": 5})
	await _refresh(_items(3, "not_started", 40))
	assert_int(toasts.size()).is_equal(4)
	assert_int(notes.badge).is_equal(8)


func test_failed_refresh_changes_nothing() -> void:
	await _refresh(_items(1))
	toasts.clear()
	h.http.reply(ROUTE, 503, AuthHarness.problem(503, "UNAVAILABLE"))
	await assignments.refresh()
	assert_array(toasts).is_empty()
	assert_int(notes.badge).is_equal(1)


func test_clear_badge_and_session_reset() -> void:
	await _refresh(_items(2))
	notes.clear_badge()
	assert_int(notes.badge).is_equal(0)
	assert_int(badges.back()).is_equal(0)
	notes.reset()
	toasts.clear()
	await _refresh(_items(2))
	# A new session starts over with the grouped toast.
	assert_str(String(toasts[0][0])).is_equal("ui.notification.assignments_pending")


func test_texts_carry_no_other_student_and_no_score() -> void:
	var item: Dictionary = AssignmentFixtures.item_sample(0)
	item["status"] = "not_started"
	item["score"] = 9.5
	item["student_name"] = "Nguyễn Văn B"
	await _refresh([])
	await _refresh([item])
	assert_dict(toasts[0][1] as Dictionary).is_equal({"title": "Bài 1"})


func test_session_end_through_the_service_locator() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock)
	assert_object(services.notifications).is_not_null()
	h.http.reply(ROUTE, 200, AssignmentFixtures.page(_items(3)))
	await services.assignments.refresh()
	assert_int(services.notifications.badge).is_equal(3)
	services.end_session()
	assert_int(services.notifications.badge).is_equal(0)
