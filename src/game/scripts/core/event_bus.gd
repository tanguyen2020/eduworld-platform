extends Node
## Autoload #3 `EventBus` (GODOT_ARCHITECTURE §4, §7, F060-AC-06).
## The fixed list of global, cross-cutting signals. Names are past tense with typed arguments; adding a signal is a
## reviewed change (src/game/tools/lint_signals.gd checks the naming). Local signals stay on their own nodes.

@warning_ignore("unused_signal")
signal session_expired()
@warning_ignore("unused_signal")
signal network_state_changed(state: int)
@warning_ignore("unused_signal")
signal notification_received(payload: Dictionary)
@warning_ignore("unused_signal")
signal zone_changed(area_id: String)
@warning_ignore("unused_signal")
signal locale_changed(locale: String)
@warning_ignore("unused_signal")
signal density_changed(band: String)
@warning_ignore("unused_signal")
signal ui_requested(screen_id: StringName, args: Dictionary)
