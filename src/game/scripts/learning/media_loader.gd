class_name MediaLoader
extends RefCounted
## Images and audio of question items (`QuestionRender.media[]` signed CDN URLs, ADR-0044, F063-AC-04). Kept in a
## small RAM cache for the attempt only (nothing on disk); the URL is never logged (it is signed).

const CACHE_MAX: int = 24
## Largest media accepted in RAM (images, voice-over clips).
const MAX_BYTES: int = 8 * 1024 * 1024

var transport: HttpTransport = null
var _cache: Dictionary = {}
var _order: Array[String] = []


func _init(http: HttpTransport = null) -> void:
	transport = http


## Coroutine: raw bytes of `url`, empty on failure.
func bytes(url: String) -> PackedByteArray:
	if url.is_empty():
		return PackedByteArray()
	if _cache.has(url):
		return _cache[url] as PackedByteArray
	if transport == null:
		return PackedByteArray()
	var response: HttpTransport.Response = await transport.send(HTTPClient.METHOD_GET, url, PackedStringArray(), "")
	if response.result != HTTPRequest.RESULT_SUCCESS or response.status != 200 or response.body.size() > MAX_BYTES:
		return PackedByteArray()
	_cache[url] = response.body
	_order.append(url)
	while _order.size() > CACHE_MAX:
		_cache.erase(_order.pop_front())
	return response.body


## Coroutine: a PNG/JPEG/WebP image as a texture, null when it cannot be decoded.
func texture(url: String) -> Texture2D:
	var data: PackedByteArray = await bytes(url)
	if data.size() < 12:
		return null
	var image: Image = Image.new()
	var err: Error = ERR_FILE_UNRECOGNIZED
	if data[0] == 0x89 and data[1] == 0x50:
		err = image.load_png_from_buffer(data)
	elif data[0] == 0xFF and data[1] == 0xD8:
		err = image.load_jpg_from_buffer(data)
	elif data.slice(8, 12).get_string_from_ascii() == "WEBP":
		err = image.load_webp_from_buffer(data)
	return ImageTexture.create_from_image(image) if err == OK else null


## Coroutine: an Ogg Vorbis clip, null when it cannot be decoded.
func audio(url: String) -> AudioStream:
	var data: PackedByteArray = await bytes(url)
	if data.size() < 4 or data.slice(0, 4).get_string_from_ascii() != "OggS":
		return null
	return AudioStreamOggVorbis.load_from_buffer(data)


func clear() -> void:
	_cache.clear()
	_order.clear()
