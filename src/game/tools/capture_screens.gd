extends SceneTree
## Review capture of the student journey (F096 T001/T030, DESIGN_SYSTEM §9, STUDENT_GAME_UX §2): renders each student
## screen at fixed window sizes and locales to PNG — before and after a UX pass — on the renderer of the run. Needs a
## renderer (not --headless); the owner reviews the Web build, so run it on Compatibility too:
##   godot --path src/game --rendering-method gl_compatibility --rendering-driver opengl3 \
##         -s res://tools/capture_screens.gd -- --out=<dir> [--tag=after] [--screens=login,hub] \
##         [--sizes=1280x720,1024x600,720x1280] [--locales=vi-VN,en-US] [--band=g1_3]
## The work is done by tools/lib/screen_capture.gd, loaded once the autoloads exist (a main-loop script compiles
## before them). TEST-ONLY data (AGENTS R17): fixtures and fakes of res://tests/fakes; nothing here ships.


func _initialize() -> void:
	_run.call_deferred()


func _run() -> void:
	if DisplayServer.get_name() == "headless":
		printerr("capture_screens: needs a renderer (run without --headless)")
		quit(2)
		return
	var runner: Node = (load("res://tools/lib/screen_capture.gd") as GDScript).new() as Node
	root.add_child(runner)
	var code: int = await runner.call("run", OS.get_cmdline_user_args())
	runner.queue_free()
	quit(code)
