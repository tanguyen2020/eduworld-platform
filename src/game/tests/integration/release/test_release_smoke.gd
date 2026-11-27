extends GdUnitTestSuite
## F133 T014 (F133-AC-06, TC-AUTH-001, ADR-0083): release smoke against a deployed backend, run by
## scripts/godot/smoke.sh in `godot / smoke` before publishing: boot → `getClientConfig` → `gameLogin` of the QA student
## through the form flow → `createGameSession` → Hub (state IN_WORLD), and the client is not refused as too old.
## Skipped unless EDUWORLD_IT_API_BASE_URL, EDUWORLD_SMOKE_USERNAME and EDUWORLD_IT_STUDENT_PASSWORD are set (the
## username/password are secrets of the GitHub Environment `staging` — never in the repository, AGENTS R3/R4).

const ENV_USERNAME: String = "EDUWORLD_SMOKE_USERNAME"
const SKIP_REASON: String = "needs a backend (EDUWORLD_IT_API_BASE_URL) and the QA student (EDUWORLD_SMOKE_USERNAME, password)"


static func _skip() -> bool:
	return LiveBackend.skip() or OS.get_environment(ENV_USERNAME).is_empty() or LiveBackend.student_password().is_empty()


func test_boot_client_config_login_session_hub(do_skip: bool = _skip(), skip_reason: String = SKIP_REASON) -> void:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await live.auth.start()
	assert_int(live.last_state()).override_failure_message("client-config refused this version or failed") \
			.is_equal(GameStateMachine.State.LOGIN)
	await live.auth.login(OS.get_environment(ENV_USERNAME), LiveBackend.student_password())
	assert_int(live.last_state()).is_equal(GameStateMachine.State.IN_WORLD)
	var calls: PackedStringArray = []
	for rec: Dictionary in live.transport.sent:
		calls.append("%d %s" % [int(rec["method"]), String(rec["path"])])
	assert_array(calls).contains(["%d /game/client-config" % HTTPClient.METHOD_GET,
		"%d /auth/game/login" % HTTPClient.METHOD_POST, "%d /game/sessions" % HTTPClient.METHOD_POST])
	await live.auth.logout()
	live.dispose()
