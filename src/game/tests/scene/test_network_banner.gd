extends GdUnitTestSuite
## F092-AC-08, F092-AC-18 (T011, T019): the HUD banner shows "Đang kết nối lại…" while the connection comes back
## (not for the quiet reconnect after `system.server.draining`), hides once Online, shows the degraded indicator on
## `system.service.degraded`; write commands are refused meanwhile; texts are `ui.realtime.*` keys translated at once
## when the locale changes; the style comes from the generated Theme (no override in the scene).

const SCENE: String = "res://scenes/ui/hud/NetworkBanner.tscn"
const STATE := GameRealtimeClient.State

var h: RealtimeHarness = null
var _locale: String = ""


func before_test() -> void:
	h = RealtimeHarness.create()
	_locale = TranslationServer.get_locale()


func after_test() -> void:
	TranslationServer.set_locale(_locale)
	h.dispose()


func _banner() -> PanelContainer:
	var banner: PanelContainer = auto_free((load(SCENE) as PackedScene).instantiate()) as PanelContainer
	add_child(banner)
	banner.call("bind", h.client)
	return banner


func _status(banner: PanelContainer) -> Label:
	return banner.find_child("NetworkStatus", true, false) as Label


func test_banner_while_reconnecting_and_writes_locked() -> void:
	var banner: PanelContainer = _banner()
	await h.connect_online()
	assert_bool(banner.visible).is_false()
	h.peer().server_close(-1)
	h.client.tick()
	assert_bool(banner.visible).is_true()
	assert_str(_status(banner).text).is_equal("ui.realtime.reconnecting")
	assert_bool(h.client.can_write()).is_false()
	var r: CommandResult = await h.client.send_command("hub.player.emote", {"room_id": "r", "emote_id": "emote_wave"})
	assert_str(r.code).is_equal(CommandResult.CODE_OFFLINE)
	h.advance_ms(700)
	await h.await_ticket()
	assert_bool(banner.visible).is_true()
	h.open_with_welcome(1, "rt-new")
	h.push({"v": 1, "type": "system.session.resumed", "seq": h.client.session.last_seq + 1, "ts": 1, "payload": {}})
	assert_int(h.client.state).is_equal(STATE.ONLINE)
	assert_bool(banner.visible).is_false()


func test_no_banner_for_the_draining_reconnect() -> void:
	var banner: PanelContainer = _banner()
	await h.connect_online()
	h.push(h.envelope("system.server.draining", {"reconnect_after_ms": 2000}))
	h.peer().server_close(GameRealtimeClient.CLOSE_SERVICE_RESTART)
	h.client.tick()
	assert_int(h.client.state).is_equal(STATE.BACKOFF)
	assert_bool(banner.visible).is_false()


func test_degraded_indicator() -> void:
	var banner: PanelContainer = _banner()
	await h.connect_online()
	h.push_fixture("system.service.degraded.chat_presence")
	assert_bool(banner.visible).is_true()
	assert_str(_status(banner).text).is_equal("ui.realtime.degraded")


func test_texts_follow_the_locale_and_use_the_theme() -> void:
	var banner: PanelContainer = _banner()
	await h.connect_online()
	h.peer().server_close(-1)
	h.client.tick()
	var label: Label = _status(banner)
	TranslationServer.set_locale("vi")
	assert_str(tr(label.text)).is_equal("Đang kết nối lại…")
	TranslationServer.set_locale("en")
	assert_str(tr(label.text)).is_equal("Reconnecting…")
	banner.call("show_hub_notice", "ui.hub.room_full")
	var notice: Label = banner.find_child("HubNotice", true, false) as Label
	assert_bool(notice.visible).is_true()
	assert_str(tr(notice.text)).is_not_equal("ui.hub.room_full")
	for key: String in ["ui.realtime.reconnecting", "ui.realtime.degraded", "ui.hub.room_full", "ui.hub.join_denied",
			"ui.hub.area_not_found", "ui.hub.emote_rate_limited", "ui.hub.emote_not_owned"]:
		for locale: String in ["vi", "en"]:
			TranslationServer.set_locale(locale)
			assert_str(tr(key)).override_failure_message("%s %s" % [locale, key]).is_not_equal(key)
	# Style from the Theme only: no local overrides on the banner nodes.
	for node: Node in [banner, label, notice]:
		var control: Control = node as Control
		assert_bool(control.has_theme_color_override("font_color")).is_false()
		assert_bool(control.has_theme_font_size_override("font_size")).is_false()
	assert_str(String(banner.theme_type_variation)).is_equal("HudPanel")
	assert_str(String(label.theme_type_variation)).is_equal("HudLabel")


func test_hub_notice_hides_after_a_few_seconds() -> void:
	var banner: PanelContainer = _banner()
	banner.call("show_hub_notice", "ui.hub.emote_rate_limited")
	assert_bool(banner.visible).is_true()
	banner.call("_process", 5.0)
	assert_bool(banner.visible).is_false()
