class_name AvatarPreview3D
extends SubViewportContainer
## One 3D preview of the student's character (F069-AC-07; STUDENT_GAME_UX_SOCIAL §3 "chỉ một AvatarPreview3D",
## STUDENT_GAME_UX §6): the F070 AvatarRig in its own SubViewport (≤ 512×512) that renders ONCE per change — a new look
## (`show_config`) or a drag/turn — never continuously (NFR-COMP-001). Display only: the look comes from the server's
## avatar config or a try-on draft built by the screen; nothing is saved here.

const TURN_STEP: float = PI / 6.0
const DRAG_TURN_FACTOR: float = 0.01

## Renders since the preview was created (tests: no continuous rendering).
var render_requests: int = 0
var config: AvatarConfigModel = null

@onready var _viewport: SubViewport = $Viewport
@onready var _rig: AvatarBuilder = $Viewport/AvatarRig


func _ready() -> void:
	gui_input.connect(_on_gui_input)
	resized.connect(refresh)


## Draws `look` (missing visuals fall back to the default item of the slot, F070-AC-18).
func show_config(look: AvatarConfigModel, catalog: CharacterCatalogModel = null) -> void:
	if look == null:
		return
	config = look
	_rig.apply(look, catalog)
	refresh()


func turn(radians: float) -> void:
	_rig.rotation.y += radians
	refresh()


## Asks the viewport for one new frame.
func refresh() -> void:
	if _viewport == null:
		return
	render_requests += 1
	_viewport.render_target_update_mode = SubViewport.UPDATE_ONCE


func is_continuous() -> bool:
	return _viewport != null and _viewport.render_target_update_mode == SubViewport.UPDATE_ALWAYS


func _on_gui_input(event: InputEvent) -> void:
	if event is InputEventScreenDrag:
		turn((event as InputEventScreenDrag).relative.x * DRAG_TURN_FACTOR)
	elif event is InputEventMouseMotion and (event as InputEventMouseMotion).button_mask != 0:
		turn((event as InputEventMouseMotion).relative.x * DRAG_TURN_FACTOR)
