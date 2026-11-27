class_name GameStateMachine
extends RefCounted
## Client app states of the entry flow (GODOT_ARCHITECTURE §5, STUDENT_GAME_UX_CORE S01 sub-states) and the scene
## that renders each one. AuthService decides the state; SceneRouter shows the scene. Pure data, no I/O.

enum State {
	BOOT, VERSION_CHECK, FORCE_UPDATE, MAINTENANCE, RESUMING, RESUME_CONFIRM, LOGIN, SIGNING_IN, LOGIN_WAITING,
	TENANT_CHOICE, CREATING_SESSION, TAKEOVER_CONFIRM, NOT_STUDENT, NOT_PROVISIONED, ACCOUNT_BLOCKED, CURFEW,
	REPLACED_NOTICE, IN_WORLD, CHARACTER_CREATION,
}

## In-world destination: the Main Hub greybox with its HUD (F062).
const IN_WORLD_SCENE: String = "res://scenes/world/WorldRoot.tscn"
## S02 first-time Character Creation, opened before the hub while `GameProfile.setup_required` (F070-AC-01).
const CHARACTER_CREATION_SCENE: String = "res://scenes/character/CharacterCreation.tscn"

## Scene per state; states without an entry keep the current scene (busy states of the same screen).
const SCENES: Dictionary = {
	State.VERSION_CHECK: "res://scenes/bootstrap/Splash.tscn",
	State.FORCE_UPDATE: "res://scenes/bootstrap/UpdateRequired.tscn",
	State.MAINTENANCE: "res://scenes/bootstrap/Maintenance.tscn",
	State.RESUMING: "res://scenes/auth/ResumeSplash.tscn",
	State.RESUME_CONFIRM: "res://scenes/auth/ResumeConfirm.tscn",
	State.LOGIN: "res://scenes/auth/Login.tscn",
	State.LOGIN_WAITING: "res://scenes/auth/LoginWaiting.tscn",
	State.TENANT_CHOICE: "res://scenes/auth/TenantChoice.tscn",
	State.TAKEOVER_CONFIRM: "res://scenes/ui/dialogs/TakeoverConfirm.tscn",
	State.NOT_STUDENT: "res://scenes/auth/NotStudent.tscn",
	State.NOT_PROVISIONED: "res://scenes/auth/AccountNotProvisioned.tscn",
	State.ACCOUNT_BLOCKED: "res://scenes/auth/AccountBlocked.tscn",
	State.CURFEW: "res://scenes/auth/CurfewNotice.tscn",
	State.REPLACED_NOTICE: "res://scenes/ui/dialogs/ReplacedNotice.tscn",
	State.IN_WORLD: IN_WORLD_SCENE,
	State.CHARACTER_CREATION: CHARACTER_CREATION_SCENE,
}

## 403 codes of a blocked account/tenant (GODOT_ARCHITECTURE §9) shown on AccountBlocked.
const BLOCKED_CODES: PackedStringArray = [
	"ACCOUNT_LOCKED", "ACCOUNT_DEACTIVATED", "NO_ACTIVE_ENROLLMENT", "TENANT_ACCESS_DISABLED",
	"ACCESS_OUTSIDE_ALLOWED_HOURS",
]
## Refresh/resume failures that end the stored session (ADR-0086, F061-AC-30).
const REFRESH_FATAL_401: PackedStringArray = [
	"REFRESH_TOKEN_INVALID", "REFRESH_TOKEN_REUSED", "REFRESH_TOKEN_DEVICE_MISMATCH",
]
const REFRESH_FATAL_403: PackedStringArray = [
	"ACCOUNT_LOCKED", "ACCOUNT_DEACTIVATED", "NO_ACTIVE_ENROLLMENT", "TENANT_ACCESS_DISABLED", "TENANT_MISMATCH",
	"GAME_ACCESS_STUDENT_ONLY", "ACCOUNT_NOT_PROVISIONED", "ACCESS_OUTSIDE_ALLOWED_HOURS",
]


static func scene_for(state: State) -> String:
	return String(SCENES.get(state, ""))


## Screen after a failed `gameLogin` / `selectGameLoginTenant` (F061-AC-05…09, 13, 14, 27, 36).
static func state_for_login_failure(code: String) -> State:
	match code:
		"TENANT_NOT_SELECTED":
			return State.TENANT_CHOICE
		"GAME_ACCESS_STUDENT_ONLY":
			return State.NOT_STUDENT
		"ACCOUNT_NOT_PROVISIONED":
			return State.NOT_PROVISIONED
		"CLIENT_VERSION_UNSUPPORTED":
			return State.FORCE_UPDATE
		"CURFEW_ACTIVE":
			return State.CURFEW
	if code in BLOCKED_CODES:
		return State.ACCOUNT_BLOCKED
	return State.LOGIN


## A refresh/resume failure that must erase the stored session (vs. transient: keep it and retry).
static func is_refresh_fatal(status: int, code: String) -> bool:
	return (status == 401 and (code in REFRESH_FATAL_401 or code == "UNAUTHENTICATED")) \
			or (status == 403 and code in REFRESH_FATAL_403)
