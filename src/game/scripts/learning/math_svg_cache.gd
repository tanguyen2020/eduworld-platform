class_name MathSvgCache
extends RefCounted
## Formula SVGs (ADR-0055, GODOT_ARCHITECTURE §6.3, F063-AC-18/20). Downloads `math/<sha256>.svg` from the CDN,
## checks its size (`math_svg_max`) and its sha256, keeps it as an immutable file `user://cache/math/<sha256>.svg`
## (a formula is public, versioned content — never an answer, ADR-0044) and rasterizes it once per
## `(sha256, scale, colour)` into a texture tinted with the Theme text colour (P-2: no re-rasterizing per frame).
## Any failure returns null: the caller shows `speech_text` instead, and `failure_handler` sends telemetry.

const CACHE_DIR: String = "user://cache/math"
## CONFIG_DEFAULTS `math_svg_max` (256 KB, scope code, ADR-0055).
const MAX_SVG_BYTES: int = 256 * 1024
## Pixels of one `ex` at scale 1 (MathJax sizes its SVG in `ex`; 1 ex ≈ half of the 16 px reference font).
const EX_PX: float = 8.0
const REFERENCE_FONT_PX: float = 16.0
const TEXTURE_CACHE_MAX: int = 128
const FAIL_DOWNLOAD: String = "download"
const FAIL_TOO_LARGE: String = "too_large"
const FAIL_SHA_MISMATCH: String = "sha_mismatch"
const FAIL_RASTERIZE: String = "rasterize"
const EX_ATTRIBUTE_PATTERN: String = "(width|height)=\"([0-9.]+)ex\""

var transport: HttpTransport = null
var cdn_base_url: String = ""
var cache_dir: String = CACHE_DIR
## `(reason: String) -> void`, called once per failed formula (telemetry `math_svg_failed`).
var failure_handler: Callable = Callable()
var _textures: Dictionary = {}
var _order: Array[String] = []


func _init(http: HttpTransport = null, cdn_base: String = "") -> void:
	transport = http
	cdn_base_url = cdn_base


## Raster scale of a formula drawn next to `font_px` text on a screen with `screen_scale` (DPI × density).
static func scale_for(font_px: int, screen_scale: float = 1.0) -> float:
	return maxf(0.25, float(font_px) / REFERENCE_FONT_PX * maxf(screen_scale, 0.25))


## Tints `currentColor` with `color` and turns the root `ex` sizes into pixels for the rasterizer.
static func prepare_svg(svg: String, color: Color) -> String:
	var out: String = svg.replace("currentColor", "#" + color.to_html(false))
	var re: RegEx = RegEx.create_from_string(EX_ATTRIBUTE_PATTERN)
	for m: RegExMatch in re.search_all(out):
		out = out.replace(m.get_string(0), "%s=\"%.2f\"" % [m.get_string(1), m.get_string(2).to_float() * EX_PX])
	return out


static func sha256_hex(bytes: PackedByteArray) -> String:
	var ctx: HashingContext = HashingContext.new()
	ctx.start(HashingContext.HASH_SHA256)
	ctx.update(bytes)
	return ctx.finish().hex_encode()


func cache_path(sha256: String) -> String:
	return cache_dir.path_join("%s.svg" % sha256)


## Coroutine: the SVG text of `asset` (disk cache first), empty on failure.
func svg_text(asset: MathAssetRefModel) -> String:
	if asset == null:
		return ""
	if not asset.sha256.is_empty() and FileAccess.file_exists(cache_path(asset.sha256)):
		var cached: PackedByteArray = FileAccess.get_file_as_bytes(cache_path(asset.sha256))
		if sha256_hex(cached) == asset.sha256:
			return cached.get_string_from_utf8()
		DirAccess.remove_absolute(cache_path(asset.sha256))
	if transport == null:
		_fail(FAIL_DOWNLOAD)
		return ""
	var response: HttpTransport.Response = await transport.send(HTTPClient.METHOD_GET, asset.url(cdn_base_url),
			PackedStringArray(["Accept: image/svg+xml"]), "")
	if response.result != HTTPRequest.RESULT_SUCCESS or response.status != 200:
		_fail(FAIL_DOWNLOAD)
		return ""
	if response.body.size() > MAX_SVG_BYTES:
		_fail(FAIL_TOO_LARGE)
		return ""
	if not asset.sha256.is_empty():
		if sha256_hex(response.body) != asset.sha256:
			_fail(FAIL_SHA_MISMATCH)
			return ""
		DirAccess.make_dir_recursive_absolute(cache_dir)
		var file: FileAccess = FileAccess.open(cache_path(asset.sha256), FileAccess.WRITE)
		if file != null:
			file.store_buffer(response.body)
			file.close()
	return response.body.get_string_from_utf8()


## Coroutine: the formula rasterized at `scale` in `color` (cached per `(sha256, scale, colour)`), null on failure.
func texture(asset: MathAssetRefModel, scale: float, color: Color) -> Texture2D:
	if asset == null:
		return null
	var key: String = texture_key(asset, scale, color)
	if _textures.has(key):
		return _textures[key] as Texture2D
	var svg: String = await svg_text(asset)
	if svg.is_empty():
		return null
	var image: Image = Image.new()
	if image.load_svg_from_string(prepare_svg(svg, color), scale) != OK or image.is_empty():
		_fail(FAIL_RASTERIZE)
		return null
	var tex: ImageTexture = ImageTexture.create_from_image(image)
	_textures[key] = tex
	_order.append(key)
	while _order.size() > TEXTURE_CACHE_MAX:
		_textures.erase(_order.pop_front())
	return tex


static func texture_key(asset: MathAssetRefModel, scale: float, color: Color) -> String:
	var id: String = asset.sha256 if not asset.sha256.is_empty() else asset.url("")
	return "%s|%.2f|%s" % [id, scale, color.to_html(true)]


func cached_texture_count() -> int:
	return _textures.size()


func _fail(reason: String) -> void:
	if failure_handler.is_valid():
		failure_handler.call(reason)
