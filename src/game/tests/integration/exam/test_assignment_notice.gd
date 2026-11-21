extends GdUnitTestSuite
## T026 (F064-AC-20, F064-AC-21, TC-LMS-002) — partial: against the backend compose, `hs.an.3a` refreshes "Nhà trường
## giao" twice; the first refresh of the session gives at most one grouped toast (the count of `not_started` work), an
## unchanged second refresh gives none. The step "`ad.truong.a` assigns new work while the student is in the Hub" is
## run by hand from quickstart.md #1 (the portal LMS flow) until a staff API helper exists for Godot tests.
## Skipped unless EDUWORLD_IT_API_BASE_URL is set (LiveBackend).

const SKIP_REASON: String = "needs the backend compose with the F041 seed (EDUWORLD_IT_API_BASE_URL)"


func test_first_refresh_is_grouped_and_an_unchanged_refresh_is_silent(
		do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await live.auth.login("hs.an.3a", LiveBackend.student_password())
	var assignments: AssignmentService = AssignmentService.new(live.api)
	var notes: NotificationService = NotificationService.new()
	notes.bind(assignments)
	var toasts: Array[String] = []
	notes.toast_requested.connect(func(key: String, _args: Dictionary) -> void: toasts.append(key))
	assert_bool(await assignments.refresh()).is_true()
	assert_int(toasts.size()).is_less_equal(1)
	if not toasts.is_empty():
		assert_str(toasts[0]).is_equal(NotificationService.KEY_PENDING)
	toasts.clear()
	assert_bool(await assignments.refresh()).is_true()
	assert_array(toasts).is_empty()
	await live.auth.logout()
	live.dispose()
