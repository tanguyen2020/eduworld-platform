@tool
extends EditorPlugin
## Registers the export plugin that ships the Android Keystore plugin AAR (F060 T053, ADR-0086).

const ExportPlugin := preload("res://addons/eduworld_platform/export_plugin.gd")

var _export_plugin: EditorExportPlugin = null


func _enter_tree() -> void:
	_export_plugin = ExportPlugin.new()
	add_export_plugin(_export_plugin)


func _exit_tree() -> void:
	if _export_plugin != null:
		remove_export_plugin(_export_plugin)
		_export_plugin = null
