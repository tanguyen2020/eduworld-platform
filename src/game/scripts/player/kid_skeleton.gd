class_name KidSkeleton
extends RefCounted
## The `kid_base` chibi rig of the avatars (F068, ADR-0103, CHARACTER_SYSTEM §5.2): one definition shared by the bake
## recipes (bone weights of the body/outfit meshes, `tools/art/character_art.gd`), the animation clips
## (`tools/art/character_anim.gd`) and the Skeleton3D of `scenes/character/AvatarRig.tscn` (checked by
## `tests/unit/art/test_kid_rig.gd`). Body space: feet at y = 0, facing -Z, +X is the character's right side.
## Chibi proportions: the head is about half of the height (ADR-0103: 1/2–1/3), short limbs, round body. Display only.

const ROOT: int = 0
const HIPS: int = 1
const CHEST: int = 2
const HEAD: int = 3
const ARM_L: int = 4
const ARM_R: int = 5
const LEG_L: int = 6
const LEG_R: int = 7
const BONE_NAMES: PackedStringArray = ["root", "hips", "chest", "head", "arm_l", "arm_r", "leg_l", "leg_r"]
const BONE_PARENTS: PackedInt32Array = [-1, 0, 1, 2, 2, 2, 1, 1]
## Rest position of every bone in body space (the rest rotation is identity).
const BONE_REST: Array[Vector3] = [
	Vector3(0, 0, 0), Vector3(0, 0.3, 0), Vector3(0, 0.44, 0), Vector3(0, 0.62, 0),
	Vector3(-0.17, 0.555, 0), Vector3(0.17, 0.555, 0), Vector3(-0.085, 0.3, 0), Vector3(0.085, 0.3, 0),
]
## Head ellipsoid (body space) — the face, hair and head items are placed on it.
const HEAD_CENTER: Vector3 = Vector3(0, 0.91, 0)
const HEAD_RADII: Vector3 = Vector3(0.31, 0.29, 0.285)
## Node path of the Skeleton3D from the AvatarRig root (animation tracks are "<path>:<bone>").
const SKELETON_PATH: String = "Bob/Skeleton3D"
## Folder of the baked character meshes and clips (assets/LICENSES.md: first-party MIT).
const MODELS_DIR: String = "res://assets/models/chr"
## Facial expressions switchable through `AvatarBuilder.set_expression()` (ADR-0103 "3D Animated Cartoon"), plus
## the internal `blink` frame of the neutral face.
const EXPRESSIONS: PackedStringArray = ["neutral", "happy", "surprised", "trying"]
const BLINK: String = "blink"
const DEFAULT_EXPRESSION: String = "neutral"
## Item slot anchors: bone they follow and offset from that bone (body-space axes).
const ANCHORS: Dictionary = {
	"hair": [HEAD, Vector3(0, 0.29, 0)],
	"head": [HEAD, Vector3(0, 0.6, 0)],
	"face": [HEAD, Vector3(0, 0.27, -0.3)],
	"back": [CHEST, Vector3(0, 0.0, 0.2)],
	"hand": [ARM_R, Vector3(0.065, -0.235, -0.01)],
}


## Rest transform of bone `i` relative to its parent.
static func rest_local(i: int) -> Transform3D:
	var parent: int = BONE_PARENTS[i]
	var origin: Vector3 = BONE_REST[i] - (BONE_REST[parent] if parent >= 0 else Vector3.ZERO)
	return Transform3D(Basis.IDENTITY, origin)


static func track(bone: int) -> NodePath:
	return NodePath("%s:%s" % [SKELETON_PATH, BONE_NAMES[bone]])


static func is_expression(expression: String) -> bool:
	return expression in EXPRESSIONS or expression == BLINK


## Baked face mesh of an expression (F068, tools/art/character_art.gd).
static func face_path(expression: String) -> String:
	return "%s/chr_face_%s_a.res" % [MODELS_DIR, expression]


## Baked animation clip (F068, tools/art/character_anim.gd).
static func clip_path(clip_name: String) -> String:
	return "%s/anim_kid_%s_a.res" % [MODELS_DIR, clip_name]
