class_name JoinFriendService
extends RefCounted
## "Tham gia bạn" (F111 US1; SOCIAL_SYSTEM §4.2, MULTIPLAYER_GAMEPLAY §6, F091): moves the student next to a friend who
## is in a public area.

var hub: HubSync = null
var social: SocialService = null
## `(area_id: String, args: Dictionary) -> Error` coroutine; default SceneRouter.request_area (F071).
var area_requester: Callable = Callable()
## `() -> String` area the world shows now; default AppState.current_zone.
var current_area: Callable = Callable()
## `() -> Dictionary` the student's own state for JoinAvailability (offline, blocking activity, exam).
var self_state: Callable = Callable()


func _init(hub_sync: HubSync = null, social_service: SocialService = null) -> void:
	hub = hub_sync
	social = social_service


func clear() -> void:
	pass
