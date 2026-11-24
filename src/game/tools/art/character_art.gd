extends RefCounted
## Recipes of the chibi characters (F067, WORLD_DESIGN §11 "Nhân vật", CHARACTER_SYSTEM §5): head ≈ 1/3 of the body,
## simple expressive face (eyes with highlights, brows, smile, blush), no weapons. Body space: feet at y = 0, facing -Z
## (WORLD_RENDERING §8.1), about 1.4 m tall. Skin, hair colour and item tints are the per-avatar `ALBEDO` faces; face,
## shoes and trousers come from the art palette tokens. Items are authored relative to their AvatarRig slot anchor.
## SPDX-License-Identifier: MIT — EduWorld first-party (ADR-0102). Bake-time only.

const LowPoly := preload("res://tools/art/lowpoly_mesh.gd")
const A: String = LowPoly.ALBEDO

## Head centre and radii (also the Hair anchor of AvatarRig.tscn).
const HEAD_CENTER: Vector3 = Vector3(0, 1.1, 0)
const HEAD_RADII: Vector3 = Vector3(0.29, 0.28, 0.27)
const HAIR_STYLES: PackedStringArray = ["short", "bob", "ponytail", "braids", "curly", "spiky"]
const OUTFIT_STYLES: PackedStringArray = ["school", "casual", "sport", "star", "rain", "explorer"]
const NPC_LOOKS: PackedStringArray = ["farmer", "postie", "scholar"]


# ---------- body ----------

## Chibi kid body with the face. `skin` is the colour of the bare parts: ALBEDO for avatars, a palette colour for
## NPC bodies that carry their look in the palette.
static func body(skin: String = A, shirt: String = "shirt_white", pants: String = "pants") -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	_body(m, skin, shirt, pants)
	_face(m, skin)
	return m.build()


static func _body(m: LowPoly, skin: String, shirt: String, pants: String) -> void:
	for x: float in [-0.095, 0.095]:
		m.box(Vector3(x, 0.045, -0.03), Vector3(0.13, 0.09, 0.2), "shoe", "shoe")
		m.cylinder(Vector3(x, 0.08, 0), 0.068, 0.072, 0.28, 6, pants)
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.165, 0.33), Vector2(0.185, 0.4), Vector2(0.18, 0.52),
		Vector2(0.17, 0.62), Vector2(0.14, 0.72), Vector2(0.07, 0.78)]), 8, shirt)
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.172, 0.33), Vector2(0.19, 0.41)]), 8, pants)
	m.cylinder(Vector3(0, 0.76, 0), 0.06, 0.055, 0.1, 6, skin)
	for s: float in [-1.0, 1.0]:
		var shoulder: Vector3 = Vector3(s * 0.17, 0.7, 0)
		var elbow: Vector3 = Vector3(s * 0.235, 0.53, 0.01)
		var hand: Vector3 = Vector3(s * 0.26, 0.4, -0.01)
		_limb(m, shoulder, elbow, 0.058, 0.05, shirt)
		_limb(m, elbow, hand, 0.048, 0.044, skin)
		m.blob(hand + Vector3(0, -0.03, 0), Vector3(0.055, 0.06, 0.055), skin, 1)
	m.blob(HEAD_CENTER, HEAD_RADII, skin, 2)
	for s: float in [-1.0, 1.0]:
		m.blob(HEAD_CENTER + Vector3(s * 0.28, -0.02, 0.02), Vector3(0.05, 0.07, 0.04), skin, 0)


## Big friendly face on the -Z side of the head: two tall eyes with highlights, soft brows, a smile, blush.
static func _face(m: LowPoly, _skin: String) -> void:
	var c: Vector3 = HEAD_CENTER
	for s: float in [-1.0, 1.0]:
		m.blob(c + Vector3(s * 0.098, 0.0, -0.245), Vector3(0.046, 0.066, 0.03), "eye", 1)
		m.blob(c + Vector3(s * 0.085, 0.024, -0.268), Vector3(0.016, 0.018, 0.012), "eye_light", 0)
		m.blob(c + Vector3(s * 0.165, -0.07, -0.21), Vector3(0.05, 0.028, 0.025), "cheek", 0)
		m.push(Transform3D(Basis(Vector3.FORWARD, s * 0.18), c + Vector3(s * 0.1, 0.098, -0.248)))
		m.box(Vector3.ZERO, Vector3(0.075, 0.016, 0.03), "eye")
		m.pop()
	# smile: a small crescent on the face plane
	var smile: PackedVector2Array = PackedVector2Array()
	for i: int in 7:
		var a: float = lerpf(PI * 1.15, PI * 1.85, float(i) / 6.0)
		smile.append(Vector2(cos(a) * 0.06, sin(a) * 0.04))
	for i: int in range(6, -1, -1):
		var a: float = lerpf(PI * 1.15, PI * 1.85, float(i) / 6.0)
		smile.append(Vector2(cos(a) * 0.045, sin(a) * 0.022 + 0.004))
	m.push(Transform3D(Basis(Vector3.UP, PI), c + Vector3(0, -0.075, -0.255)))
	m.extrude(smile, 0.03, "mouth")
	m.pop()


static func _limb(m: LowPoly, a: Vector3, b: Vector3, ra: float, rb: float, col: String) -> void:
	var dir: Vector3 = b - a
	var length: float = dir.length()
	var up: Vector3 = dir / length
	var side: Vector3 = up.cross(Vector3.FORWARD).normalized()
	var fwd: Vector3 = side.cross(up)
	m.push(Transform3D(Basis(side, up, fwd), a))
	m.cylinder(Vector3.ZERO, ra, rb, length, 6, col)
	m.pop()


# ---------- hair (relative to the Hair anchor = head centre) ----------

## Hair shell that leaves the face open: an ellipsoid whose front-lower vertices are pulled behind the face.
static func _shell(m: LowPoly, radii: Vector3, offset: Vector3, brow: float, open_width: float) -> void:
	var t: Array = LowPoly._icosphere(2)
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
		m.tri(moved[faces[i]], moved[faces[i + 1]], moved[faces[i + 2]], A)


static func hair(style: String) -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	var rng: RandomNumberGenerator = RandomNumberGenerator.new()
	rng.seed = 6730 + HAIR_STYLES.find(style)
	match style:
		"short":
			_shell(m, Vector3(0.305, 0.27, 0.3), Vector3(0, 0.05, 0.02), 0.07, 0.24)
			_fringe(m, 3, 0.055)
		"bob":
			_shell(m, Vector3(0.335, 0.31, 0.32), Vector3(0, 0.0, 0.025), 0.08, 0.22)
			_fringe(m, 4, 0.06)
		"ponytail":
			_shell(m, Vector3(0.305, 0.275, 0.3), Vector3(0, 0.05, 0.02), 0.07, 0.24)
			_fringe(m, 3, 0.05)
			m.blob(Vector3(0, 0.08, 0.31), Vector3(0.05, 0.05, 0.05), "cloth_red", 0)
			m.blob(Vector3(0, -0.08, 0.36), Vector3(0.09, 0.17, 0.09), A, 1)
		"braids":
			_shell(m, Vector3(0.31, 0.28, 0.305), Vector3(0, 0.04, 0.02), 0.07, 0.23)
			_fringe(m, 4, 0.05)
			for s: float in [-1.0, 1.0]:
				for k: int in 3:
					m.blob(Vector3(s * 0.25, -0.16 - k * 0.1, 0.06), Vector3(0.055, 0.06, 0.055), A, 0)
				m.blob(Vector3(s * 0.25, -0.46, 0.06), Vector3(0.04, 0.03, 0.04), "cloth_red", 0)
		"curly":
			_shell(m, Vector3(0.31, 0.28, 0.305), Vector3(0, 0.05, 0.02), 0.08, 0.24)
			for k: int in 14:
				var a: float = TAU * float(k) / 14.0
				var y: float = 0.12 + 0.12 * sin(a * 2.0 + 0.5)
				var p: Vector3 = Vector3(cos(a) * 0.27, y, sin(a) * 0.26 + 0.03)
				if p.z < -0.15 and y < 0.15:
					p.y += 0.1
				m.blob(p, Vector3(0.085, 0.085, 0.085) * rng.randf_range(0.9, 1.15), A, 0)
			m.blob(Vector3(0, 0.3, 0.02), Vector3(0.14, 0.08, 0.14), A, 0)
		"spiky":
			_shell(m, Vector3(0.3, 0.26, 0.295), Vector3(0, 0.05, 0.02), 0.07, 0.24)
			for k: int in 7:
				var a: float = lerpf(-1.2, 1.2, float(k) / 6.0)
				var base: Vector3 = Vector3(sin(a) * 0.16, 0.24, cos(a) * 0.1 + 0.02)
				var tip: Vector3 = base + Vector3(sin(a) * 0.12, 0.17, 0.08 + absf(sin(a)) * 0.05)
				_spike(m, base, tip, 0.07)
	return m.build()


static func _fringe(m: LowPoly, count: int, size: float) -> void:
	for k: int in count:
		var x: float = lerpf(-0.17, 0.17, float(k) / float(maxi(count - 1, 1)))
		var top: Vector3 = Vector3(x, 0.2, -0.2)
		m.tri(top + Vector3(-size, 0, 0.02), top + Vector3(size, 0, 0.02), Vector3(x + 0.01, 0.1, -0.268), A)
		m.tri(top + Vector3(size, 0, 0.02), top + Vector3(-size, 0, 0.02), top + Vector3(0, 0.03, 0.08), A)


static func _spike(m: LowPoly, base: Vector3, tip: Vector3, r: float) -> void:
	var up: Vector3 = (tip - base).normalized()
	var side: Vector3 = up.cross(Vector3.FORWARD).normalized()
	var fwd: Vector3 = side.cross(up)
	m.push(Transform3D(Basis(side, up, fwd), base))
	m.cylinder(Vector3.ZERO, r, 0.0, (tip - base).length(), 4, A)
	m.pop()


# ---------- outfits (body space, Outfit anchor at the feet) ----------

static func outfit(style: String) -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	var long: bool = style == "rain"
	var bottom: float = 0.26 if long else 0.37
	m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.2 if long else 0.19, bottom), Vector2(0.197, 0.45),
		Vector2(0.19, 0.53), Vector2(0.182, 0.63), Vector2(0.152, 0.72), Vector2(0.09, 0.79)]), 8, A)
	for s: float in [-1.0, 1.0]:
		var shoulder: Vector3 = Vector3(s * 0.17, 0.71, 0)
		var sleeve_end: Vector3 = Vector3(s * 0.232, 0.53 if not long else 0.45, 0.01)
		_limb(m, shoulder, sleeve_end, 0.07, 0.064, A)
	match style:
		"school":
			# white collar and the red scarf (khăn quàng đỏ) of the school uniform
			m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.135, 0.74), Vector2(0.1, 0.8)]), 8, "shirt_white")
			m.extrude(PackedVector2Array([Vector2(-0.07, 0.0), Vector2(0.07, 0.0), Vector2(0.0, -0.13)]), 0.02, "cloth_red")
			_scarf(m)
		"casual":
			m.box(Vector3(0.08, 0.6, -0.19), Vector3(0.07, 0.07, 0.02), "wood")
		"sport":
			m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.196, 0.5), Vector2(0.192, 0.55)]), 8, "shirt_white")
		"star":
			m.push(Transform3D(Basis(Vector3.UP, PI).scaled(Vector3(0.09, 0.09, 1)), Vector3(0, 0.6, -0.188)))
			m.extrude(_star(), 0.02, "lantern_glow")
			m.pop()
		"rain":
			for y: float in [0.62, 0.5, 0.38]:
				m.blob(Vector3(0, y, -0.2), Vector3(0.018, 0.018, 0.012), "lantern_glow", 0)
			m.blob(Vector3(0, 0.83, 0.13), Vector3(0.2, 0.12, 0.12), A, 1)
		"explorer":
			for x: float in [-0.09, 0.09]:
				m.box(Vector3(x, 0.47, -0.19), Vector3(0.08, 0.07, 0.025), "wood")
			_limb(m, Vector3(-0.16, 0.74, -0.08), Vector3(0.17, 0.42, -0.14), 0.016, 0.016, "wood_dark")
	return m.build()


static func _scarf(m: LowPoly) -> void:
	m.push(Transform3D(Basis.IDENTITY, Vector3(0, 0.745, -0.15)))
	m.extrude(PackedVector2Array([Vector2(-0.06, 0.02), Vector2(0.06, 0.02), Vector2(0.015, -0.12), Vector2(-0.015, -0.12)]), 0.02, "cloth_red")
	m.pop()


static func _star() -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array()
	for i: int in 10:
		var a: float = PI * 0.5 + TAU * float(i) / 10.0
		pts.append(Vector2(cos(a), sin(a)) * (1.0 if i % 2 == 0 else 0.45))
	return pts


# ---------- accessories (relative to their slot anchor) ----------

static func accessory(code: String) -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	match code:
		"acc_cap_star":  # Head anchor: top of the head
			m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.27, -0.08), Vector2(0.25, 0.0), Vector2(0.18, 0.07),
				Vector2(0.0, 0.1)]), 10, A)
			m.box(Vector3(0, -0.07, -0.27), Vector3(0.32, 0.03, 0.2), A)
			m.push(Transform3D(Basis(Vector3.UP, PI).scaled(Vector3(0.06, 0.06, 1)), Vector3(0, -0.01, -0.245)))
			m.extrude(_star(), 0.02, "lantern_glow")
			m.pop()
		"acc_glasses_round":  # Face anchor: front of the eyes
			for s: float in [-1.0, 1.0]:
				m.push(Transform3D(Basis.IDENTITY, Vector3(s * 0.098, 0, 0)))
				_ring_xy(m, 0.07, 0.012, 12, "eye")
				m.pop()
			m.box(Vector3(0, 0.01, 0), Vector3(0.06, 0.014, 0.014), "eye")
		"acc_backpack_owl":  # Back anchor
			m.box(Vector3(0, 0, 0.06), Vector3(0.3, 0.32, 0.14), A)
			m.blob(Vector3(0, 0.16, 0.06), Vector3(0.15, 0.06, 0.07), A, 0)
			for s: float in [-1.0, 1.0]:
				m.blob(Vector3(s * 0.065, 0.06, 0.135), Vector3(0.045, 0.045, 0.015), "eye_light", 0)
				m.blob(Vector3(s * 0.065, 0.06, 0.148), Vector3(0.02, 0.02, 0.01), "eye", 0)
				m.box(Vector3(s * 0.11, 0.05, -0.06), Vector3(0.04, 0.3, 0.04), "wood_dark")
			m.extrude(PackedVector2Array([Vector2(-0.025, 0.0), Vector2(0.025, 0.0), Vector2(0.0, -0.04)]), 0.03, "lantern_glow")
		"acc_flag_small":  # Hand anchor
			m.cylinder(Vector3(0, -0.05, 0), 0.012, 0.012, 0.5, 4, "wood")
			m.push(Transform3D(Basis(Vector3.UP, PI * 0.5), Vector3(0, 0.33, 0)))
			m.extrude(PackedVector2Array([Vector2(0, 0.12), Vector2(0.24, 0.05), Vector2(0, -0.02)]), 0.012, A)
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


# ---------- NPC bodies (whole look in the palette, one draw call + outline) ----------

static func npc(look: String) -> ArrayMesh:
	var m: LowPoly = LowPoly.new()
	match look:
		"farmer":  # áo bà ba + nón lá
			_body(m, "skin_npc", "cloth_blue", "wood_dark")
			_face(m, "skin_npc")
			m.push(Transform3D(Basis.IDENTITY, HEAD_CENTER + Vector3(0, 0.2, 0)))
			m.cylinder(Vector3.ZERO, 0.52, 0.0, 0.3, 14, "basket", "", false)
			m.cylinder(Vector3(0, -0.005, 0), 0.52, 0.52, 0.01, 14, "wood", "", false)
			m.pop()
			_limb(m, Vector3(-0.3, 1.25, -0.02), Vector3(-0.25, 0.85, -0.06), 0.008, 0.008, "wood_dark")
		"postie":  # postal cap and bag
			_body(m, "skin_npc", "cloth_green", "pants")
			_face(m, "skin_npc")
			_shell(m, Vector3(0.3, 0.26, 0.295), HEAD_CENTER + Vector3(0, 0.05, 0.02), HEAD_CENTER.y + 0.07, 0.24)
			m.lathe(HEAD_CENTER + Vector3(0, 0.17, 0), PackedVector2Array([Vector2(0.3, 0.0), Vector2(0.28, 0.08),
				Vector2(0.0, 0.12)]), 10, "cloth_green")
			m.box(HEAD_CENTER + Vector3(0, 0.17, -0.3), Vector3(0.3, 0.03, 0.16), "wood_dark")
			m.box(Vector3(0.22, 0.45, 0.05), Vector3(0.1, 0.22, 0.26), "wood")
			_limb(m, Vector3(-0.15, 0.74, -0.02), Vector3(0.22, 0.52, 0.0), 0.014, 0.014, "wood_dark")
		"scholar":  # áo dài-style long robe and khăn đóng
			_body(m, "skin_npc", "cloth_violet", "cloth_violet")
			m.lathe(Vector3.ZERO, PackedVector2Array([Vector2(0.24, 0.12), Vector2(0.21, 0.4), Vector2(0.18, 0.6),
				Vector2(0.15, 0.72)]), 8, "cloth_violet")
			_face(m, "skin_npc")
			m.lathe(HEAD_CENTER + Vector3(0, 0.12, 0.02), PackedVector2Array([Vector2(0.3, 0.0), Vector2(0.31, 0.07),
				Vector2(0.27, 0.14), Vector2(0.0, 0.16)]), 12, "eye")
			m.blob(Vector3(0, 0.52, -0.2), Vector3(0.13, 0.17, 0.035), "flower_white", 0)
	return m.build()
