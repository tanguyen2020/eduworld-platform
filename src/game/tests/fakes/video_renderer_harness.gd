class_name VideoRendererHarness
extends RefCounted
## TEST-ONLY (AGENTS R17): a VideoLessonRenderer on a FakeVideoPlayback, a VideoFileCache whose downloader writes a
## placeholder file, and a MediaLoader on FakeHttp serving the WebVTT fixtures; records the renderer's signals.

const SCENE: String = "res://scenes/learning/activity_renderers/video_lesson/VideoLessonRenderer.tscn"

var renderer: VideoLessonRenderer = null
var playback: FakeVideoPlayback = FakeVideoPlayback.new()
var http: FakeHttp = FakeHttp.new()
var reports: Array[Dictionary] = []
var submissions: Array[Dictionary] = []
var finished: int = 0
var shown: Array[int] = []


## Builds and sets up the renderer under `parent` (awaits the first load).
static func create(parent: Node, data: Dictionary) -> VideoRendererHarness:
	var h: VideoRendererHarness = VideoRendererHarness.new()
	h.http.reply_bytes("GET https://cdn.example.invalid/ew-media-private/media/*/vi.vtt", 200, VideoFixtures.VTT_VI.to_utf8_buffer())
	h.http.reply_bytes("GET https://cdn.example.invalid/ew-media-private/media/*/en.vtt", 200, VideoFixtures.VTT_EN.to_utf8_buffer())
	h.renderer = (load(SCENE) as PackedScene).instantiate() as VideoLessonRenderer
	h.renderer.playback = h.playback
	parent.add_child(h.renderer)
	h.renderer.progress_reported.connect(func(r: Dictionary) -> void: h.reports.append(r))
	h.renderer.response_submitted.connect(func(s: Dictionary) -> void: h.submissions.append(s))
	h.renderer.finish_requested.connect(func() -> void: h.finished += 1)
	h.renderer.item_shown.connect(func(p: int) -> void: h.shown.append(p))
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = "vi"
	ctx.video = VideoFileCache.new()
	ctx.video.downloader = VideoFixtures.fake_download
	ctx.media = MediaLoader.new(h.http)
	await h.renderer.setup(AttemptModel.from_dict(data), ctx)
	return h


## Plays `ms` of video in steps of one second (one _process call per step).
func play_for(ms: int) -> void:
	var step: int = 1000
	var done: int = 0
	while done < ms:
		playback.advance(step)
		renderer._process(1.0)
		done += step
		if not playback.playing:
			return


func dispose() -> void:
	if renderer != null:
		renderer.teardown()
		renderer.free()
	VideoFixtures.cleanup()
