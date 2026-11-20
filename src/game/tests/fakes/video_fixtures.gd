class_name VideoFixtures
extends RefCounted
## TEST-ONLY (AGENTS R17): `video_lesson` attempts for the F065 client tests, derived from the contract fixture of
## `Attempt` (OpenAPI example) with the render payload of `video-lesson.schema.json#/$defs/render` and the media list of
## ActivityRender (variants, subtitles). Signed URLs are fake (`example.invalid`).

const SHA: String = "0f65aaaa0000000000000000000000000000000000000000000000000000beef"
const CDN: String = "https://cdn.example.invalid/ew-media-private/media/"
const VTT_VI: String = "WEBVTT\n\nintro\n00:00.000 --> 00:04.000\nXin chào các em!\n\n00:05.000 --> 00:08.500\n<v Cô>Hôm nay mình học\nphép nhân.</v>\n"
const VTT_EN: String = "WEBVTT\n\n00:00.000 --> 00:04.000\nHello everyone!\n"
const DOWNLOAD_DIR: String = "user://cache/media/" + SHA


static func media() -> Array:
	return [
		{"asset_id": "0f650000-0000-4000-8000-000000000001", "url": CDN + SHA + "/ogv_720.ogv?expires=1&md5=x", "kind": "video", "variant": "ogv_720"},
		{"asset_id": "0f650000-0000-4000-8000-000000000001", "url": CDN + SHA + "/ogv_480.ogv?expires=1&md5=x", "kind": "video", "variant": "ogv_480"},
		{"asset_id": "0f650000-0000-4000-8000-000000000003", "url": CDN + SHA + "/water_480.ogv?expires=1", "kind": "video", "variant": "ogv_480",
			"segment_id": "water"},
		{"asset_id": "0f650000-0000-4000-8000-000000000002", "url": CDN + SHA + "/vi.vtt?expires=1", "kind": "subtitle", "variant": "vtt",
			"language": "vi"},
		{"asset_id": "0f650000-0000-4000-8000-000000000004", "url": CDN + SHA + "/en.vtt?expires=1", "kind": "subtitle", "variant": "vtt",
			"language": "en"},
	]


static func payload(branched: bool = false) -> Dictionary:
	var p: Dictionary = {
		"duration_ms": 100000, "heartbeat_seconds": 10,
		"chapters": [{"chapter_id": "intro", "title": {"vi": "Mở đầu"}, "start_ms": 0, "end_ms": 50000},
			{"chapter_id": "main", "title": {"vi": "Nội dung"}, "start_ms": 50000, "end_ms": 100000}],
		"cues": [{"cue_id": "q1", "at_ms": 30000, "position": 1, "required": true, "allow_skip_after_wrong": true, "pause": true}],
		"branches": [], "segments": [],
		"subtitles": [{"language": "vi", "is_default": true}, {"language": "en", "is_default": false}],
		"interaction": {"allow_seek": "watched_only", "playback_rates": [1, 1.5], "allow_subtitles_toggle": true, "autoplay_next_chapter": true},
		"completion": {"min_watched_ratio": 0.9, "require_required_cues": true},
	}
	if branched:
		p["cues"] = []
		p["segments"] = [{"segment_id": "water", "is_ending": true}, {"segment_id": "land", "is_ending": true}]
		p["branches"] = [{"branch_point_id": "bp1", "at_ms": 20000, "prompt": {"vi": "Vịt đi đâu?"},
			"options": [{"option_id": "swim", "label": {"vi": "Xuống nước"}}, {"option_id": "walk", "label": {"vi": "Lên bờ"}}]}]
	return p


## A video attempt: one single-choice cue item at position 1 (or no cue with branches).
static func attempt(branched: bool = false, resume_ms: int = 0) -> Dictionary:
	var a: Dictionary = LearningFixtures.fresh_attempt()
	a["activity_type"] = "video_lesson"
	a["items"] = [] if branched else [LearningFixtures.item_of("single_choice").duplicate(true)]
	var render: Dictionary = (a["render"] as Dictionary).duplicate(true)
	render["activity_type"] = "video_lesson"
	render["render_payload"] = payload(branched)
	render["media"] = media()
	a["render"] = render
	a["resume_position_ms"] = resume_ms
	a["progress"] = {"position_ms": resume_ms, "watched_ratio": 0, "completed": false,
		"watched_ranges": [{"start_ms": 0, "end_ms": resume_ms}] if resume_ms > 0 else [], "pending_cue_ids": [] if branched or resume_ms > 0 else ["q1"]}
	return a


## TEST downloader of VideoFileCache: writes a placeholder file instead of downloading.
static func fake_download(_url: String, path: String) -> bool:
	DirAccess.make_dir_recursive_absolute(path.get_base_dir())
	var f: FileAccess = FileAccess.open(path, FileAccess.WRITE)
	if f == null:
		return false
	f.store_string("ogv")
	f.close()
	return true


static func cleanup() -> void:
	var dir: DirAccess = DirAccess.open(DOWNLOAD_DIR)
	if dir == null:
		return
	for f: String in dir.get_files():
		DirAccess.remove_absolute(DOWNLOAD_DIR.path_join(f))
	DirAccess.remove_absolute(DOWNLOAD_DIR)
