extends GdUnitTestSuite
## T018 (TC-LMS-002, F062-AC-07, F062-AC-09, F062-AC-11, AGENTS R8) against the backend compose with F041: `hs.an.3a`
## (th-demo-a, school DEMOA1, khối 3) and `hs.dung.7a` (pgd-demo, school DEMOB1, khối 7) each read their own
## `listMyAssignments`; no assignment id of one appears in the other's list (tenant isolation; since ADR-0095 the
## audience is the (school, grade) of the student, no class). Every call carries the session `X-Tenant-Id`. Each kind
## maps to the intent of F062-AC-09 (OQ F062-2: `question_set` → `activity_host {question_set_id}`). Skipped unless
## EDUWORLD_IT_API_BASE_URL is set (LiveBackend); the password comes from the environment only. Each student logs in
## exactly once per run and a failed login is never retried (the seed accounts lock after repeated failures).

const SKIP_REASON: String = "needs the backend compose with F041 (EDUWORLD_IT_API_BASE_URL)"
## States with a game session: the hub, or S02 first (F070) for a seed student who never created an avatar.
const SIGNED_IN_STATES: Array[int] = [GameStateMachine.State.IN_WORLD, GameStateMachine.State.CHARACTER_CREATION]
const EXPECTED_SCREENS: Dictionary = {
	"activity": &"activity_host", "question_set": &"activity_host", "assessment": &"exam_lobby",
}


func _assignments_of(username: String) -> Array:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	live.auth.profiles = ProfileService.new(live.api)
	await live.auth.login(username, LiveBackend.student_password())
	var signed_in: bool = live.last_state() in SIGNED_IN_STATES
	assert_bool(signed_in).override_failure_message("%s: login ended in state %d" % [username, live.last_state()]).is_true()
	var ids: Array = []
	if not signed_in:
		live.dispose()
		return ids
	var service: AssignmentService = AssignmentService.new(live.api)
	var intents: Array[Array] = []
	service.intent_sink = func(screen: StringName, args: Dictionary) -> void: intents.append([screen, args])
	var ok: bool = await service.refresh()
	assert_bool(ok).override_failure_message("%s: listMyAssignments failed (%s)" % [username, service.last_error_code]).is_true()
	var trace_ids: PackedStringArray = []
	for rec: Dictionary in live.transport.sent:
		if String(rec["path"]) == AssignmentService.PATH:
			var headers: Dictionary = rec["headers"] as Dictionary
			assert_str(String(headers.get("x-tenant-id", ""))).is_equal(live.api.tokens.tenant_id)
			trace_ids.append(TraceContext.trace_id_of(String(headers.get("traceparent", ""))))
	var kinds: Dictionary = {}
	var with_subject: int = 0
	for m: MyAssignmentModel in service.assignments:
		ids.append(m.assignment_id)
		kinds[m.content_kind] = int(kinds.get(m.content_kind, 0)) + 1
		if not m.subject_name.is_empty():
			with_subject += 1
		var screen: StringName = service.start(m)
		if EXPECTED_SCREENS.has(m.content_kind):
			assert_str(String(screen)).is_equal(String(EXPECTED_SCREENS[m.content_kind]))
		else:
			assert_str(String(screen)).is_empty()
	# Evidence for quickstart.md (no token, no password, no student name).
	print("[F062-IT] user=%s state=%d tenant=%s assignments=%d kinds=%s with_subject_name=%d intents=%d trace_ids=%s" % [
		username, live.last_state(), live.api.tokens.tenant_id, ids.size(), JSON.stringify(kinds), with_subject,
		intents.size(),
		",".join(trace_ids),
	])
	await live.auth.logout()
	live.dispose()
	return ids


func test_students_only_see_their_own_school_grade(
		do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var an: Array = await _assignments_of("hs.an.3a")
	var dung: Array = await _assignments_of("hs.dung.7a")
	assert_array(an).override_failure_message("hs.an.3a has no assignment: nothing assigned to DEMOA1 khối 3").is_not_empty()
	for id: Variant in an:
		assert_array(dung).not_contains([id])
