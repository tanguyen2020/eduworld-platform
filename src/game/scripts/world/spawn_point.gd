class_name SpawnPoint
extends Marker3D
## Spawn point of a zone (WORLD_RENDERING §4): `spawn_point_id` unique in the zone (e.g. `main_hub.default`), a
## scatter `radius` so players do not overlap, the initial camera `facing` (yaw, degrees) and `tags`.

@export var spawn_point_id: String = ""
@export var radius: float = 0.0
@export var facing: float = 0.0
@export var tags: PackedStringArray = []


## Position for one player: the marker, scattered inside `radius` by `rng` (deterministic in tests).
func pick_position(rng: RandomNumberGenerator = null) -> Vector3:
	if radius <= 0.0 or rng == null:
		return global_position
	var angle: float = rng.randf() * TAU
	var dist: float = sqrt(rng.randf()) * radius
	return global_position + Vector3(cos(angle) * dist, 0.0, sin(angle) * dist)
