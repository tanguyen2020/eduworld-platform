class_name BlobShadow
extends MeshInstance3D
## Soft round shadow under a character (F068, open decision F067-3): shown only where the sun casts no real-time
## shadow — the Compatibility renderer (Web) and presets without shadows (QualityPolicy.blob_shadow_visible) — so a
## character never looks like it floats. Colour/opacity from the tokens (`Art3dTokens.LIGHT_BLOB_SHADOW_*`, R32).
## Display only. With `snap_to_ground` (local player) it stays on the floor under a jump and shrinks with height.

const GROUP: StringName = &"blob_shadow"
const SHADER: Shader = preload("res://resources/art/shaders/blob_shadow.gdshader")
## Above the raised plaza paving (≈ 0.085 m) so the spot is never hidden under it.
const LIFT: float = 0.1

static var _material: ShaderMaterial = null

@export var radius: float = 0.34
@export var snap_to_ground: bool = false
## Height above the floor (m) at which the shadow has shrunk to half size.
@export var fade_height: float = 2.0


func _ready() -> void:
	add_to_group(GROUP)
	cast_shadow = GeometryInstance3D.SHADOW_CASTING_SETTING_OFF
	var plane: PlaneMesh = PlaneMesh.new()
	plane.size = Vector2(radius * 2.0, radius * 2.0)
	mesh = plane
	material_override = shared_material()
	position.y = LIFT
	visible = QualityPolicy.blob_shadow_visible(RenderingServer.get_current_rendering_method(), sun_shadows(self))
	set_physics_process(snap_to_ground)


## Whether real-time sun shadows are asked for (QualityManager preset + student setting; medium preset without it).
static func sun_shadows(from: Node) -> bool:
	var quality: Node = from.get_node_or_null(^"/root/QualityManager") if from.is_inside_tree() else null
	if quality != null and quality.has_method(&"shadows_enabled"):
		return bool(quality.call(&"shadows_enabled"))
	return bool(QualityPolicy.preset_value(QualityPolicy.MEDIUM, "shadows"))


static func shared_material() -> ShaderMaterial:
	if _material == null:
		_material = ShaderMaterial.new()
		_material.shader = SHADER
		_material.set_shader_parameter(&"shadow_color", Art3dTokens.LIGHT_BLOB_SHADOW_COLOR)
		_material.set_shader_parameter(&"opacity", Art3dTokens.LIGHT_BLOB_SHADOW_OPACITY)
	return _material


## Scale of the shadow `height` metres above the floor (1 on the floor, ½ at `fade_height`, ≥ 0.3).
static func scale_for_height(height: float, fade: float) -> float:
	return clampf(1.0 / (1.0 + maxf(height, 0.0) / maxf(fade, 0.01)), 0.3, 1.0)


func _physics_process(_delta: float) -> void:
	var body: Node3D = get_parent() as Node3D
	if body == null or not visible or not is_inside_tree():
		return
	var from: Vector3 = body.global_position + Vector3.UP * 0.2
	var query: PhysicsRayQueryParameters3D = PhysicsRayQueryParameters3D.create(from, from + Vector3.DOWN * 20.0)
	var owner_body: Node = body
	while owner_body != null and not owner_body is CollisionObject3D:
		owner_body = owner_body.get_parent()
	if owner_body != null:
		query.exclude = [(owner_body as CollisionObject3D).get_rid()]
	var hit: Dictionary = get_world_3d().direct_space_state.intersect_ray(query)
	if hit.is_empty():
		return
	var floor_y: float = (hit["position"] as Vector3).y
	global_position = Vector3(body.global_position.x, floor_y + LIFT, body.global_position.z)
	var k: float = scale_for_height(body.global_position.y - floor_y, fade_height)
	scale = Vector3(k, 1.0, k)
