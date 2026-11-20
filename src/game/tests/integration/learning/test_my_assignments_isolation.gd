extends GdUnitTestSuite
## T018 (TC-LMS-002, F062-AC-07, F062-AC-11, AGENTS R8): against the backend compose with F041 — `hs.an.3a` (th-demo-a,
## lớp 3A) sees only the assignments of 3A and `hs.dung.7a` (pgd-demo) none of th-demo-a; every call carries the
## session `X-Tenant-Id`. Skipped unless EDUWORLD_IT_API_BASE_URL is set (LiveBackend); until F041 is Done the endpoint
## exists only on the labelled Prism MOCK, so this test stays skipped in M-C2 (CODE_PLAN §5 (*)).

const SKIP_REASON: String = "needs the backend compose with F041 (EDUWORLD_IT_API_BASE_URL)"


func _assignments_of(username: String) -> Array:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await live.auth.login(username, LiveBackend.student_password())
	assert_int(live.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	var service: AssignmentService = AssignmentService.new(live.api)
	assert_bool(await service.refresh()).is_true()
	for rec: Dictionary in live.transport.sent:
		if String(rec["path"]) == AssignmentService.PATH:
			assert_str(String((rec["headers"] as Dictionary).get("x-tenant-id", ""))).is_equal(live.api.tokens.tenant_id)
	var ids: Array = []
	for m: MyAssignmentModel in service.assignments:
		ids.append(m.assignment_id)
	await live.auth.logout()
	live.dispose()
	return ids


func test_students_only_see_their_own_class(do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var an: Array = await _assignments_of("hs.an.3a")
	var dung: Array = await _assignments_of("hs.dung.7a")
	assert_array(an).is_not_empty()
	for id: Variant in an:
		assert_array(dung).not_contains([id])
