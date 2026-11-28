class_name ClubService
extends GameApiService
## Clubs of the signed-in student (F111 US5, US6; SOCIAL_SYSTEM §6; backend F120). The only place of the client that
## calls `/social/clubs*`, `/social/club-*` (AGENTS R1).

var store: ClubStore = ClubStore.new()


func _init(rest: RestApiClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)


func clear() -> void:
	store.clear()
