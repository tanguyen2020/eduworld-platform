class_name ChatAssetCache
extends RefCounted
## Stickers and GIFs of the internal library (F119-AC-16, AC-17; CHAT_SYSTEM §7.5, STUDENT_GAME_UX_SOCIAL §7):
## - the sticker packs of `listStickerPacks` (locked packs keep their `unlock_hint`) and the GIF pages of `listGifs`
##   by topic `tag` — there is no Internet search and no third-party GIF service;
## - pictures are fetched only from the EduWorld CDN (`Config.cdn_base_url`); any other host is refused and the
##   bubble shows the `alt_text` (spec edge case) — never a URL typed by a student (no such field exists);
## - files are cached by URL in RAM and in `user://cache/chat_assets/` — the CDN keys are immutable
##   (`media/<sha256>/…`, ADR-0044), so the URL is the version; the cache is a disposable copy (data-model §1);
## - PNG / JPEG / WebP are decoded; a GIF shows its still `preview_url` (Godot has no GIF decoder — open question,
##   [Proposed] sprite sheets from the content pipeline).

const CACHE_DIR: String = "user://cache/chat_assets"
const RAM_MAX: int = 48
## `chat_asset_file_max` (CHAT_SYSTEM §7.5): GIF ≤ 2 MB is the largest chat asset.
const FILE_MAX_BYTES: int = 2 * 1024 * 1024

var transport: HttpTransport = null
var cdn_base_url: String = ""
## Write the disk cache (off in tests, and on the Web build, which keeps nothing in browser storage — ADR-0086).
var use_disk: bool = not OS.has_feature("web")
## Sticker packs as the server listed them: `{pack_id, code, name, unlocked, unlock_hint, stickers: [{sticker_id, url,
## alt_text}]}`.
var packs: Array[Dictionary] = []
## GIFs by topic tag ("" = the first page without a tag): `{gif_id, url, preview_url, width, height, tags}`.
var gifs_by_tag: Dictionary = {}
var _stickers: Dictionary = {}
var _gifs: Dictionary = {}
var _textures: Dictionary = {}
var _order: Array[String] = []


func _init(http: HttpTransport = null, cdn: String = "") -> void:
	transport = http
	cdn_base_url = cdn


## `listStickerPacks` data → packs and the sticker index.
func load_packs(data: Variant) -> void:
	packs.clear()
	_stickers.clear()
	if not data is Array:
		return
	for raw: Variant in data as Array:
		if not raw is Dictionary:
			continue
		var d: Dictionary = raw as Dictionary
		var pack: Dictionary = {
			"pack_id": DataRead.str_of(d, "pack_id"), "code": DataRead.str_of(d, "code").left(64),
			"name": DataRead.str_of(d, "name").left(255), "unlocked": DataRead.bool_of(d, "unlocked"),
			"unlock_hint": DataRead.str_of(d, "unlock_hint").left(1000), "stickers": [],
		}
		if not Uuid.is_valid(String(pack["pack_id"])):
			continue
		for s: Variant in DataRead.arr_of(d, "stickers"):
			if not s is Dictionary:
				continue
			var sticker: Dictionary = {"sticker_id": DataRead.str_of(s as Dictionary, "sticker_id"),
				"url": DataRead.str_of(s as Dictionary, "url"), "alt_text": DataRead.str_of(s as Dictionary, "alt_text").left(1000),
				"pack_id": pack["pack_id"], "unlocked": pack["unlocked"]}
			if Uuid.is_valid(String(sticker["sticker_id"])):
				(pack["stickers"] as Array).append(sticker)
				_stickers[sticker["sticker_id"]] = sticker
		packs.append(pack)


## A `listGifs` page for `tag` (kept per topic, deduplicated by id).
func load_gifs(tag: String, data: Variant) -> void:
	var list: Array[Dictionary] = []
	if data is Array:
		for raw: Variant in data as Array:
			if not raw is Dictionary:
				continue
			var d: Dictionary = raw as Dictionary
			var gif: Dictionary = {"gif_id": DataRead.str_of(d, "gif_id"), "url": DataRead.str_of(d, "url"),
				"preview_url": DataRead.str_of(d, "preview_url"), "width": maxi(1, DataRead.int_of(d, "width", 1)),
				"height": maxi(1, DataRead.int_of(d, "height", 1)), "tags": DataRead.strings_of(d, "tags", 10)}
			if Uuid.is_valid(String(gif["gif_id"])):
				list.append(gif)
				_gifs[gif["gif_id"]] = gif
	gifs_by_tag[tag] = list


## Topic chips of the GIF tab: the tags of the GIFs the server returned (server data, no free search).
func topics() -> PackedStringArray:
	var out: PackedStringArray = []
	for list: Variant in gifs_by_tag.values():
		for gif: Dictionary in list as Array:
			for t: String in gif["tags"] as PackedStringArray:
				if not t.is_empty() and not t in out:
					out.append(t)
	return out


func gifs(tag: String) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for g: Variant in gifs_by_tag.get(tag, []) as Array:
		out.append(g as Dictionary)
	return out


func sticker(sticker_id: String) -> Dictionary:
	return _stickers.get(sticker_id, {}) as Dictionary


func gif(gif_id: String) -> Dictionary:
	return _gifs.get(gif_id, {}) as Dictionary


## Only the EduWorld CDN serves chat pictures (F119-AC-16).
func is_allowed(url: String) -> bool:
	if url.is_empty() or cdn_base_url.is_empty():
		return false
	var base: String = cdn_base_url.trim_suffix("/") + "/"
	return url.begins_with(base) and not url.contains("..")


## Coroutine: the texture of `url` (RAM → disk → CDN), null when refused, missing or not decodable.
func texture(url: String) -> Texture2D:
	if not is_allowed(url):
		return null
	if _textures.has(url):
		return _textures[url] as Texture2D
	var data: PackedByteArray = _read_disk(url)
	if data.is_empty():
		if transport == null:
			return null
		var response: HttpTransport.Response = await transport.send(HTTPClient.METHOD_GET, url, PackedStringArray(), "")
		if response.result != HTTPRequest.RESULT_SUCCESS or response.status != 200 or response.body.size() > FILE_MAX_BYTES:
			return null
		data = response.body
		_write_disk(url, data)
	var tex: Texture2D = decode(data)
	if tex != null:
		_remember(url, tex)
	return tex


## PNG / JPEG / WebP bytes → texture (null otherwise, e.g. a raw GIF).
static func decode(data: PackedByteArray) -> Texture2D:
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


func cached_texture(url: String) -> Texture2D:
	return _textures.get(url) as Texture2D


func clear() -> void:
	packs.clear()
	gifs_by_tag.clear()
	_stickers.clear()
	_gifs.clear()
	_textures.clear()
	_order.clear()


func _remember(url: String, tex: Texture2D) -> void:
	_textures[url] = tex
	_order.append(url)
	while _order.size() > RAM_MAX:
		_textures.erase(_order.pop_front())


func _disk_path(url: String) -> String:
	return "%s/%s.bin" % [CACHE_DIR, url.sha256_text()]


func _read_disk(url: String) -> PackedByteArray:
	if not use_disk:
		return PackedByteArray()
	var path: String = _disk_path(url)
	return FileAccess.get_file_as_bytes(path) if FileAccess.file_exists(path) else PackedByteArray()


func _write_disk(url: String, data: PackedByteArray) -> void:
	if not use_disk:
		return
	DirAccess.make_dir_recursive_absolute(CACHE_DIR)
	var f: FileAccess = FileAccess.open(_disk_path(url), FileAccess.WRITE)
	if f != null:
		f.store_buffer(data)
