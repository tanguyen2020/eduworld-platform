extends SceneTree
## CLI: bakes the first-party art (F067, F068, F093; ADR-0102, ADR-0103 — meshes, MultiMesh scatters, animation clips) into
## res://assets/models/** or checks it is up to date.
##   godot --headless --path src/game -s res://tools/art/bake_art.gd             write every mesh / MultiMesh
##   godot --headless --path src/game -s res://tools/art/bake_art.gd -- --check  exit 1 when a resource is stale
## The recipes (tools/art/*_art.gd) are the source; the .res files are build output reviewed with screenshots
## (tools/capture_hub.gd). Every baked file is first-party MIT and listed in res://assets/LICENSES.md.
## SPDX-License-Identifier: MIT — EduWorld first-party.

const ArtCatalog := preload("res://tools/art/art_catalog.gd")


func _initialize() -> void:
	if "--check" in OS.get_cmdline_user_args():
		var drift: PackedStringArray = ArtCatalog.drift()
		for line: String in drift:
			printerr("bake_art: stale " + line)
		print("bake_art: %d resource(s) stale" % drift.size())
		quit(1 if not drift.is_empty() else 0)
		return
	var written: int = 0
	var triangles: int = 0
	var recipes: Dictionary = ArtCatalog.meshes()
	for path: String in recipes:
		var mesh: ArrayMesh = (recipes[path] as Callable).call() as ArrayMesh
		triangles += mesh.get_faces().size() / 3
		prints(path.get_file(), mesh.get_faces().size() / 3)
		written += _save(mesh, path)
	var clips: Dictionary = ArtCatalog.animations()
	for path: String in clips:
		written += _save((clips[path] as Callable).call() as Animation, path)
	for kind: String in ArtCatalog.VEG_KINDS:
		var mesh_res: Mesh = ResourceLoader.load(ArtCatalog.veg_mesh_path(kind), "", ResourceLoader.CACHE_MODE_REPLACE) as Mesh
		written += _save(ArtCatalog.scatter(kind, mesh_res), ArtCatalog.veg_scatter_path(kind))
	var zone_sets: Dictionary = ArtCatalog.zone_scatters()
	for path: String in zone_sets:
		var entry: Array = zone_sets[path] as Array
		var veg: Mesh = ResourceLoader.load(String(entry[0]), "", ResourceLoader.CACHE_MODE_REPLACE) as Mesh
		var transforms: Array[Transform3D] = (entry[1] as Callable).call()
		written += _save(ArtCatalog.scatter_of(veg, transforms), path)
	print("bake_art: %d resource(s) written, %d mesh triangles" % [written, triangles])
	quit(0)


func _save(res: Resource, path: String) -> int:
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(path.get_base_dir()))
	var err: Error = ResourceSaver.save(res, path, ResourceSaver.FLAG_COMPRESS)
	if err != OK:
		printerr("bake_art: cannot save %s (%d)" % [path, err])
		return 0
	return 1
