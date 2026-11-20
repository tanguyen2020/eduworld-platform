class_name AudioFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): `audio_lesson` attempts for the F066 client tests, derived from the contract fixture of
## `Attempt` with the render payload of `audio-lesson.schema.json#/$defs/render` and the media of ActivityRender (Ogg
## Vorbis, transcript). Signed URLs are fake (`example.invalid`).

const CDN: String = "https://cdn.example.invalid/ew-media-private/media/" + VideoFixtures.SHA
const TRANSCRIPT: String = "A: Hello! My name is Mai.\nB: Hi Mai, I am Nam."


static func payload(max_plays: Variant, plays_used: int, show_transcript: String, speed: bool) -> Dictionary:
	return {
		"duration_ms": 60000, "heartbeat_seconds": 10,
		"segments": [{"segment_id": "greet", "title": {"vi": "Chào hỏi"}, "start_ms": 0, "end_ms": 30000}],
		"questions": [{"cue_id": "q02", "position": 1, "at_ms": 20000}, {"cue_id": "q01", "position": 2}],
		"interaction": {"max_plays": max_plays, "allow_speed_change": speed, "show_transcript": show_transcript,
			"playback_rates": [0.75, 1] if speed else [1]},
		"plays_used": plays_used, "completion": {"min_watched_ratio": 0.9},
	}


## An audio attempt with two single-choice items (timed at 20 s, then after the end).
static func attempt(max_plays: Variant = 2, plays_used: int = 0, show_transcript: String = "always", speed: bool = true,
		resume_ms: int = 0) -> Dictionary:
	var a: Dictionary = LearningFixtures.fresh_attempt()
	a["activity_type"] = "audio_lesson"
	var first: Dictionary = LearningFixtures.item_of("single_choice").duplicate(true)
	var second: Dictionary = first.duplicate(true)
	second["position"] = 2
	a["items"] = [first, second]
	var render: Dictionary = (a["render"] as Dictionary).duplicate(true)
	render["activity_type"] = "audio_lesson"
	render["render_payload"] = payload(max_plays, plays_used, show_transcript, speed)
	var media: Array = [{"asset_id": "0f660000-0000-4000-8000-000000000001", "url": CDN + "/ogg.ogg?expires=1&md5=x", "kind": "audio",
		"variant": "ogg"}]
	if show_transcript == "always":
		media.append({"asset_id": "0f660000-0000-4000-8000-000000000002", "url": CDN + "/transcript.txt?expires=1", "kind": "transcript",
			"variant": "original"})
	render["media"] = media
	a["render"] = render
	a["resume_position_ms"] = resume_ms
	a["progress"] = {"position_ms": resume_ms, "watched_ratio": 0, "completed": false, "watched_ranges": [], "pending_cue_ids": [],
		"plays_used": plays_used, "max_plays": max_plays}
	return a
