extends RefCounted
## Layer rule (GODOT_ARCHITECTURE §1 G-4, §6; AGENTS R1; F060-AC-05): UI scenes and gameplay scripts never open a
## network connection nor call the network services directly — they emit intents to services.

const ToolFiles := preload("res://tools/lib/tool_files.gd")
const NETWORK_TYPES: PackedStringArray = [
	"HTTPRequest", "HTTPClient", "WebSocketPeer", "WebSocketMultiplayerPeer", "StreamPeerTCP", "StreamPeerTLS",
	"TCPServer", "PacketPeerUDP", "UDPServer", "ENetConnection", "ENetMultiplayerPeer", "JavaScriptBridge",
]
const NETWORK_SERVICES: PackedStringArray = ["ApiClient", "RealtimeClient"]
## Layers checked: every scene script (UI + gameplay scenes) and the gameplay script folders.
const CHECKED_PREFIXES: PackedStringArray = [
	"res://scenes/", "res://scripts/world/", "res://scripts/player/",
]


static func applies_to(path: String) -> bool:
	for prefix: String in CHECKED_PREFIXES:
		if path.begins_with(prefix):
			return true
	return false


static func check(path: String, text: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var lines: PackedStringArray = text.split("\n")
	for i: int in lines.size():
		var line: String = lines[i]
		if path.ends_with(".tscn"):
			for type_name: String in NETWORK_TYPES:
				if line.begins_with("[node ") and line.contains("type=\"%s\"" % type_name):
					out.append("%s:%d network node %s in a UI/gameplay scene (R1, G-4)" % [path, i + 1, type_name])
			continue
		var code: String = ToolFiles.strip_strings(ToolFiles.strip_comment(line))
		for type_name: String in NETWORK_TYPES:
			if _has_identifier(code, type_name):
				out.append("%s:%d uses %s in a UI/gameplay script (R1, G-4)" % [path, i + 1, type_name])
		for service: String in NETWORK_SERVICES:
			if _has_identifier(code, service):
				out.append("%s:%d calls %s directly from a UI/gameplay script (R1, G-4)" % [path, i + 1, service])
	return out


static func _has_identifier(code: String, identifier: String) -> bool:
	var regex: RegEx = RegEx.create_from_string("(?<![A-Za-z0-9_])%s(?![A-Za-z0-9_])" % identifier)
	return regex.search(code) != null
