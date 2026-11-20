class_name AudioRendererHarness
extends RefCounted
## TEST-ONLY (AGENTS R17): an AudioLessonRenderer on a FakeAudioPlayback, a VideoFileCache whose downloader writes a
## placeholder file, and a MediaLoader on FakeHttp serving the transcript; records the renderer's signals.

const SCENE: String = "res://scenes/learning/activity_renderers/audio_lesson/AudioLessonRenderer.tscn"

var renderer: AudioLessonRenderer = null
var playback: FakeAudioPlayback = FakeAudioPlayback.new()
var http: FakeHttp = FakeHttp.new()
var reports: Array[Dictionary] = []
var submissions: Array[Dictionary] = []
var finished: int = 0


static func create(parent: Node, data: Dictionary) -> AudioRendererHarness:
	var h: AudioRendererHarness = AudioRendererHarness.new()
	h.http.reply_bytes("GET https://cdn.example.invalid/ew-media-private/media/*/transcript.txt", 200, AudioFixtures.TRANSCRIPT.to_utf8_buffer())
	h.renderer = (load(SCENE) as PackedScene).instantiate() as AudioLessonRenderer
	h.renderer.playback = h.playback
	parent.add_child(h.renderer)
	h.renderer.progress_reported.connect(func(r: Dictionary) -> void: h.reports.append(r))
	h.renderer.response_submitted.connect(func(s: Dictionary) -> void: h.submissions.append(s))
	h.renderer.finish_requested.connect(func() -> void: h.finished += 1)
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = "en"
	ctx.video = VideoFileCache.new()
	ctx.video.downloader = VideoFixtures.fake_download
	ctx.media = MediaLoader.new(h.http)
	await h.renderer.setup(AttemptModel.from_dict(data), ctx)
	return h


## The server answers the last report as ActivityProgressState with `plays_used`.
func accept(plays_used: int, max_plays: Variant = 2) -> void:
	renderer.apply_progress({"position_ms": 0, "watched_ratio": 0, "completed": false, "watched_ranges": [], "pending_cue_ids": [],
		"plays_used": plays_used, "max_plays": max_plays})


func play_for(ms: int) -> void:
	var done: int = 0
	while done < ms:
		playback.advance(1000)
		renderer._process(1.0)
		done += 1000
		if not playback.playing:
			return


func dispose() -> void:
	if renderer != null:
		renderer.teardown()
		renderer.free()
	VideoFixtures.cleanup()
