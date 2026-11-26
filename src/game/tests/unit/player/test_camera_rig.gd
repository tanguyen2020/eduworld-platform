extends GdUnitTestSuite
## F096-AC-7 (FR-GAME-003, F062-AC-02, STUDENT_GAME_UX §4): at the spawn point the third-person camera is above and behind
## the player — the SpringArm3D starts inside the rig's pitch range. Before F096 it pointed 20° down into the ground, so
## the first entry to the Hub showed the camera at ground level inside the marker of the welcome sign.

const PLAYER: String = "res://scenes/character/Player.tscn"


func test_initial_pitch_is_inside_the_rig_range() -> void:
	var player: Node3D = auto_free((load(PLAYER) as PackedScene).instantiate()) as Node3D
	var rig: CameraRig = player.get("camera_rig") as CameraRig
	var pitch_deg: float = rad_to_deg(rig.pitch())
	assert_float(pitch_deg).is_between(rig.pitch_min_deg, rig.pitch_max_deg)


func test_camera_is_above_and_behind_the_player_at_spawn() -> void:
	var world: Node3D = auto_free(Node3D.new()) as Node3D
	add_child(world)
	var player: Node3D = (load(PLAYER) as PackedScene).instantiate() as Node3D
	world.add_child(player)
	player.global_position = Vector3(0.0, 0.0, 9.0)
	await await_idle_frame()
	await await_idle_frame()
	var camera: Camera3D = player.find_child("Camera3D", true, false) as Camera3D
	var rig: CameraRig = player.get("camera_rig") as CameraRig
	assert_float(camera.global_position.y).override_failure_message("camera above the head").is_greater(rig.global_position.y)
	# Behind: on the +Z side of a player facing -Z (yaw 0), looking down at the player.
	assert_float(camera.global_position.z).is_greater(player.global_position.z)
	assert_float(-camera.global_basis.z.y).override_failure_message("looks down at the player").is_less(0.0)
