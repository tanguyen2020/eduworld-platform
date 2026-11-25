class_name ZoneArtQuality
extends RefCounted
## Applies the art settings of a quality preset to a zone (F067; PERFORMANCE_BUDGET §5 "Outline toon", "Mật độ cỏ / vật
## trang trí", "Khoảng cách nhìn"; values in QualityPolicy.PRESETS, CONFIG_DEFAULTS `quality_presets`). Display only:
## - the global shader parameter `outline_fade_distance` of the toon outline (0 = outline at every distance);
## - every MultiMeshInstance3D in the `decor` group draws `decor_density` of its instances (spatially uniform: the
##   bake shuffles placements) up to the preset's visibility range;
## - every BlobShadow (F068) is shown only where the sun casts no real-time shadow (QualityPolicy.blob_shadow_visible).

const DECOR_GROUP: StringName = &"decor"
const OUTLINE_PARAMETER: StringName = &"outline_fade_distance"


static func outline_distance(preset: String) -> float:
	return float(QualityPolicy.preset_value(preset, "outline_distance"))


static func decor_density(preset: String) -> float:
	return clampf(float(QualityPolicy.preset_value(preset, "decor_density")), 0.0, 1.0)


## Instances of a decor MultiMesh drawn at `density` (at least one when the MultiMesh has any).
static func visible_count(instance_count: int, density: float) -> int:
	if instance_count <= 0:
		return 0
	return clampi(roundi(float(instance_count) * density), 1, instance_count)


static func apply(zone: Node, preset: String) -> void:
	RenderingServer.global_shader_parameter_set(OUTLINE_PARAMETER, outline_distance(preset))
	var density: float = decor_density(preset)
	var view: float = float(QualityPolicy.preset_value(preset, "view_distance"))
	if zone.is_inside_tree():
		var blob: bool = QualityPolicy.blob_shadow_visible(RenderingServer.get_current_rendering_method(),
				BlobShadow.sun_shadows(zone))
		for node: Node in zone.get_tree().get_nodes_in_group(BlobShadow.GROUP):
			(node as Node3D).visible = blob
	for node: Node in zone.find_children("*", "MultiMeshInstance3D", true, false):
		var decor: MultiMeshInstance3D = node as MultiMeshInstance3D
		if not decor.is_in_group(DECOR_GROUP) or decor.multimesh == null:
			continue
		decor.multimesh.visible_instance_count = visible_count(decor.multimesh.instance_count, density)
		decor.visibility_range_end = view
