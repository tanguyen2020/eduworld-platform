extends GdUnitTestSuite
## F133 T016 (F133-AC-12, TC-AUTH-066, ADR-0101): UpdateService turns `426 CLIENT_VERSION_UNSUPPORTED` or
## `getClientConfig.update_required` into one `update_required` signal with the right way to update for the platform:
## Android/iOS open the store page (server `store_url`, else the store link of the build config), a shared Android
## device of a school asks the teacher (MDM installs the school kit), Web reloads the page (no Windows self-update).
## A link that is not https is never opened.

const PLAY: String = "https://play.google.com/store/apps/details?id=vn.eduworld.game"
const APPLE: String = "https://apps.apple.com/app/id0000000000"
const STORES: Dictionary = {"android": PLAY, "ios": APPLE}

var _seen: Array[Dictionary] = []


func before_test() -> void:
	_seen.clear()


func _service(platform: String, shared: bool = false, stores: Dictionary = STORES) -> UpdateService:
	var s: UpdateService = UpdateService.new(platform, shared, stores)
	s.update_required.connect(func(action: int, url: String) -> void: _seen.append({"action": action, "url": url}))
	return s


func _problem_426(store_url: String) -> ApiResult:
	var r: ApiResult = ApiResult.failure("CLIENT_VERSION_UNSUPPORTED")
	r.status = 426
	r.problem = ApiProblem.from_dict({"type": "about:blank", "title": "x", "status": 426,
		"code": "CLIENT_VERSION_UNSUPPORTED", "min_client_version": "2.0.0", "store_url": store_url})
	return r


func _config(update_required: bool, min_version: String) -> ClientConfig:
	return ClientConfig.from_dict({"min_client_version": min_version, "update_required": update_required,
		"maintenance": {"active": false}})


func test_426_on_android_opens_the_store_link_from_the_server() -> void:
	var s: UpdateService = _service("android")
	assert_bool(s.check_result(_problem_426("https://play.google.com/store/apps/details?id=x"))).is_true()
	assert_int(_seen.size()).is_equal(1)
	assert_int(int(_seen[0]["action"])).is_equal(UpdateService.Action.OPEN_STORE)
	assert_str(String(_seen[0]["url"])).is_equal("https://play.google.com/store/apps/details?id=x")
	assert_str(s.min_client_version).is_equal("2.0.0")


func test_update_required_on_ios_uses_the_app_store_link_of_the_build() -> void:
	var s: UpdateService = _service("ios")
	assert_bool(s.check_client_config(_config(true, "1.0.0"), "1.0.0")).is_true()
	assert_int(int(_seen[0]["action"])).is_equal(UpdateService.Action.OPEN_STORE)
	assert_str(String(_seen[0]["url"])).is_equal(APPLE)


func test_version_below_minimum_also_requires_an_update_and_a_current_one_does_not() -> void:
	var s: UpdateService = _service("android")
	assert_bool(s.check_client_config(_config(false, "1.0.0"), "1.0.0")).is_false()
	assert_bool(s.check_client_config(_config(false, "1.1.0"), "1.0.0")).is_true()
	assert_str(String(_seen[0]["url"])).is_equal(PLAY)
	assert_bool(s.check_result(ApiResult.failure("NETWORK_ERROR"))).is_false()
	assert_int(_seen.size()).is_equal(1)


func test_web_reloads_the_page_and_never_opens_a_store() -> void:
	var s: UpdateService = _service("web", true)
	s.check_result(_problem_426(PLAY))
	assert_int(int(_seen[0]["action"])).is_equal(UpdateService.Action.RELOAD_PAGE)
	assert_str(String(_seen[0]["url"])).is_empty()


func test_shared_android_device_asks_the_teacher() -> void:
	var s: UpdateService = _service("android", true)
	s.check_result(_problem_426(PLAY))
	assert_int(int(_seen[0]["action"])).is_equal(UpdateService.Action.ASK_TEACHER)


func test_no_https_link_means_ask_the_teacher_never_an_insecure_link() -> void:
	var s: UpdateService = _service("android", false, {"android": "http://play.example/app"})
	s.check_result(_problem_426("javascript:alert(1)"))
	assert_int(int(_seen[0]["action"])).is_equal(UpdateService.Action.ASK_TEACHER)
	assert_str(String(_seen[0]["url"])).is_empty()
	var opened: Array[String] = []
	s.opener = func(url: String) -> void: opened.append(url)
	assert_bool(s.perform(UpdateService.Action.OPEN_STORE, "http://play.example/app")).is_false()
	assert_bool(s.perform(UpdateService.Action.OPEN_STORE, PLAY)).is_true()
	assert_array(opened).contains_exactly([PLAY])


func test_resolve_is_pure_and_documents_each_platform() -> void:
	assert_dict(UpdateService.resolve("android", false, "", STORES)).is_equal(
			{"action": UpdateService.Action.OPEN_STORE, "url": PLAY})
	assert_dict(UpdateService.resolve("web", true, PLAY, STORES)).is_equal(
			{"action": UpdateService.Action.RELOAD_PAGE, "url": ""})
	assert_dict(UpdateService.resolve("other", true, "", STORES)).is_equal(
			{"action": UpdateService.Action.ASK_TEACHER, "url": ""})
	assert_dict(UpdateService.resolve("windows", false, PLAY, STORES)).is_equal(
			{"action": UpdateService.Action.ASK_TEACHER, "url": ""})
