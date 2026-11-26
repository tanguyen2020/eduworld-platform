class_name OnboardingGuide
extends RefCounted
## Mây's first-time guide of the Hub (F096-AC-10, STUDENT_GAME_UX_CORE S03 "Hướng dẫn lần đầu", WORLD_DESIGN §2: Mây
## guides): at most MAX_STEPS short steps, each one sentence with an icon and the HUD control it points at; always
## skippable. Shown when the student just created the character in this session or when this device never showed it
## (`Settings.onboarding_hub_seen`). Pure model: the HUD renders it, nothing here calls a service (AGENTS R1).

## STUDENT_GAME_UX_CORE S03: at most 3 steps.
const MAX_STEPS: int = 3
## Steps: translation key, icon (res://resources/ui/icons) and the unique name of the HUD control to highlight
## ("" = none).
const STEPS: Array[Dictionary] = [
	{"key": "ui.onboarding.step_welcome", "icon": "res://resources/ui/icons/home.svg", "target": ""},
	{"key": "ui.onboarding.step_assignments", "icon": "res://resources/ui/icons/assignments.svg",
		"target": "AssignmentsChip"},
	{"key": "ui.onboarding.step_explore", "icon": "res://resources/ui/icons/map.svg", "target": "MapButton"},
]

var step: int = 0
var finished: bool = false


static func should_show(seen_on_device: bool, just_created_character: bool) -> bool:
	return just_created_character or not seen_on_device


func count() -> int:
	return mini(STEPS.size(), MAX_STEPS)


func current() -> Dictionary:
	return STEPS[clampi(step, 0, count() - 1)]


func is_last() -> bool:
	return step >= count() - 1


## "Tiếp": next step, or finished after the last one. Returns true while the guide is still open.
func next() -> bool:
	if is_last():
		finished = true
	else:
		step += 1
	return not finished


## "Bỏ qua": closes the guide at once.
func skip() -> void:
	finished = true


func restart() -> void:
	step = 0
	finished = false
