extends GdUnitTestSuite
## F068 avatar rig (ADR-0103 "3D Chibi" + "3D Animated Cartoon"; CHARACTER_SYSTEM §4, §5.2):
## - AC-06: AvatarRig.tscn carries the `kid_base` Skeleton3D defined by KidSkeleton; the skinned body and outfits bind
##   to it, hair/accessories ride bone attachments;
## - AC-05: `AvatarBuilder.set_expression()` swaps the face mesh (unknown values rejected), the neutral face blinks;
## - AC-07: the existing movement state (AnimDriver) plays the baked clips on the bones and picks the expression
##   (run → trying, jump → surprised, idle → neutral); an emote plays its baked clip with its expression.

const RIG: String = "res://scenes/character/AvatarRig.tscn"
const PLAYER: String = "res://scenes/character/Player.tscn"


func _rig() -> AvatarBuilder:
	var rig: AvatarBuilder = auto_free((load(RIG) as PackedScene).instantiate()) as AvatarBuilder
	add_child(rig)
	return rig


func _player() -> PlayerController:
	var runner: GdUnitSceneRunner = scene_runner(PLAYER)
	await runner.simulate_frames(1)
	return runner.scene() as PlayerController


func test_rig_skeleton_matches_kid_skeleton() -> void:
	var rig: AvatarBuilder = _rig()
	var skeleton: Skeleton3D = rig.get_node(KidSkeleton.SKELETON_PATH) as Skeleton3D
	assert_object(rig.skeleton).is_same(skeleton)
	assert_int(skeleton.get_bone_count()).is_equal(KidSkeleton.BONE_NAMES.size())
	for i: int in KidSkeleton.BONE_NAMES.size():
		assert_str(skeleton.get_bone_name(i)).is_equal(KidSkeleton.BONE_NAMES[i])
		assert_int(skeleton.get_bone_parent(i)).is_equal(KidSkeleton.BONE_PARENTS[i])
		assert_bool(skeleton.get_bone_rest(i).is_equal_approx(KidSkeleton.rest_local(i))).override_failure_message(
				KidSkeleton.BONE_NAMES[i]).is_true()
	var body: MeshInstance3D = rig.body_mesh
	assert_object(body.get_node(body.skeleton)).is_same(skeleton)
	assert_bool(AvatarBuilder.is_skinned(body.mesh)).is_true()
	# Slot anchors follow their bones (KidSkeleton.ANCHORS).
	for slot: String in KidSkeleton.ANCHORS:
		var spec: Array = KidSkeleton.ANCHORS[slot] as Array
		var anchor: Node3D = rig.slot_anchor(slot)
		var attach: BoneAttachment3D = anchor.get_parent() as BoneAttachment3D
		assert_str(attach.bone_name).override_failure_message(slot).is_equal(KidSkeleton.BONE_NAMES[int(spec[0])])
		assert_vector(anchor.position).is_equal_approx(spec[1] as Vector3, Vector3.ONE * 0.001)


func test_outfit_binds_to_the_skeleton_and_items_stay_rigid() -> void:
	var rig: AvatarBuilder = _rig()
	rig.apply(AvatarConfigModel.from_dict({"body_type": "kid_a", "skin_tone": "tone_02",
		"hair": {"item_id": "hair_short_01", "color": "hair_col_01"}, "outfit": {"item_id": "outfit_school_01"},
		"accessories": [{"slot": "hand", "item_id": "acc_flag_small"}]}))
	var outfit: MeshInstance3D = rig.slot_anchor("outfit").get_child(0) as MeshInstance3D
	assert_object(outfit.get_node(outfit.skeleton)).is_same(rig.skeleton)
	var outfit_material: ShaderMaterial = outfit.material_override as ShaderMaterial
	var skinned_preset: ShaderMaterial = load(String(Art3dTokens.MATERIAL_PATHS["toon_skinned"])) as ShaderMaterial
	assert_object(outfit_material.next_pass).is_same(skinned_preset.next_pass)
	var hair: MeshInstance3D = rig.slot_anchor("hair").get_child(0) as MeshInstance3D
	var solid_preset: ShaderMaterial = load(String(Art3dTokens.MATERIAL_PATHS["toon"])) as ShaderMaterial
	assert_object((hair.material_override as ShaderMaterial).next_pass).is_same(solid_preset.next_pass)
	assert_int(rig.slot_anchor("hand").get_child_count()).is_equal(1)


func test_expressions_are_switched_through_the_api() -> void:
	var rig: AvatarBuilder = _rig()
	var seen: Array[String] = []
	rig.expression_changed.connect(func(e: String) -> void: seen.append(e))
	for expression: String in KidSkeleton.EXPRESSIONS:
		assert_bool(rig.set_expression(expression)).is_true()
		assert_str(rig.face_mesh.mesh.resource_path).is_equal(KidSkeleton.face_path(expression))
	assert_bool(rig.set_expression("angry_villain")).is_false()
	assert_bool(rig.set_expression(KidSkeleton.BLINK)).is_false()
	assert_str(rig.expression).is_equal("trying")
	assert_array(seen).contains_exactly(["happy", "surprised", "trying"])


func test_neutral_face_blinks() -> void:
	var rig: AvatarBuilder = _rig()
	rig.set_process(false)
	rig.set_expression("neutral")
	rig.advance_blink(AvatarBuilder.BLINK_EVERY + 0.01)
	assert_bool(rig.blinking).is_true()
	assert_str(rig.face_mesh.mesh.resource_path).is_equal(KidSkeleton.face_path(KidSkeleton.BLINK))
	rig.advance_blink(AvatarBuilder.BLINK_TIME + 0.01)
	assert_bool(rig.blinking).is_false()
	assert_str(rig.face_mesh.mesh.resource_path).is_equal(KidSkeleton.face_path("neutral"))
	# Other expressions never blink.
	rig.set_expression("happy")
	rig.advance_blink(AvatarBuilder.BLINK_EVERY * 2.0)
	assert_bool(rig.blinking).is_false()


func test_movement_state_drives_the_baked_clips_and_the_face() -> void:
	var p: PlayerController = await _player()
	var driver: AnimDriver = p.anim_driver
	assert_bool(driver.rig_clips).is_true()
	var rig: AvatarBuilder = p.avatar as AvatarBuilder
	assert_object(driver.face).is_same(rig)
	var library: AnimationLibrary = (rig.get_node("AnimationPlayer") as AnimationPlayer).get_animation_library(AnimDriver.LIBRARY)
	for s: StringName in AnimDriver.STATES:
		assert_str(library.get_animation(s).resource_path).is_equal(KidSkeleton.clip_path(String(s)))
	driver.update(Vector3(0.0, 0.0, -5.0), true)
	assert_str(rig.expression).is_equal("trying")
	var runner: GdUnitSceneRunner = scene_runner(p)
	await runner.simulate_frames(12)
	var skeleton: Skeleton3D = rig.skeleton
	var leg: Quaternion = skeleton.get_bone_pose_rotation(KidSkeleton.LEG_L)
	assert_float(rad_to_deg(leg.get_angle())).is_greater(3.0)
	driver.update(Vector3(0.0, 2.0, 0.0), false)
	assert_str(rig.expression).is_equal("surprised")
	driver.update(Vector3.ZERO, true)
	assert_str(rig.expression).is_equal("neutral")


func test_emotes_play_their_clip_with_their_expression() -> void:
	var p: PlayerController = await _player()
	var driver: AnimDriver = p.anim_driver
	var rig: AvatarBuilder = p.avatar as AvatarBuilder
	var wave: ItemVisual = ItemVisual.load_item("emote_wave")
	assert_bool(driver.play_emote("emote_wave", wave.emote_clip)).is_true()
	assert_str(rig.expression).is_equal(AnimDriver.EMOTE_EXPRESSION)
	assert_float(driver.emote_time_left).is_equal_approx(wave.emote_clip.length, 0.001)
	driver.advance_emote(wave.emote_clip.length + 0.1)
	assert_str(rig.expression).is_equal("neutral")
	var sorry: ItemVisual = ItemVisual.load_item("emote_apologize")
	driver.play_emote("emote_apologize", sorry.emote_clip)
	assert_str(rig.expression).is_equal(sorry.expression)
