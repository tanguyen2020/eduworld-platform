class_name ScatterInstance
extends MultiMeshInstance3D
## Draws a baked ScatterSet as one instanced MultiMesh (one draw call per pass, F067). Display only; the quality
## preset may lower `visible_instance_count` afterwards (ZoneArtQuality, decor density).

@export var scatter: ScatterSet = null


func _ready() -> void:
	if multimesh == null and scatter != null:
		multimesh = build(scatter)


static func build(data: ScatterSet) -> MultiMesh:
	var mm: MultiMesh = MultiMesh.new()
	mm.transform_format = MultiMesh.TRANSFORM_3D
	mm.mesh = data.mesh
	mm.instance_count = data.transforms.size()
	for i: int in data.transforms.size():
		mm.set_instance_transform(i, data.transforms[i])
	return mm
