class_name ScatterSet
extends Resource
## Baked scatter of one repeated decor mesh (F067: trees, bamboo, grass tufts, flowers, rice; PERFORMANCE_BUDGET §13
## "MultiMeshInstance3D for objects repeated ≥ 10 times"). Written by tools/art/bake_art.gd; ScatterInstance turns it
## into a MultiMesh at run time (a MultiMesh keeps its transforms in the RenderingServer, which a headless bake or
## test cannot save or read back). Display data only.

@export var mesh: Mesh = null
@export var transforms: Array[Transform3D] = []
