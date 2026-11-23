class_name MiniGameFeedbackSfx
extends RefCounted
## Short right/wrong sounds of the mini-games (F073-AC-24, NFR-A11Y-001: a sound and an icon with every verdict, so
## colour is never the only signal). Generated once as tiny PCM clips (no audio asset to ship or load): a rising two-note
## chime for "correct", a soft low note for "try again", a neutral tick for "answer saved". Played on the `SFX` bus
## (its volume follows the player's sound settings, AudioDirector).

const KIND_CORRECT: String = "correct"
const KIND_INCORRECT: String = "incorrect"
const KIND_SAVED: String = "saved"
const BUS: StringName = &"SFX"
const MIX_RATE: int = 22050
## Notes of each clip: [frequency Hz, seconds].
const NOTES: Dictionary = {
	"correct": [[660.0, 0.09], [990.0, 0.14]],
	"incorrect": [[300.0, 0.22]],
	"saved": [[520.0, 0.06]],
}
const VOLUME: float = 0.35

static var _cache: Dictionary = {}


## The clip of `kind` (cached; null for an unknown kind).
static func stream(kind: String) -> AudioStreamWAV:
	if _cache.has(kind):
		return _cache[kind] as AudioStreamWAV
	if not NOTES.has(kind):
		return null
	var data: PackedByteArray = PackedByteArray()
	for note: Variant in (NOTES[kind] as Array):
		var freq: float = float((note as Array)[0])
		var count: int = int(float((note as Array)[1]) * MIX_RATE)
		for i: int in count:
			var envelope: float = 1.0 - float(i) / float(count)
			var sample: int = int(sin(TAU * freq * float(i) / MIX_RATE) * envelope * VOLUME * 32767.0)
			data.append(sample & 0xFF)
			data.append((sample >> 8) & 0xFF)
	var wav: AudioStreamWAV = AudioStreamWAV.new()
	wav.format = AudioStreamWAV.FORMAT_16_BITS
	wav.mix_rate = MIX_RATE
	wav.stereo = false
	wav.data = data
	_cache[kind] = wav
	return wav


## A player for these clips on the SFX bus (Master when the bus does not exist, e.g. in tests).
static func make_player() -> AudioStreamPlayer:
	var player: AudioStreamPlayer = AudioStreamPlayer.new()
	player.name = "FeedbackSfx"
	player.bus = BUS if AudioServer.get_bus_index(BUS) >= 0 else &"Master"
	return player
