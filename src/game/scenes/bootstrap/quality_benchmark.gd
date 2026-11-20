extends Control
## First-run quality benchmark (PERFORMANCE_BUDGET §6, F062-AC-14): before the student can play, the camera orbits
## the Hub for at most `BENCHMARK_MAX_SEC` while QualityManager records frame times, then the preset is chosen and
## saved on this device. "Bỏ qua" stops it and lets the student pick Thấp / Trung bình / Cao by hand. The measuring
## and the choice live in QualityManager/QualityPolicy; this screen only drives the camera and the buttons.

signal finished(preset: String)

## Frames ignored at the start (shader compilation, first upload).
const WARMUP_FRAMES: int = 10

var duration_sec: float = QualityPolicy.BENCHMARK_MAX_SEC
var elapsed_sec: float = 0.0
var running: bool = false
var camera_rig: CameraRig = null
var _frames: int = 0
var _start_yaw: float = 0.0

@onready var _progress: ProgressBar = %Progress
@onready var _skip: Button = %Skip
@onready var _manual: Control = %Manual
@onready var _measuring: Control = %Measuring


func _ready() -> void:
	_skip.pressed.connect(skip)
	_manual.visible = false
	for preset: String in QualityPolicy.PRESET_NAMES:
		var button: Button = find_child("Preset_" + preset, true, false) as Button
		button.pressed.connect(_on_manual_preset.bind(preset))
	set_process(false)


## Starts measuring; `rig` (optional) orbits around the Hub.
func start(rig: CameraRig = null) -> void:
	camera_rig = rig
	_start_yaw = rig.yaw() if rig != null else 0.0
	elapsed_sec = 0.0
	_frames = 0
	running = true
	_measuring.visible = true
	_manual.visible = false
	QualityManager.start_benchmark()
	show()
	set_process(true)


func _process(delta: float) -> void:
	step(delta)


## One frame of the benchmark (tests drive it with synthetic frame times).
func step(delta: float) -> void:
	if not running:
		return
	elapsed_sec += delta
	_frames += 1
	if _frames > WARMUP_FRAMES:
		QualityManager.push_benchmark_frame(delta * 1000.0)
	if camera_rig != null:
		camera_rig.set_yaw(_start_yaw + elapsed_sec / duration_sec * TAU)
	_progress.value = clampf(elapsed_sec / duration_sec, 0.0, 1.0) * _progress.max_value
	if elapsed_sec >= duration_sec:
		_finish(QualityManager.finish_benchmark())


## "Bỏ qua": stop measuring and show the manual choice.
func skip() -> void:
	running = false
	set_process(false)
	_measuring.visible = false
	_manual.visible = true


func _on_manual_preset(preset: String) -> void:
	QualityManager.apply_preset(preset)
	_finish(preset)


func _finish(preset: String) -> void:
	running = false
	set_process(false)
	if camera_rig != null:
		camera_rig.set_yaw(_start_yaw)
	hide()
	finished.emit(preset)
