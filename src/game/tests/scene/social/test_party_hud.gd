extends GdUnitTestSuite
## T019, T021 (F102-AC-01 entry, F102-AC-12, F102-AC-13, F102-AC-14): the HUD `PartyChip` slot is the PartyHud — the
## "Nhóm" button (group icon + word, touch target of the band) asks for the Party screen; while in a party the strip
## shows each member (2D portrait, crown, tick when Ready, faded frame + wifi-off while reconnecting, the nickname and
## the state in words in the tooltip) and the leader grace banner; leaving the party empties it.

const HUD: String = "res://scenes/ui/hud/MainHud.tscn"

var s: PartySceneHarness = null


func before_test() -> void:
	s = PartySceneHarness.new()
	await s.start()


func after_test() -> void:
	s.dispose()


func _hud() -> Array:
	var runner: GdUnitSceneRunner = scene_runner(HUD)
	await runner.simulate_frames(2)
	var hud: CanvasLayer = runner.scene() as CanvasLayer
	return [runner, hud, hud.call("party_hud") as PartyHud]


func test_party_button_asks_for_the_party_screen() -> void:
	var r: Array = await _hud()
	var hud: CanvasLayer = r[1] as CanvasLayer
	var party_hud: PartyHud = r[2] as PartyHud
	var asked: Array[Dictionary] = []
	hud.connect("party_requested", func(args: Dictionary) -> void: asked.append(args))
	var button: Button = party_hud.button()
	assert_bool(button.is_visible_in_tree()).is_true()
	assert_object(button.icon).is_not_null()
	assert_str(tr(button.text)).is_equal(tr("ui.party.hud.button"))
	assert_float(button.size.y).is_greater_equal(UiBuild.target())
	assert_int(party_hud.member_count()).is_equal(0)
	button.pressed.emit()
	assert_array(asked).is_equal([{}])
	assert_bool(button in (hud.call("touch_buttons") as Array)).is_true()


func test_strip_shows_members_ready_and_reconnecting_and_the_leader_banner() -> void:
	var r: Array = await _hud()
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var party_hud: PartyHud = r[2] as PartyHud
	await s.party.load_my_party()
	await runner.simulate_frames(1)
	assert_int(party_hud.member_count()).is_equal(3)
	assert_str(tr(party_hud.button().text)).is_equal(tr("ui.party.hud.button_in_party"))
	var mini: Control = party_hud.find_child("Mini_%s" % PartyFixtures.WAITING.right(12), true, false) as Control
	assert_str(mini.tooltip_text).is_equal(tr("ui.party.hud.member_waiting").format({"nickname": "GauNau15"}))
	s.push("party.member.status_changed", PartyFixtures.ws("member.status_changed.disconnected"))
	await runner.simulate_frames(1)
	assert_bool(party_hud.is_faded(PartyFixtures.MEMBER)).is_true()
	assert_bool(party_hud.is_faded(PartyFixtures.WAITING)).is_false()
	assert_bool(party_hud.banner_visible()).is_false()
	s.push("party.member.status_changed", {"party_id": PartyFixtures.PARTY_ID, "user_id": s.self_id, "status": "disconnected"})
	await runner.simulate_frames(1)
	assert_bool(party_hud.banner_visible()).is_true()
	s.party.store.close(PartyStore.CLOSED_LEFT)
	await runner.simulate_frames(1)
	assert_int(party_hud.member_count()).is_equal(0)
	assert_bool(party_hud.banner_visible()).is_false()
