class_name PendingCommand
extends RefCounted
## A client command waiting for `system.command.acked` / `system.command.failed` with a matching `ref`, or for its
## deadline (`client_command_timeout`, F092-AC-06). `completed` fires exactly once with the outcome.

signal completed(result: CommandResult)

var id: String = ""
var type: String = ""
var deadline_ms: int = 0
var done: bool = false


func finish(result: CommandResult) -> void:
	if done:
		return
	done = true
	completed.emit(result)
