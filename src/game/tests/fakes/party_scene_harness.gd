class_name PartySceneHarness
extends RefCounted
## TEST-ONLY (AGENTS R17): a signed-in student (AuthHarness on FakeHttp) with the session services in AppState and the
## party routes scripted from the contract fixtures (PartyFixtures), for the F102 scene tests, the grade-band matrix
## and the review captures. The signed-in student leads the sample party ("Nhóm Sao Băng") unless `as_member()`.
## The realtime client is the offline autoload: Ready / activity go over REST, the realtime messages are delivered
## with `push()` (the dispatcher handler of PartyService).

var h: AuthHarness = null
var services: ServiceLocator = null
var party: PartyService = null
var self_id: String = ""
## Calls of the session router (scene_ref|room_id) instead of a real zone change.
var entered: PackedStringArray = []


func start(alone: bool = false) -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	AppState.services = services
	party = services.party
	party._refresh_self()
	self_id = party.store.self_id
	party.session_router = func(scene_ref: String, room_id: String) -> Error:
		entered.append("%s|%s" % [scene_ref, room_id])
		return OK
	PartyFixtures.script(h.http, self_id, alone)


## The sample party with the signed-in student as a plain member (CaHeoVui07's place); MeoXanh42 leads.
func member_party() -> Dictionary:
	var p: Dictionary = PartyFixtures.party()
	return JSON.parse_string(JSON.stringify(p).replace(PartyFixtures.MEMBER, self_id)) as Dictionary


func as_member() -> void:
	h.http.clear(PartyFixtures.ROUTE_MY_PARTY).reply(PartyFixtures.ROUTE_MY_PARTY, 200, member_party())


## Delivers the realtime message `type` with `payload` (AsyncAPI shape) to PartyService.
func push(type: String, payload: Dictionary) -> void:
	party.handle_message(type, payload)


func dispose() -> void:
	if party != null:
		party.clear()
	if services != null and services.realtime != null:
		services.realtime.unregister_handler(PartyService.PREFIX, party.handle_message)
	AppState.services = null
	SceneRouter.party_room_id = ""
	if h != null:
		h.dispose()
