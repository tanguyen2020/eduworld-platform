extends RefCounted
## Recipes of the chibi anime cartoon characters (F068, ADR-0103; WORLD_DESIGN §11 "Nhân vật", CHARACTER_SYSTEM §5):
## - 3D Chibi: head about half of the height (KidSkeleton.HEAD_RADII), short limbs, round body;
## - Stylized Anime 3D: big eyes with highlights, block-shaped hair locks, blush; cel shading + ink outline come from
##   the shared toon presets;
## - Stylized 3D Cartoon: rounded, smooth-shaded forms (builder defaults), no weapons.
## The avatar body and outfits are skinned to the `kid_base` rig (KidSkeleton); faces are separate meshes per
## expression in head-bone space (AvatarBuilder.set_expression); hair and accessories are authored relative to their
## slot anchor (KidSkeleton.ANCHORS). Skin, hair colour and item tints are the per-avatar `ALBEDO` faces; face, shoes
## and trousers come from the art palette tokens. NPC bodies are static (unskinned) with the neutral face baked in.
## SPDX-License-Identifier: MIT — EduWorld first-party (ADR-0102). Bake-time only.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const A: String = LowPoly.ALBEDO
const K := preload("res://scripts/player/kid_skeleton.gd")

const HEAD_CENTER: Vector3 = KidSkeleton.HEAD_CENTER
const HEAD_RADII: Vector3 = KidSkeleton.HEAD_RADII
const HAIR_STYLES: PackedStringArray = ["short", "bob", "ponytail", "braids", "curly", "spiky"]
const OUTFIT_STYLES: PackedStringArray = ["school", "casual", "sport", "star", "rain", "explorer"]
const NPC_LOOKS: PackedStringArray = ["farmer", "postie", "scholar"]
## Face features in face coordinates (u = body X, v = height from the head centre).
const EYE_U: float = 0.108
const EYE_V: float = -0.02
const MOUTH_V: float = -0.14
const CHEEK_U: float = 0.175
const CHEEK_V: float = -0.095
## Longest edge of a face decal triangle before it is projected on the head (keeps it on the curved surface).
const DECAL_EDGE: float = 0.04


# ---------- body ----------

## Skinned chibi kid body (no face: AvatarBuilder adds the expression mesh). `skin` = ALBEDO for avatars.
static func body(skin: String = A, shirt: String = "shirt_white", pants: String = "pants") -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	m.skinned = true
	_body(m, skin, shirt, pants)
	return m.build()


static func _body(m: LowPoly, skin: String, shirt: String, pants: String) -> void:
	for side: int in [K.LEG_L, K.LEG_R]:
		var x: float = K.BONE_REST[side].x
		m.bone = side
		m.box(Vector3(x, 0.045, -0.03), Vector3(0.125, 0.09, 0.19), "shoe", "shoe")
		m.cylinder(Vector3(x, 0.08, 0), 0.058, 0.064, 0.24, 8, pants, "", false)
	m.bone = K.HIPS
	m.skin_fn = _torso_weights
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.06, 0.215), Vector2(0.15, 0.24), Vector2(0.183, 0.29),
		Vector2(0.192, 0.33)]), 12, pants)
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.192, 0.33), Vector2(0.195, 0.39), Vector2(0.187, 0.46),
		Vector2(0.163, 0.54), Vector2(0.12, 0.6), Vector2(0.055, 0.635)]), 12, shirt)
	m.skin_fn = Callable()
	for side: int in [K.ARM_L, K.ARM_R]:
		m.bone = side
		var s: float = signf(K.BONE_REST[side].x)
		var shoulder: Vector3 = K.BONE_REST[side]
		var elbow: Vector3 = shoulder + Vector3(s * 0.035, -0.11, 0.0)
		var hand: Vector3 = shoulder + Vector3(s * 0.065, -0.225, -0.01)
		m.blob(shoulder, Vector3(0.058, 0.058, 0.058), shirt, 0)
		_limb(m, shoulder, elbow, 0.056, 0.05, shirt)
		_limb(m, elbow, hand, 0.046, 0.044, skin)
		m.blob(hand + Vector3(0, -0.02, 0), Vector3(0.056, 0.058, 0.052), skin, 0)
	m.bone = K.HEAD
	m.blob(HEAD_CENTER, HEAD_RADII, skin, 2)
	for s: float in [-1.0, 1.0]:
		m.blob(HEAD_CENTER + Vector3(s * 0.3, -0.035, 0.02), Vector3(0.045, 0.065, 0.04), skin, 0)


## Torso vertices blend from the hips (below the belly) to the chest (above it).
static func _torso_weights(p: Vector3) -> Array:
	var t: float = clampf(inverse_lerp(0.36, 0.48, p.y), 0.0, 1.0)
	return [PackedInt32Array([K.HIPS, K.CHEST, 0, 0]), PackedFloat32Array([1.0 - t, t, 0.0, 0.0])]


static func _limb(m: LowPoly, a: Vector3, b: Vector3, ra: float, rb: float, col: String) -> void:
	var dir: Vector3 = b - a
	var length: float = dir.length()
	var up: Vector3 = dir / length
	var side: Vector3 = up.cross(Vector3.FORWARD).normalized()
	var fwd: Vector3 = side.cross(up)
	m.push(Transform3D(Basis(side, up, fwd), a))
	m.cylinder(Vector3.ZERO, ra, rb, length, 8, col, "", false)
	m.pop()


# ---------- faces (head-bone space) ----------

## Face of one expression (KidSkeleton.EXPRESSIONS + blink) in head-bone space: anime eyes, brows, mouth, blush
## projected on the head ellipsoid. Drawn without outline (toon_no_outline).
static func face(expression: String) -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	m.push(Transform3D(Basis.IDENTITY, -K.BONE_REST[K.HEAD]))
	_face(m, expression)
	m.pop()
	return m.build()


static func _face(m: LowPoly, expression: String) -> void:
	for s: float in [-1.0, 1.0]:
		_decal(m, _ellipse(Vector2(s * CHEEK_U, CHEEK_V), Vector2(0.048, 0.026), 14), "cheek", 0.004)
		var c: Vector2 = Vector2(s * EYE_U, EYE_V)
		match expression:
			"happy":  # closed smiling eyes ^ ^
				_decal(m, _arc_band(c + Vector2(0, -0.02), Vector2(0.05, 0.05), 0.2, PI - 0.2, 0.022, 10), "eye", 0.008)
				_brow(m, s, 0.012, 0.0)
			"surprised":  # round eyes, small pupils
				_decal(m, _ellipse(c, Vector2(0.05, 0.062), 18), "eye_white", 0.005)
				_decal(m, _ellipse(c + Vector2(0, -0.004), Vector2(0.026, 0.034), 14), "eye", 0.008)
				_decal(m, _ellipse(c + Vector2(-0.01, 0.012), Vector2(0.01, 0.011), 8), "eye_light", 0.011)
				_decal(m, _arc_band(c, Vector2(0.056, 0.068), 0.35, PI - 0.35, 0.012, 10), "eye", 0.01)
				_brow(m, s, 0.04, 0.0)
			"trying":  # squeezed > < eyes
				var tip: Vector2 = c + Vector2(-s * 0.03, 0.0)
				var back: float = s * 0.03
				_decal(m, _polyline_band([c + Vector2(back, 0.036), tip, c + Vector2(back, -0.036)], 0.02), "eye", 0.008)
				_brow(m, s, -0.004, -0.35)
			"blink":  # neutral face with the eyes shut for a frame
				_decal(m, _arc_band(c + Vector2(0, 0.03), Vector2(0.05, 0.035), PI + 0.25, TAU - 0.25, 0.016, 10), "eye", 0.008)
				_brow(m, s, 0.0, 0.0)
			_:  # neutral: big anime eyes with highlights
				_decal(m, _ellipse(c, Vector2(0.05, 0.064), 18), "eye_white", 0.004)
				_decal(m, _ellipse(c + Vector2(0, -0.006), Vector2(0.043, 0.058), 18), "eye", 0.007)
				_decal(m, _ellipse(c + Vector2(0, -0.03), Vector2(0.032, 0.024), 14), "eye_iris", 0.0095)
				_decal(m, _ellipse(c + Vector2(-s * 0.012 - 0.006, 0.022), Vector2(0.017, 0.019), 12), "eye_light", 0.012)
				_decal(m, _ellipse(c + Vector2(s * 0.016, -0.028), Vector2(0.008, 0.008), 8), "eye_light", 0.012)
				_decal(m, _arc_band(c + Vector2(0, -0.004), Vector2(0.055, 0.066), 0.15, PI - 0.15, 0.014, 12), "eye", 0.011)
				_brow(m, s, 0.0, 0.0)
	match expression:
		"happy":  # open "D" smile
			_decal(m, _half_disc(Vector2(0, MOUTH_V + 0.012), Vector2(0.048, 0.042), 12), "mouth", 0.006)
			_decal(m, _ellipse(Vector2(0, MOUTH_V - 0.012), Vector2(0.022, 0.012), 10), "flower_pink", 0.009)
		"surprised":
			_decal(m, _ellipse(Vector2(0, MOUTH_V - 0.005), Vector2(0.02, 0.026), 12), "mouth", 0.006)
		"trying":
			_decal(m, _polyline_band([Vector2(-0.035, MOUTH_V), Vector2(-0.012, MOUTH_V + 0.01), Vector2(0.012, MOUTH_V),
				Vector2(0.035, MOUTH_V + 0.01)], 0.012), "mouth", 0.006)
			_decal(m, _drop(Vector2(-0.215, 0.115), 0.024), "wind", 0.006)
		_:
			_decal(m, _arc_band(Vector2(0, MOUTH_V + 0.03), Vector2(0.034, 0.03), PI + 0.55, TAU - 0.55, 0.011, 10), "mouth", 0.006)


## Short soft brow above eye `s` (-1 left, +1 right), raised by `lift`, tilted by `tilt` (radians, + = outer end up).
static func _brow(m: LowPoly, s: float, lift: float, tilt: float) -> void:
	var c: Vector2 = Vector2(s * EYE_U, EYE_V + 0.098 + lift)
	var pts: PackedVector2Array = _arc_band(Vector2.ZERO, Vector2(0.034, 0.016), 0.5, PI - 0.5, 0.009, 6)
	var out: PackedVector2Array = PackedVector2Array()
	for p: Vector2 in pts:
		out.append(c + p.rotated(s * tilt))
	_decal(m, out, "eye", 0.006)


## A 2D shape (face coordinates) projected on the head ellipsoid, lifted `lift` along the surface normal; triangles
## are subdivided first so the decal follows the curve.
static func _decal(m: LowPoly, pts: PackedVector2Array, color: String, lift: float) -> void:
	var idx: PackedInt32Array = Geometry2D.triangulate_polygon(pts)
	for i: int in range(0, idx.size(), 3):
		_decal_tri(m, pts[idx[i]], pts[idx[i + 1]], pts[idx[i + 2]], color, lift)


static func _decal_tri(m: LowPoly, a: Vector2, b: Vector2, c: Vector2, color: String, lift: float) -> void:
	if maxf(a.distance_to(b), maxf(b.distance_to(c), c.distance_to(a))) > DECAL_EDGE:
		var ab: Vector2 = (a + b) * 0.5
		var bc: Vector2 = (b + c) * 0.5
		var ca: Vector2 = (c + a) * 0.5
		for t: Array in [[a, ab, ca], [ab, b, bc], [ca, bc, c], [ab, bc, ca]]:
			_decal_tri(m, t[0] as Vector2, t[1] as Vector2, t[2] as Vector2, color, lift)
		return
	m.tri_out(_on_head(a, lift), _on_head(b, lift), _on_head(c, lift), color, HEAD_CENTER)


## Point of the front (-Z) of the head ellipsoid at face coordinates `p`, lifted along the normal.
static func _on_head(p: Vector2, lift: float) -> Vector3:
	var r: Vector3 = HEAD_RADII
	var k: float = 1.0 - (p.x / r.x) * (p.x / r.x) - (p.y / r.y) * (p.y / r.y)
	var z: float = -r.z * sqrt(maxf(k, 0.0))
	var local: Vector3 = Vector3(p.x, p.y, z)
	var normal: Vector3 = Vector3(local.x / (r.x * r.x), local.y / (r.y * r.y), local.z / (r.z * r.z)).normalized()
	return HEAD_CENTER + local + normal * lift


static func _ellipse(c: Vector2, r: Vector2, n: int) -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array()
	for i: int in n:
		var a: float = TAU * float(i) / float(n)
		pts.append(c + Vector2(cos(a) * r.x, sin(a) * r.y))
	return pts


## Crescent band along an elliptic arc from angle `a0` to `a1` (counter-clockwise), `thick` wide, tapered ends.
static func _arc_band(c: Vector2, r: Vector2, a0: float, a1: float, thick: float, n: int) -> PackedVector2Array:
	var outer: PackedVector2Array = PackedVector2Array()
	var inner: PackedVector2Array = PackedVector2Array()
	for i: int in n + 1:
		var t: float = float(i) / float(n)
		var a: float = lerpf(a0, a1, t)
		var w: float = thick * (0.35 + 0.65 * sin(PI * t))
		var d: Vector2 = Vector2(cos(a), sin(a))
		outer.append(c + Vector2(d.x * r.x, d.y * r.y) + d * w * 0.5)
		inner.append(c + Vector2(d.x * r.x, d.y * r.y) - d * w * 0.5)
	inner.reverse()
	outer.append_array(inner)
	return outer


static func _half_disc(c: Vector2, r: Vector2, n: int) -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array()
	for i: int in n + 1:
		var a: float = PI + PI * float(i) / float(n)
		pts.append(c + Vector2(cos(a) * r.x, sin(a) * r.y))
	return pts


## Thick polyline (outline polygon) through `points`.
static func _polyline_band(points: Array, thick: float) -> PackedVector2Array:
	var left: PackedVector2Array = PackedVector2Array()
	var right: PackedVector2Array = PackedVector2Array()
	for i: int in points.size():
		var p: Vector2 = points[i] as Vector2
		var prev: Vector2 = points[maxi(i - 1, 0)] as Vector2
		var next: Vector2 = points[mini(i + 1, points.size() - 1)] as Vector2
		var dir: Vector2 = (next - prev).normalized()
		var n: Vector2 = Vector2(-dir.y, dir.x) * thick * 0.5
		left.append(p + n)
		right.append(p - n)
	right.reverse()
	left.append_array(right)
	return left


## Sweat drop (cartoon effort sign).
static func _drop(c: Vector2, size: float) -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array([c + Vector2(0, size * 1.4)])
	for i: int in 9:
		var a: float = lerpf(-0.25 * PI, 1.25 * PI, float(i) / 8.0)
		pts.append(c + Vector2(cos(a), -sin(a)) * size * 0.75)
	return pts


# ---------- hair (relative to the Hair anchor = head centre) ----------

## Hair shell that leaves the face open: an ellipsoid whose front-lower vertices are pulled behind the face.
static func _shell(m: LowPoly, radii: Vector3, offset: Vector3, brow: float, open_width: float, color: String = A) -> void:
	var t: Array = LowPoly._icosphere(2 + m.blob_detail)
	var verts: PackedVector3Array = t[0]
	var faces: PackedInt32Array = t[1]
	var moved: PackedVector3Array = PackedVector3Array()
	for v: Vector3 in verts:
		var p: Vector3 = Vector3(v.x * radii.x, v.y * radii.y, v.z * radii.z) + offset
		if p.z < 0.0 and p.y < brow:
			var k: float = clampf(absf(p.x) / open_width, 0.0, 1.0)
			p.z *= lerpf(0.05, 0.85, k * k)
		moved.append(p)
	for i: int in range(0, faces.size(), 3):
		m.tri(moved[faces[i]], moved[faces[i + 1]], moved[faces[i + 2]], color)


## One block-shaped anime lock along `path` (root → tip), `width` wide and `thick` deep at the root, tapering to a
## point; its flat side faces away from `center` (the head).
static func _lock(m: LowPoly, path: Array, width: float, thick: float, center: Vector3 = Vector3.ZERO,
		color: String = A) -> void:
	var rings: Array[PackedVector3Array] = []
	var n: int = path.size()
	for i: int in n - 1:
		var p: Vector3 = path[i] as Vector3
		var tangent: Vector3 = ((path[i + 1] as Vector3) - (path[maxi(i - 1, 0)] as Vector3)).normalized()
		var out: Vector3 = p - center
		out = (out - tangent * out.dot(tangent)).normalized()
		var side: Vector3 = tangent.cross(out).normalized()
		var k: float = 1.0 - float(i) / float(n - 1)
		var w: float = width * (0.45 + 0.55 * k)
		var d: float = thick * (0.5 + 0.5 * k)
		var ring: PackedVector3Array = PackedVector3Array()
		for j: int in 6:
			var a: float = TAU * float(j) / 6.0
			ring.append(p + side * cos(a) * w * 0.5 + out * sin(a) * d * 0.5)
		rings.append(ring)
	for i: int in rings.size() - 1:
		for j: int in 6:
			var j1: int = (j + 1) % 6
			m.quad_out(rings[i][j], rings[i + 1][j], rings[i + 1][j1], rings[i][j1], color,
					((path[i] as Vector3) + (path[i + 1] as Vector3)) * 0.5)
	var tip: Vector3 = path[n - 1] as Vector3
	var last: PackedVector3Array = rings[rings.size() - 1]
	for j: int in 6:
		m.tri_out(last[j], tip, last[(j + 1) % 6], color, path[n - 2] as Vector3)
	var first: PackedVector3Array = rings[0]
	var cap: Vector3 = (path[0] as Vector3) - ((path[1] as Vector3) - (path[0] as Vector3)) * 0.2
	for j: int in 6:
		m.tri_out(first[j], first[(j + 1) % 6], cap, color, path[1] as Vector3)


## Fringe (bangs) of `count` locks from the hairline over the forehead down to `length` below it.
static func _fringe(m: LowPoly, count: int, length: float, spread: float = 0.23, blunt: bool = false) -> void:
	var r: Vector3 = HEAD_RADII
	for k: int in count:
		var t: float = float(k) / float(maxi(count - 1, 1))
		var x: float = lerpf(-spread, spread, t)
		var top: Vector3 = Vector3(x * 0.6, 0.27, -0.12)
		var mid: Vector3 = Vector3(x * 0.95, 0.19, -r.z * 0.95)
		var drop: float = length * (1.0 if blunt else lerpf(0.75, 1.15, absf(sin(float(k) * 2.3))))
		var tip: Vector3 = Vector3(x * 1.05 + (0.0 if blunt else signf(x) * 0.012), 0.19 - drop, -r.z * 1.04 - 0.02)
		_lock(m, [top, mid, (mid + tip) * 0.5 + Vector3(0, 0, -0.012), tip], 0.105 if blunt else 0.095, 0.05)


## Side locks in front of the ears, down to `bottom` (y relative to the head centre).
static func _side_locks(m: LowPoly, bottom: float, curl: float = 0.0) -> void:
	for s: float in [-1.0, 1.0]:
		var top: Vector3 = Vector3(s * 0.26, 0.17, -0.12)
		var mid: Vector3 = Vector3(s * 0.315, 0.0, -0.13)
		var tip: Vector3 = Vector3(s * (0.3 - curl), bottom, -0.15 - curl * 0.5)
		_lock(m, [top, mid, (mid + tip) * 0.5 + Vector3(s * 0.012, 0, 0), tip], 0.1, 0.055)


static func hair(style: String) -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 6830 + HAIR_STYLES.find(style)
	var r: Vector3 = HEAD_RADII
	match style:
		"short":
			_shell(m, r + Vector3(0.025, 0.03, 0.03), Vector3(0, 0.025, 0.015), 0.12, 0.27)
			_fringe(m, 6, 0.1)
			_side_locks(m, -0.04)
			# ahoge: the springy lock on top
			_lock(m, [Vector3(0.02, 0.3, 0.0), Vector3(0.03, 0.4, -0.03), Vector3(0.0, 0.45, -0.1), Vector3(-0.04, 0.43, -0.14)],
					0.05, 0.03, Vector3(0, 0.2, 0))
			for x: float in [-0.12, 0.0, 0.12]:
				_lock(m, [Vector3(x, 0.05, 0.27), Vector3(x * 1.1, -0.08, 0.3), Vector3(x * 1.15, -0.16, 0.27)], 0.12, 0.05)
		"bob":
			_shell(m, r + Vector3(0.04, 0.04, 0.045), Vector3(0, 0.0, 0.02), 0.12, 0.26)
			_fringe(m, 7, 0.12, 0.24, true)
			_side_locks(m, -0.2, 0.04)
			for k: int in 5:
				var a: float = lerpf(-1.1, 1.1, float(k) / 4.0)
				var root: Vector3 = Vector3(sin(a) * 0.3, 0.05, cos(a) * 0.28)
				_lock(m, [root, root * Vector3(1.08, 1, 1.08) + Vector3(0, -0.13, 0),
					root * Vector3(0.95, 1, 0.95) + Vector3(0, -0.23, 0)], 0.17, 0.06)
		"ponytail":
			_shell(m, r + Vector3(0.022, 0.03, 0.03), Vector3(0, 0.025, 0.015), 0.12, 0.27)
			_fringe(m, 6, 0.1)
			_side_locks(m, -0.06)
			m.blob(Vector3(0, 0.16, 0.31), Vector3(0.055, 0.055, 0.045), "cloth_red", 0)
			for x: float in [-0.04, 0.0, 0.04]:
				_lock(m, [Vector3(x, 0.17, 0.33), Vector3(x * 1.5, 0.12, 0.45), Vector3(x * 1.8, -0.06, 0.5),
					Vector3(x * 1.2, -0.28, 0.44)], 0.13, 0.09)
		"braids":
			_shell(m, r + Vector3(0.022, 0.03, 0.03), Vector3(0, 0.025, 0.015), 0.12, 0.27)
			_fringe(m, 6, 0.11)
			for s: float in [-1.0, 1.0]:
				for k: int in 4:
					m.blob(Vector3(s * 0.27, -0.12 - k * 0.085, 0.07), Vector3(0.056, 0.058, 0.056), A, 0)
				m.blob(Vector3(s * 0.27, -0.46, 0.07), Vector3(0.04, 0.03, 0.04), "cloth_red", 0)
		"curly":
			_shell(m, r + Vector3(0.03, 0.035, 0.035), Vector3(0, 0.03, 0.02), 0.13, 0.27)
			for k: int in 16:
				var a: float = TAU * float(k) / 16.0
				var y: float = 0.14 + 0.1 * sin(a * 2.0 + 0.5)
				var p: Vector3 = Vector3(cos(a) * 0.29, y, sin(a) * 0.28 + 0.03)
				if p.z < -0.15 and y < 0.2:
					p.y += 0.1
				m.blob(p, Vector3(0.085, 0.085, 0.085) * rng.randf_range(0.9, 1.15), A, 0)
			for x: float in [-0.13, 0.0, 0.13]:
				m.blob(Vector3(x, 0.22, -0.25), Vector3(0.075, 0.065, 0.06), A, 0)
			m.blob(Vector3(0, 0.33, 0.02), Vector3(0.15, 0.08, 0.15), A, 0)
		"spiky":
			_shell(m, r + Vector3(0.02, 0.025, 0.025), Vector3(0, 0.03, 0.015), 0.12, 0.27)
			_fringe(m, 5, 0.09, 0.2)
			_side_locks(m, -0.02)
			for k: int in 7:
				var a: float = lerpf(-1.2, 1.2, float(k) / 6.0)
				var base: Vector3 = Vector3(sin(a) * 0.17, 0.25, cos(a) * 0.1 + 0.03)
				var tip: Vector3 = base + Vector3(sin(a) * 0.13, 0.17, 0.09 + absf(sin(a)) * 0.05)
				_lock(m, [base, (base + tip) * 0.5 + Vector3(0, 0.02, 0), tip], 0.13, 0.08, Vector3(0, 0.1, 0))
	return m.build()


# ---------- outfits (skinned, Outfit anchor = the Skeleton3D) ----------

static func outfit(style: String) -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	m.skinned = true
	var long: bool = style == "rain"
	var bottom: float = 0.15 if long else 0.27
	m.skin_fn = _torso_weights
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.215 if long else 0.2, bottom), Vector2(0.207, 0.33),
		Vector2(0.209, 0.39), Vector2(0.2, 0.46), Vector2(0.176, 0.54), Vector2(0.132, 0.6), Vector2(0.075, 0.64)]), 12, A)
	m.skin_fn = Callable()
	for side: int in [K.ARM_L, K.ARM_R]:
		m.bone = side
		var s: float = signf(K.BONE_REST[side].x)
		var shoulder: Vector3 = K.BONE_REST[side]
		var cuff: Vector3 = shoulder + Vector3(s * 0.035, -0.11 if not long else -0.18, 0.0)
		m.blob(shoulder, Vector3(0.068, 0.066, 0.068), A, 0)
		_limb(m, shoulder, cuff, 0.066, 0.062, A)
	m.bone = K.CHEST
	match style:
		"school":
			# white collar and the red scarf (khăn quàng đỏ) of the school uniform
			m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.15, 0.585), Vector2(0.11, 0.625), Vector2(0.07, 0.645)]), 12,
					"shirt_white")
			m.push(Transform3D(Basis(Vector3.RIGHT, -0.35), Vector3(0, 0.578, -0.17)))
			m.blob(Vector3(0, 0, -0.008), Vector3(0.042, 0.036, 0.03), "cloth_red", 0)
			m.extrude(PackedVector2Array([Vector2(-0.025, -0.01), Vector2(-0.06, -0.13), Vector2(-0.02, -0.12)]), 0.02, "cloth_red")
			m.extrude(PackedVector2Array([Vector2(0.025, -0.01), Vector2(0.02, -0.12), Vector2(0.06, -0.13)]), 0.02, "cloth_red")
			m.pop()
		"casual":
			m.box(Vector3(0.08, 0.47, -0.2), Vector3(0.07, 0.07, 0.02), "wood")
		"sport":
			m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.211, 0.4), Vector2(0.208, 0.44)]), 12, "shirt_white")
		"star":
			m.push(Transform3D(Basis(Vector3.UP, PI).scaled(Vector3(0.075, 0.075, 1)), Vector3(0, 0.46, -0.2)))
			m.extrude(_star(), 0.02, "lantern_glow")
			m.pop()
		"rain":
			for y: float in [0.52, 0.42, 0.32, 0.22]:
				m.blob(Vector3(0, y, -0.212), Vector3(0.017, 0.017, 0.01), "lantern_glow", 0)
			m.bone = K.HEAD
			m.blob(Vector3(0, 0.66, 0.15), Vector3(0.2, 0.1, 0.11), A, 1)
		"explorer":
			for x: float in [-0.085, 0.085]:
				m.box(Vector3(x, 0.38, -0.2), Vector3(0.075, 0.065, 0.025), "wood")
			_limb(m, Vector3(-0.15, 0.6, -0.09), Vector3(0.18, 0.33, -0.15), 0.016, 0.016, "wood_dark")
	return m.build()


static func _star() -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array()
	for i: int in 10:
		var a: float = PI * 0.5 + TAU * float(i) / 10.0
		pts.append(Vector2(cos(a), sin(a)) * (1.0 if i % 2 == 0 else 0.45))
	return pts


# ---------- accessories (relative to their slot anchor, KidSkeleton.ANCHORS) ----------

static func accessory(code: String) -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	match code:
		"acc_cap_star":  # Head anchor: top of the head
			m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.335, -0.13), Vector2(0.32, -0.05), Vector2(0.25, 0.03),
				Vector2(0.12, 0.07), Vector2(0.0, 0.08)]), 14, A)
			m.box(Vector3(0, -0.12, -0.32), Vector3(0.36, 0.035, 0.2), A)
			m.push(Transform3D(Basis(Vector3.UP, PI).scaled(Vector3(0.065, 0.065, 1)), Vector3(0, -0.06, -0.3)))
			m.extrude(_star(), 0.02, "lantern_glow")
			m.pop()
		"acc_glasses_round":  # Face anchor: front of the eyes
			for s: float in [-1.0, 1.0]:
				m.push(Transform3D(Basis.IDENTITY, Vector3(s * EYE_U, 0, 0)))
				_ring_xy(m, 0.068, 0.011, 16, "eye")
				m.pop()
			m.box(Vector3(0, 0.012, 0), Vector3(0.05, 0.014, 0.014), "eye")
		"acc_backpack_owl":  # Back anchor
			m.box(Vector3(0, 0, 0.05), Vector3(0.27, 0.28, 0.13), A)
			m.blob(Vector3(0, 0.14, 0.05), Vector3(0.135, 0.06, 0.068), A, 1)
			for s: float in [-1.0, 1.0]:
				m.blob(Vector3(s * 0.058, 0.05, 0.118), Vector3(0.042, 0.042, 0.014), "eye_white", 0)
				m.blob(Vector3(s * 0.058, 0.05, 0.13), Vector3(0.02, 0.02, 0.01), "eye", 0)
				m.box(Vector3(s * 0.1, 0.05, -0.07), Vector3(0.04, 0.26, 0.036), "wood_dark")
			m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.0, 0.12)))
			m.extrude(PackedVector2Array([Vector2(-0.022, 0.0), Vector2(0.022, 0.0), Vector2(0.0, -0.036)]), 0.03, "lantern_glow")
			m.pop()
		"acc_flag_small":  # Hand anchor
			m.cylinder(Vector3(0, -0.05, 0), 0.012, 0.012, 0.46, 6, "wood")
			m.blob(Vector3(0, 0.42, 0), Vector3(0.022, 0.022, 0.022), "lantern_glow", 0)
			m.push(Transform3D(Basis(Vector3.UP, PI * 0.5), Vector3(0, 0.3, 0)))
			m.extrude(PackedVector2Array([Vector2(0, 0.11), Vector2(0.22, 0.05), Vector2(0, -0.01)]), 0.012, A)
			m.pop()
	return m.build()


static func _ring_xy(m: LowPoly, big: float, small: float, segs: int, col: String) -> void:
	for i: int in segs:
		var a0: float = TAU * float(i) / float(segs)
		var a1: float = TAU * float(i + 1) / float(segs)
		var d0: Vector3 = Vector3(cos(a0), sin(a0), 0)
		var d1: Vector3 = Vector3(cos(a1), sin(a1), 0)
		m.quad(d0 * (big - small), d1 * (big - small), d1 * (big + small), d0 * (big + small), col)
		m.quad(d0 * (big + small), d1 * (big + small), d1 * (big - small), d0 * (big - small), col)


# ---------- NPC bodies (static, whole look in the palette, neutral face baked in) ----------

static func npc(look: String) -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	# NPCs stand around the hub (several per zone): base blob detail keeps them in the NPC budget (F068).
	m.blob_detail = 0
	var c: Vector3 = HEAD_CENTER
	match look:
		"farmer":  # áo bà ba + nón lá
			_body(m, "skin_npc", "cloth_blue", "wood_dark")
			_face(m, "neutral")
			_hair_cap(m, "eye")
			m.push(Transform3D(Basis.IDENTITY, c + Vector3(0, 0.21, 0)))
			m.cylinder(Vector3.ZERO, 0.56, 0.0, 0.3, 18, "basket", "", false)
			m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.56, -0.012), Vector2(0.56, 0.004)]), 18, "wood")
			m.pop()
			_limb(m, Vector3(-0.31, c.y + 0.2, -0.02), Vector3(-0.27, c.y - 0.15, -0.08), 0.008, 0.008, "wood_dark")
		"postie":  # postal cap and bag
			_body(m, "skin_npc", "cloth_green", "pants")
			_face(m, "neutral")
			_hair_cap(m, "eye")
			m.lathe(c + Vector3(0, 0.19, 0), PackedVector2Array([Vector2(0.33, 0.0), Vector2(0.31, 0.08),
				Vector2(0.18, 0.13), Vector2(0.0, 0.14)]), 14, "cloth_green")
			m.box(c + Vector3(0, 0.19, -0.33), Vector3(0.32, 0.035, 0.17), "wood_dark")
			m.box(Vector3(0.22, 0.36, 0.05), Vector3(0.1, 0.2, 0.25), "wood")
			_limb(m, Vector3(-0.15, 0.6, -0.02), Vector3(0.22, 0.42, 0.0), 0.014, 0.014, "wood_dark")
		"scholar":  # áo dài-style long robe and khăn đóng
			_body(m, "skin_npc", "cloth_violet", "cloth_violet")
			m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.25, 0.08), Vector2(0.22, 0.3), Vector2(0.2, 0.46),
				Vector2(0.16, 0.56)]), 12, "cloth_violet")
			_face(m, "neutral")
			_hair_cap(m, "eye")
			m.lathe(c + Vector3(0, 0.12, 0.02), PackedVector2Array([Vector2(0.325, 0.0), Vector2(0.335, 0.07),
				Vector2(0.29, 0.15), Vector2(0.0, 0.17)]), 16, "eye")
			m.blob(Vector3(0, 0.42, -0.2), Vector3(0.12, 0.13, 0.03), "flower_white", 1)
	return m.build()


## Simple dark hair cap of NPCs under their hats (palette colour).
static func _hair_cap(m: LowPoly, color: String) -> void:
	_shell(m, HEAD_RADII + Vector3(0.018, 0.022, 0.022), HEAD_CENTER + Vector3(0, 0.03, 0.015), HEAD_CENTER.y + 0.14,
			0.27, color)
