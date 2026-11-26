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
@warning_ignore("unused_signal")
signal ui_closed(screen_id: StringName)
## F096: "Xem lại hướng dẫn của Mây" in Cài đặt — the world host opens the Hub guide again.
signal onboarding_requested()
## F102: a party invitation arrived (realtime `party.invitation.received`) — the HUD shows the InvitationToast.
@warning_ignore("unused_signal")
signal party_invitation_received(invitation: InvitationModel)
## F069: the S07 level-up modal was shown (accessibility announce, analytics without PII).
@warning_ignore("unused_signal")
signal level_up_shown(to_level: int)
## F069: the Xu balance changed (server value: `getWallet` or `balance_after`).
@warning_ignore("unused_signal")
signal wallet_changed(balance: int)
