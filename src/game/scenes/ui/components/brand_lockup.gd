class_name BrandLockup
extends VBoxContainer
## EduWorld brand lockup (F096, DESIGN_SYSTEM §5.1): the Knowledge Spark (`SparkMark`), the "EduWorld" wordmark
## (`LogoLabel`) and, below, the guide Mây (`CloudyMascot`). Shared by the in-game splash and the entry notices so they
## match the boot splash and the Web loading view, and rendered to PNG by tools/brand_assets.gd. Vector shapes and
## Theme styles only (AGENTS R32); no logic.

## Shows Mây under the logo (hidden on short windows by the owner screen when space is short).
@export var show_mascot: bool = true:
	set(value):
		show_mascot = value
		if is_node_ready():
			_mascot.visible = value

@onready var _mascot: Control = %Mascot


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_mascot.visible = show_mascot
