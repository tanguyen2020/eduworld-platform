extends RefCounted
## Animation clips of the `kid_base` rig (F068, ADR-0103 "3D Animated Cartoon"; CHARACTER_SYSTEM §4, §5.2): idle
## breathing, walk, bouncy run, jump, and the emote clips wave, celebrate, clap, dance, thank, apologize. Cartoon squash
## & stretch is a scale of the root bone (volume kept: x·y·z ≈ 1). Every clip keys the same tracks (rotation of every
## bone, position + scale of the root, scale of the chest) so the AnimationTree blends between them without falling
## back to other values. Baked to res://assets/models/chr/anim_kid_<clip>_a.res by tools/art/bake_art.gd.
## Angles in degrees (Euler YXZ, bone local): +X swings a limb forward, +Z raises the right arm / -Z the left arm,
## -X leans the chest or head forward.
## SPDX-License-Identifier: MIT — EduWorld first-party (ADR-0102). Bake-time only.

const K := preload("res://scripts/player/kid_skeleton.gd")
const LOCOMOTION: PackedStringArray = ["idle", "walk", "run", "jump"]
const EMOTES: PackedStringArray = ["wave", "celebrate", "clap", "dance", "thank", "apologize"]
## Clip → [length s, loop].
const SPECS: Dictionary = {
	"idle": [2.4, true], "walk": [0.8, true], "run": [0.5, true], "jump": [0.7, false],
	"wave": [1.8, false], "celebrate": [1.8, false], "clap": [1.5, false], "dance": [1.6, false],
	"thank": [1.4, false], "apologize": [1.8, false],
}


static func names() -> PackedStringArray:
	var out: PackedStringArray = LOCOMOTION.duplicate()
	out.append_array(EMOTES)
	return out


static func clip(clip_name: String) -> Animation:
	var spec: Array = SPECS[clip_name] as Array
	var c: Dictionary = _keys(clip_name, float(spec[0]))
	var anim: Animation = Animation.new()
	anim.length = float(spec[0])
	anim.loop_mode = Animation.LOOP_LINEAR if bool(spec[1]) else Animation.LOOP_NONE
	for bone: int in K.BONE_NAMES.size():
		var keys: Array = c.get("rot_%d" % bone, [[0.0, Vector3.ZERO]]) as Array
		var t: int = anim.add_track(Animation.TYPE_ROTATION_3D)
		anim.track_set_path(t, K.track(bone))
		anim.track_set_interpolation_type(t, Animation.INTERPOLATION_CUBIC)
		for k: Array in keys:
			var e: Vector3 = k[1] as Vector3
			anim.rotation_track_insert_key(t, float(k[0]), Quaternion.from_euler(Vector3(deg_to_rad(e.x), deg_to_rad(e.y),
					deg_to_rad(e.z))))
	var pos: int = anim.add_track(Animation.TYPE_POSITION_3D)
	anim.track_set_path(pos, K.track(K.ROOT))
	anim.track_set_interpolation_type(pos, Animation.INTERPOLATION_CUBIC)
	for k: Array in c.get("root_pos", [[0.0, 0.0]]) as Array:
		anim.position_track_insert_key(pos, float(k[0]), K.BONE_REST[K.ROOT] + Vector3(0, float(k[1]), 0))
	for target: Array in [[K.ROOT, "root_squash"], [K.CHEST, "chest_breath"]]:
		var sc: int = anim.add_track(Animation.TYPE_SCALE_3D)
		anim.track_set_path(sc, K.track(int(target[0])))
		anim.track_set_interpolation_type(sc, Animation.INTERPOLATION_CUBIC)
		for k: Array in c.get(String(target[1]), [[0.0, 0.0]]) as Array:
			anim.scale_track_insert_key(sc, float(k[0]), squash(float(k[1])))
	return anim


## Volume-keeping squash (`amount` < 0) / stretch (> 0) along Y.
static func squash(amount: float) -> Vector3:
	var y: float = 1.0 + amount
	var xz: float = 1.0 / sqrt(maxf(y, 0.1))
	return Vector3(xz, y, xz)


## Cycle of two keys a / b over `length` (a at 0 and length, b at the middle), as [[t, v], …].
static func _swing(length: float, a: Variant, b: Variant) -> Array:
	return [[0.0, a], [length * 0.5, b], [length, a]]


static func _keys(clip_name: String, l: float) -> Dictionary:
	var c: Dictionary = {}
	match clip_name:
		"idle":  # breathing: chest swells, a soft sway of the head and arms
			c["chest_breath"] = _swing(l, 0.0, 0.035)
			c["root_squash"] = _swing(l, 0.0, -0.012)
			c["rot_%d" % K.HEAD] = _swing(l, Vector3(0, 0, 0), Vector3(-2, 0, 3))
			c["rot_%d" % K.ARM_L] = _swing(l, Vector3(0, 0, -6), Vector3(0, 0, -10))
			c["rot_%d" % K.ARM_R] = _swing(l, Vector3(0, 0, 6), Vector3(0, 0, 10))
		"walk":
			c["rot_%d" % K.LEG_L] = _swing(l, Vector3(28, 0, 0), Vector3(-28, 0, 0))
			c["rot_%d" % K.LEG_R] = _swing(l, Vector3(-28, 0, 0), Vector3(28, 0, 0))
			c["rot_%d" % K.ARM_L] = _swing(l, Vector3(-24, 0, -7), Vector3(24, 0, -7))
			c["rot_%d" % K.ARM_R] = _swing(l, Vector3(24, 0, 7), Vector3(-24, 0, 7))
			c["rot_%d" % K.CHEST] = _swing(l, Vector3(-3, 5, 0), Vector3(-3, -5, 0))
			c["rot_%d" % K.HIPS] = _swing(l, Vector3(0, -3, 0), Vector3(0, 3, 0))
			c["rot_%d" % K.HEAD] = [[0.0, Vector3(0, -2, 1)], [l * 0.5, Vector3(0, 2, -1)], [l, Vector3(0, -2, 1)]]
			c["root_pos"] = [[0.0, 0.0], [l * 0.25, 0.035], [l * 0.5, 0.0], [l * 0.75, 0.035], [l, 0.0]]
			c["root_squash"] = [[0.0, -0.03], [l * 0.25, 0.02], [l * 0.5, -0.03], [l * 0.75, 0.02], [l, -0.03]]
		"run":  # bouncy cartoon run: lean, big swings, squash on contact, stretch in the air
			c["rot_%d" % K.LEG_L] = _swing(l, Vector3(48, 0, 0), Vector3(-42, 0, 0))
			c["rot_%d" % K.LEG_R] = _swing(l, Vector3(-42, 0, 0), Vector3(48, 0, 0))
			c["rot_%d" % K.ARM_L] = _swing(l, Vector3(-55, 0, -14), Vector3(55, 0, -14))
			c["rot_%d" % K.ARM_R] = _swing(l, Vector3(55, 0, 14), Vector3(-55, 0, 14))
			c["rot_%d" % K.CHEST] = _swing(l, Vector3(-12, 7, 0), Vector3(-12, -7, 0))
			c["rot_%d" % K.HEAD] = _swing(l, Vector3(7, -3, 0), Vector3(7, 3, 0))
			c["root_pos"] = [[0.0, 0.0], [l * 0.25, 0.09], [l * 0.5, 0.0], [l * 0.75, 0.09], [l, 0.0]]
			c["root_squash"] = [[0.0, -0.1], [l * 0.25, 0.07], [l * 0.5, -0.1], [l * 0.75, 0.07], [l, -0.1]]
		"jump":  # stretch on take-off, tuck in the air, arms up
			c["root_squash"] = [[0.0, 0.12], [l * 0.3, 0.0], [l * 0.6, -0.03], [l, 0.0]]
			c["rot_%d" % K.LEG_L] = [[0.0, Vector3(10, 0, 0)], [l * 0.3, Vector3(38, 0, 0)], [l, Vector3(30, 0, 0)]]
			c["rot_%d" % K.LEG_R] = [[0.0, Vector3(-12, 0, 0)], [l * 0.3, Vector3(18, 0, 0)], [l, Vector3(12, 0, 0)]]
			c["rot_%d" % K.ARM_L] = [[0.0, Vector3(0, 0, -40)], [l * 0.3, Vector3(10, 0, -110)], [l, Vector3(0, 0, -85)]]
			c["rot_%d" % K.ARM_R] = [[0.0, Vector3(0, 0, 40)], [l * 0.3, Vector3(10, 0, 110)], [l, Vector3(0, 0, 85)]]
			c["rot_%d" % K.CHEST] = [[0.0, Vector3(5, 0, 0)], [l * 0.4, Vector3(-5, 0, 0)], [l, Vector3(-3, 0, 0)]]
			c["rot_%d" % K.HEAD] = [[0.0, Vector3(8, 0, 0)], [l, Vector3(2, 0, 0)]]
		"wave":  # right arm up, waving, head tilted
			var arm: Array = [[0.0, Vector3(0, 0, 8)], [0.25, Vector3(0, -10, 132)]]
			for i: int in 5:
				arm.append([0.45 + i * 0.22, Vector3(0, -10, 105 if i % 2 == 0 else 132)])
			arm.append([l, Vector3(0, 0, 8)])
			c["rot_%d" % K.ARM_R] = arm
			c["rot_%d" % K.ARM_L] = [[0.0, Vector3(0, 0, -6)], [0.3, Vector3(0, 0, -12)], [l, Vector3(0, 0, -6)]]
			c["rot_%d" % K.HEAD] = [[0.0, Vector3.ZERO], [0.3, Vector3(0, -6, -9)], [l - 0.2, Vector3(0, -6, -9)], [l, Vector3.ZERO]]
			c["rot_%d" % K.CHEST] = [[0.0, Vector3.ZERO], [0.3, Vector3(0, -8, -3)], [l - 0.2, Vector3(0, -8, -3)], [l, Vector3.ZERO]]
			c["root_squash"] = [[0.0, 0.0], [0.12, -0.04], [0.3, 0.03], [0.5, 0.0], [l, 0.0]]
		"celebrate":  # both arms up, two hops with squash and stretch
			c["rot_%d" % K.ARM_L] = [[0.0, Vector3(0, 0, -8)], [0.2, Vector3(0, 0, -130)], [0.6, Vector3(0, 0, -115)],
				[1.1, Vector3(0, 0, -130)], [1.5, Vector3(0, 0, -120)], [l, Vector3(0, 0, -8)]]
			c["rot_%d" % K.ARM_R] = [[0.0, Vector3(0, 0, 8)], [0.2, Vector3(0, 0, 130)], [0.6, Vector3(0, 0, 115)],
				[1.1, Vector3(0, 0, 130)], [1.5, Vector3(0, 0, 120)], [l, Vector3(0, 0, 8)]]
			c["root_pos"] = [[0.0, 0.0], [0.15, 0.0], [0.4, 0.22], [0.65, 0.0], [0.9, 0.22], [1.15, 0.0], [l, 0.0]]
			c["root_squash"] = [[0.0, 0.0], [0.15, -0.14], [0.4, 0.1], [0.65, -0.14], [0.9, 0.1], [1.15, -0.12],
				[1.35, 0.0], [l, 0.0]]
			c["rot_%d" % K.LEG_L] = [[0.0, Vector3.ZERO], [0.4, Vector3(25, 0, -6)], [0.65, Vector3.ZERO], [0.9, Vector3(25, 0, -6)],
				[1.15, Vector3.ZERO], [l, Vector3.ZERO]]
			c["rot_%d" % K.LEG_R] = [[0.0, Vector3.ZERO], [0.4, Vector3(25, 0, 6)], [0.65, Vector3.ZERO], [0.9, Vector3(25, 0, 6)],
				[1.15, Vector3.ZERO], [l, Vector3.ZERO]]
			c["rot_%d" % K.HEAD] = [[0.0, Vector3.ZERO], [0.4, Vector3(10, 0, 0)], [1.2, Vector3(10, 0, 0)], [l, Vector3.ZERO]]
		"clap":  # arms forward, clapping four times
			var left: Array = [[0.0, Vector3(0, 0, -6)], [0.2, Vector3(65, 0, 4)]]
			var right: Array = [[0.0, Vector3(0, 0, 6)], [0.2, Vector3(65, 0, -4)]]
			for i: int in 8:
				var closed: bool = i % 2 == 0
				left.append([0.32 + i * 0.12, Vector3(65, 0, 24 if closed else 4)])
				right.append([0.32 + i * 0.12, Vector3(65, 0, -24 if closed else -4)])
			left.append([l, Vector3(0, 0, -6)])
			right.append([l, Vector3(0, 0, 6)])
			c["rot_%d" % K.ARM_L] = left
			c["rot_%d" % K.ARM_R] = right
			c["root_squash"] = [[0.0, 0.0], [0.32, -0.03], [0.56, 0.0], [0.8, -0.03], [1.04, 0.0], [l, 0.0]]
			c["rot_%d" % K.HEAD] = [[0.0, Vector3.ZERO], [0.4, Vector3(-6, 0, 4)], [0.9, Vector3(-6, 0, -4)], [l, Vector3.ZERO]]
		"dance":  # hip sway, alternating arms, bounce
			var hips: Array = []
			var chest: Array = []
			var arm_l: Array = []
			var arm_r: Array = []
			var pos: Array = []
			var squash_keys: Array = []
			for i: int in 5:
				var t: float = l * float(i) / 4.0
				var s: float = 1.0 if i % 2 == 0 else -1.0
				hips.append([t, Vector3(0, 0, 10 * s) if i < 4 else Vector3.ZERO])
				chest.append([t, Vector3(0, 0, -8 * s) if i < 4 else Vector3.ZERO])
				arm_l.append([t, Vector3(0, 0, -115 if s > 0 else -30) if i < 4 else Vector3(0, 0, -6)])
				arm_r.append([t, Vector3(0, 0, 30 if s > 0 else 110) if i < 4 else Vector3(0, 0, 6)])
				pos.append([t, 0.0])
				squash_keys.append([t, -0.06 if i < 4 else 0.0])
				if i < 4:
					pos.append([t + l / 8.0, 0.06])
					squash_keys.append([t + l / 8.0, 0.05])
			c["rot_%d" % K.HIPS] = hips
			c["rot_%d" % K.CHEST] = chest
			c["rot_%d" % K.ARM_L] = arm_l
			c["rot_%d" % K.ARM_R] = arm_r
			c["root_pos"] = pos
			c["root_squash"] = squash_keys
		"thank":  # friendly bow
			c["rot_%d" % K.CHEST] = [[0.0, Vector3.ZERO], [0.45, Vector3(-28, 0, 0)], [0.9, Vector3(-28, 0, 0)], [l, Vector3.ZERO]]
			c["rot_%d" % K.HEAD] = [[0.0, Vector3.ZERO], [0.45, Vector3(-10, 0, 0)], [0.9, Vector3(-10, 0, 0)], [l, Vector3.ZERO]]
			c["rot_%d" % K.ARM_L] = [[0.0, Vector3(0, 0, -6)], [0.45, Vector3(18, 0, 10)], [0.9, Vector3(18, 0, 10)],
				[l, Vector3(0, 0, -6)]]
			c["rot_%d" % K.ARM_R] = [[0.0, Vector3(0, 0, 6)], [0.45, Vector3(18, 0, -10)], [0.9, Vector3(18, 0, -10)],
				[l, Vector3(0, 0, 6)]]
		"apologize":  # deeper bow, head down
			c["rot_%d" % K.CHEST] = [[0.0, Vector3.ZERO], [0.5, Vector3(-36, 0, 0)], [1.3, Vector3(-36, 0, 0)], [l, Vector3.ZERO]]
			c["rot_%d" % K.HEAD] = [[0.0, Vector3.ZERO], [0.5, Vector3(-20, 0, 0)], [1.3, Vector3(-20, 0, 0)], [l, Vector3.ZERO]]
			c["rot_%d" % K.ARM_L] = [[0.0, Vector3(0, 0, -6)], [0.5, Vector3(22, 0, 14)], [1.3, Vector3(22, 0, 14)],
				[l, Vector3(0, 0, -6)]]
			c["rot_%d" % K.ARM_R] = [[0.0, Vector3(0, 0, 6)], [0.5, Vector3(22, 0, -14)], [1.3, Vector3(22, 0, -14)],
				[l, Vector3(0, 0, 6)]]
			c["root_squash"] = [[0.0, 0.0], [0.5, -0.04], [1.3, -0.04], [l, 0.0]]
	return c
