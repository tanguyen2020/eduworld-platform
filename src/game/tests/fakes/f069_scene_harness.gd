class_name F069SceneHarness
extends RefCounted
## TEST-ONLY (AGENTS R17): a signed-in student on FakeHttp with the session services (AppState.services), every F069
## route scripted with F069Fixtures, the profile read and the grade band set — for the F069 scene tests. `dispose()`
## restores AppState.

var h: AuthHarness = null
var services: ServiceLocator = null
var _previous: ServiceLocator = null
var _band: String = ""
var _locale: String = ""


static func create(band: String = "g1_3") -> F069SceneHarness:
	var s: F069SceneHarness = F069SceneHarness.new()
	return s


## Coroutine: signs in, builds the services and scripts the F069 routes.
func start(band: String = "g1_3") -> void:
	_previous = AppState.services
	_band = AppState.grade_band
	_locale = String(Settings.locale)
	Settings.set_locale("vi-VN", false)
	h = AuthHarness.create()
	await h.sign_in()
	F069Fixtures.script_all(h.http, band)
	services = ServiceLocator.new(h.api, AppState, h.clock)
	AppState.services = services
	await services.profile.fetch_profile()
	AppState.player = AuthHarness.bootstrap()
	AppState.start_ui(Engine.get_main_loop().root as Window)
	AppState.set_grade_band(band)


func dispose() -> void:
	if h != null:
		h.dispose()
	AppState.services = _previous
	AppState.set_grade_band(_band)
	Settings.set_locale(_locale, false)
	for blocker: StringName in [&"challenge", &"exam"]:
		AppState.end_blocking(blocker)
