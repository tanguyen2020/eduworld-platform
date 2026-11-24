extends RefCounted
## Low-poly mesh builder of the F067 art pass (bake-time tool, not shipped: tools/* is excluded from exports).
## SPDX-License-Identifier: MIT — EduWorld first-party (ADR-0102).
## Every face is flat shaded and coloured by NAME from the art palette tokens (`Art3dTokens.ART_PALETTE`): the face
## gets UV = (slot + 0.5, 0.5) — the slot of the toon shader palette, stable when slots are appended — or `ALBEDO`
## (UV.y > 1) for parts tinted per
## avatar (skin, hair, outfit). No colour value lives in a mesh, so a token change re-colours every baked asset without
## a re-bake (R32). Smoothed normals (welded by position) are stored in COLOR for the crack-free outline pass.
## Deterministic: same calls → same arrays (random placement uses a seeded RandomNumberGenerator).

const ALBEDO: String = "@albedo"
const ALBEDO_UV: Vector2 = Vector2(0.5, 1.5)
const WELD: float = 10000.0

var palette: PackedStringArray = PackedStringArray(Art3dTokens.ART_PALETTE)
var _verts: PackedVector3Array = PackedVector3Array()
var _normals: PackedVector3Array = PackedVector3Array()
var _uvs: PackedVector2Array = PackedVector2Array()
var _stack: Array[Transform3D] = [Transform3D.IDENTITY]


func triangle_count() -> int:
	return _verts.size() / 3


# ---------- transform stack ----------

func push(t: Transform3D) -> void:
	_stack.append(_stack.back() * t)


func push_at(origin: Vector3, yaw_deg: float = 0.0, scale: Vector3 = Vector3.ONE) -> void:
	push(Transform3D(Basis(Vector3.UP, deg_to_rad(yaw_deg)).scaled(scale), origin))


func pop() -> void:
	if _stack.size() > 1:
		_stack.pop_back()


func _xf() -> Transform3D:
	return _stack.back()


# ---------- colour ----------

func uv_of(color: String) -> Vector2:
	if color == ALBEDO:
		return ALBEDO_UV
	var slot: int = palette.find(color)
	assert(slot >= 0, "unknown art palette colour '%s'" % color)
	return Vector2(float(maxi(slot, 0)) + 0.5, 0.5)


# ---------- faces ----------

## One flat triangle (counter-clockwise seen from the front, Godot convention: front faces are clockwise in screen
## space, so the builder emits a, c, b order internally).
func tri(a: Vector3, b: Vector3, c: Vector3, color: String) -> void:
	var t: Transform3D = _xf()
	var pa: Vector3 = t * a
	var pb: Vector3 = t * b
	var pc: Vector3 = t * c
	var n: Vector3 = (pb - pa).cross(pc - pa)
	if n.length_squared() < 1e-12:
		return
	n = n.normalized()
	var uv: Vector2 = uv_of(color)
	# A mirrored transform (negative determinant) flips the winding: keep the face pointing outwards.
	var order: PackedVector3Array = PackedVector3Array([pa, pc, pb])
	if t.basis.determinant() < 0.0:
		order = PackedVector3Array([pa, pb, pc])
		n = -n
	for p: Vector3 in order:
		_verts.append(p)
		_normals.append(n)
		_uvs.append(uv)


func quad(a: Vector3, b: Vector3, c: Vector3, d: Vector3, color: String) -> void:
	tri(a, b, c, color)
	tri(a, c, d, color)


## Axis-aligned box centred on `center`; `top` colours the top face (default: `color`).
func box(center: Vector3, size: Vector3, color: String, top: String = "", bottom: bool = false) -> void:
	var h: Vector3 = size * 0.5
	var c: Array[Vector3] = []
	for i: int in 8:
		c.append(center + Vector3(h.x if i & 1 else -h.x, h.y if i & 2 else -h.y, h.z if i & 4 else -h.z))
	var t: String = color if top.is_empty() else top
	quad(c[2], c[6], c[7], c[3], t)          # top (+Y)
	quad(c[4], c[5], c[7], c[6], color)      # +Z
	quad(c[1], c[0], c[2], c[3], color)      # -Z
	quad(c[5], c[1], c[3], c[7], color)      # +X
	quad(c[0], c[4], c[6], c[2], color)      # -X
	if bottom:
		quad(c[0], c[1], c[5], c[4], color)


## Frustum / cylinder / cone along +Y from `base` (sides ≥ 3). `top_color` colours the cap.
func cylinder(base: Vector3, r_bottom: float, r_top: float, height: float, sides: int, color: String,
		top_color: String = "", cap: bool = true, phase: float = 0.0) -> void:
	var tc: String = color if top_color.is_empty() else top_color
	for i: int in sides:
		var a0: float = TAU * (float(i) + phase) / float(sides)
		var a1: float = TAU * (float(i + 1) + phase) / float(sides)
		var d0: Vector3 = Vector3(cos(a0), 0, sin(a0))
		var d1: Vector3 = Vector3(cos(a1), 0, sin(a1))
		var b0: Vector3 = base + d0 * r_bottom
		var b1: Vector3 = base + d1 * r_bottom
		var t0: Vector3 = base + d0 * r_top + Vector3.UP * height
		var t1: Vector3 = base + d1 * r_top + Vector3.UP * height
		if r_top > 0.0001:
			quad(b0, t0, t1, b1, color)
			if cap:
				tri(base + Vector3.UP * height, t1, t0, tc)
		else:
			tri(b0, t0, b1, color)


## Low-poly ellipsoid (icosphere, `subdiv` 0–2) centred on `center` with radii `radii`; `jitter` (0–0.3) roughens
## the surface for foliage. `bottom_flat` clips everything below `center.y - radii.y * bottom_flat`.
func blob(center: Vector3, radii: Vector3, color: String, subdiv: int = 1, jitter: float = 0.0,
		rng: RandomNumberGenerator = null) -> void:
	var t: Array = _icosphere(subdiv)
	var verts: PackedVector3Array = t[0]
	var faces: PackedInt32Array = t[1]
	var moved: PackedVector3Array = PackedVector3Array()
	for v: Vector3 in verts:
		var k: float = 1.0
		if jitter > 0.0 and rng != null:
			k += rng.randf_range(-jitter, jitter)
		moved.append(center + Vector3(v.x * radii.x, v.y * radii.y, v.z * radii.z) * k)
	for i: int in range(0, faces.size(), 3):
		tri(moved[faces[i]], moved[faces[i + 1]], moved[faces[i + 2]], color)


## Polygon in the XY plane (counter-clockwise), extruded along +Z by `depth` (centred on z = 0).
func extrude(points: PackedVector2Array, depth: float, color: String, side_color: String = "") -> void:
	var sc: String = color if side_color.is_empty() else side_color
	if _area(points) < 0.0:
		points = points.duplicate()
		points.reverse()
	var idx: PackedInt32Array = Geometry2D.triangulate_polygon(points)
	var hz: float = depth * 0.5
	for i: int in range(0, idx.size(), 3):
		var a: Vector2 = points[idx[i]]
		var b: Vector2 = points[idx[i + 1]]
		var c: Vector2 = points[idx[i + 2]]
		if (b - a).cross(c - a) < 0.0:
			var swap: Vector2 = b
			b = c
			c = swap
		tri(Vector3(a.x, a.y, hz), Vector3(b.x, b.y, hz), Vector3(c.x, c.y, hz), color)
		tri(Vector3(a.x, a.y, -hz), Vector3(c.x, c.y, -hz), Vector3(b.x, b.y, -hz), color)
	var n: int = points.size()
	for i: int in n:
		var p: Vector2 = points[i]
		var q: Vector2 = points[(i + 1) % n]
		quad(Vector3(p.x, p.y, -hz), Vector3(q.x, q.y, -hz), Vector3(q.x, q.y, hz), Vector3(p.x, p.y, hz), sc)


## Surface of revolution around +Y: `profile` = [(radius, y), …] bottom to top.
func lathe(base: Vector3, profile: PackedVector2Array, sides: int, color: String, inside: bool = false) -> void:
	for s: int in profile.size() - 1:
		var p0: Vector2 = profile[s]
		var p1: Vector2 = profile[s + 1]
		for i: int in sides:
			var a0: float = TAU * float(i) / float(sides)
			var a1: float = TAU * float(i + 1) / float(sides)
			var d0: Vector3 = Vector3(cos(a0), 0, sin(a0))
			var d1: Vector3 = Vector3(cos(a1), 0, sin(a1))
			var v00: Vector3 = base + d0 * p0.x + Vector3.UP * p0.y
			var v01: Vector3 = base + d1 * p0.x + Vector3.UP * p0.y
			var v10: Vector3 = base + d0 * p1.x + Vector3.UP * p1.y
			var v11: Vector3 = base + d1 * p1.x + Vector3.UP * p1.y
			if inside:
				quad(v00, v01, v11, v10, color)
			else:
				quad(v00, v10, v11, v01, color)


## Vietnamese tiled roof (mái ngói): a hip roof over a `width` × `depth` footprint at height `y`, ridge length
## `width - depth`, eaves overhang `over`, with upturned corners (`curl`) and a dark ridge beam.
func tiled_roof(center: Vector3, width: float, depth: float, rise: float, over: float, curl: float,
		color: String = "roof", ridge_color: String = "roof_dark") -> void:
	var hw: float = width * 0.5 + over
	var hd: float = depth * 0.5 + over
	var ridge_half: float = maxf(width * 0.5 - depth * 0.5, 0.3)
	var e: Array[Vector3] = [
		center + Vector3(-hw, curl, -hd), center + Vector3(hw, curl, -hd),
		center + Vector3(hw, curl, hd), center + Vector3(-hw, curl, hd),
	]
	# Eave mid points slightly lower than the corners: the curved line of a mái ngói.
	var m: Array[Vector3] = [
		center + Vector3(0, 0, -hd), center + Vector3(hw * 0.98, 0, 0),
		center + Vector3(0, 0, hd), center + Vector3(-hw * 0.98, 0, 0),
	]
	var r0: Vector3 = center + Vector3(-ridge_half, rise, 0)
	var r1: Vector3 = center + Vector3(ridge_half, rise, 0)
	var rm: Vector3 = (r0 + r1) * 0.5
	# front (-Z) and back (+Z) slopes, split at the eave mid point
	quad(e[0], r0, rm, m[0], color)
	quad(m[0], rm, r1, e[1], color)
	quad(e[2], r1, rm, m[2], color)
	quad(m[2], rm, r0, e[3], color)
	# hip ends
	tri(e[1], r1, m[1], color)
	tri(m[1], r1, e[2], color)
	tri(e[3], r0, m[3], color)
	tri(m[3], r0, e[0], color)
	# ridge beam with raised ends
	box((r0 + r1) * 0.5 + Vector3.UP * 0.06, Vector3(ridge_half * 2.0 + 0.3, 0.14, 0.2), ridge_color)
	for r: Vector3 in [r0, r1]:
		cylinder(r + Vector3(0, 0.02, 0), 0.1, 0.02, 0.35, 4, ridge_color)
	# eave edge (fascia) for thickness
	for i: int in 4:
		var a: Vector3 = e[i]
		var b: Vector3 = m[i]
		var c: Vector3 = e[(i + 1) % 4]
		for seg: Array in [[a, b], [b, c]]:
			var p: Vector3 = seg[0]
			var q: Vector3 = seg[1]
			quad(p, q, q + Vector3.DOWN * 0.12, p + Vector3.DOWN * 0.12, ridge_color)


# ---------- output ----------

## Builds the ArrayMesh (one surface: VERTEX, NORMAL, COLOR = smoothed normal * 0.5 + 0.5, TEX_UV).
func build() -> ArrayMesh:
	var smooth: Dictionary = {}
	for i: int in _verts.size():
		var key: Vector3i = _weld_key(_verts[i])
		smooth[key] = (smooth.get(key, Vector3.ZERO) as Vector3) + _normals[i]
	var colors: PackedColorArray = PackedColorArray()
	colors.resize(_verts.size())
	for i: int in _verts.size():
		var n: Vector3 = smooth[_weld_key(_verts[i])] as Vector3
		n = n.normalized() if n.length_squared() > 1e-8 else _normals[i]
		colors[i] = Color(n.x * 0.5 + 0.5, n.y * 0.5 + 0.5, n.z * 0.5 + 0.5, 1.0)  # art-lint: not a colour (smoothed normal)
	var arrays: Array = []
	arrays.resize(Mesh.ARRAY_MAX)
	arrays[Mesh.ARRAY_VERTEX] = _verts
	arrays[Mesh.ARRAY_NORMAL] = _normals
	arrays[Mesh.ARRAY_COLOR] = colors
	arrays[Mesh.ARRAY_TEX_UV] = _uvs
	var mesh: ArrayMesh = ArrayMesh.new()
	if not _verts.is_empty():
		mesh.add_surface_from_arrays(Mesh.PRIMITIVE_TRIANGLES, arrays)
	return mesh


func _weld_key(v: Vector3) -> Vector3i:
	return Vector3i(roundi(v.x * WELD), roundi(v.y * WELD), roundi(v.z * WELD))


static func _area(points: PackedVector2Array) -> float:
	var a: float = 0.0
	for i: int in points.size():
		a += points[i].cross(points[(i + 1) % points.size()])
	return a * 0.5


static func _icosphere(subdiv: int) -> Array:
	var p: float = (1.0 + sqrt(5.0)) / 2.0
	var verts: PackedVector3Array = PackedVector3Array([
		Vector3(-1, p, 0), Vector3(1, p, 0), Vector3(-1, -p, 0), Vector3(1, -p, 0),
		Vector3(0, -1, p), Vector3(0, 1, p), Vector3(0, -1, -p), Vector3(0, 1, -p),
		Vector3(p, 0, -1), Vector3(p, 0, 1), Vector3(-p, 0, -1), Vector3(-p, 0, 1),
	])
	for i: int in verts.size():
		verts[i] = verts[i].normalized()
	var faces: PackedInt32Array = PackedInt32Array([
		0, 11, 5, 0, 5, 1, 0, 1, 7, 0, 7, 10, 0, 10, 11, 1, 5, 9, 5, 11, 4, 11, 10, 2, 10, 7, 6, 7, 1, 8,
		3, 9, 4, 3, 4, 2, 3, 2, 6, 3, 6, 8, 3, 8, 9, 4, 9, 5, 2, 4, 11, 6, 2, 10, 8, 6, 7, 9, 8, 1,
	])
	for s: int in clampi(subdiv, 0, 3):
		var mid: Dictionary = {}
		var next: PackedInt32Array = PackedInt32Array()
		for i: int in range(0, faces.size(), 3):
			var a: int = faces[i]
			var b: int = faces[i + 1]
			var c: int = faces[i + 2]
			var ab: int = _mid(verts, mid, a, b)
			var bc: int = _mid(verts, mid, b, c)
			var ca: int = _mid(verts, mid, c, a)
			next.append_array(PackedInt32Array([a, ab, ca, b, bc, ab, c, ca, bc, ab, bc, ca]))
		faces = next
	return [verts, faces]


static func _mid(verts: PackedVector3Array, cache: Dictionary, a: int, b: int) -> int:
	var key: Vector2i = Vector2i(mini(a, b), maxi(a, b))
	if cache.has(key):
		return int(cache[key])
	verts.append(((verts[a] + verts[b]) * 0.5).normalized())
	cache[key] = verts.size() - 1
	return verts.size() - 1
