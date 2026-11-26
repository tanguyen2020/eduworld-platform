extends GdUnitTestSuite
## F096-AC-01, AC-03, AC-05, AC-06 (owner 2026-11-26, ADR-0102, GODOT_ARCHITECTURE §5): no Godot branding — the boot splash
## and the app icons are the EduWorld images rendered by tools/brand_assets.gd (sizes, settings, licence row all match:
## `brand_assets.gd --check` returns no error), window/tab title "EduWorld", and the screens before the world (in-game
## splash, "Đang vào lớp…", giờ nghỉ, hàng chờ) show the S01 sky, the EduWorld logo and Mây.

const BrandAssets := preload("res://tools/brand_assets.gd")
const ENTRY_SCENES: PackedStringArray = [
	"res://scenes/bootstrap/Splash.tscn", "res://scenes/auth/ResumeSplash.tscn", "res://scenes/auth/CurfewNotice.tscn",
	"res://scenes/auth/LoginWaiting.tscn",
]


func test_brand_assets_check_is_clean() -> void:
	var errors: PackedStringArray = BrandAssets.check()
	assert_array(errors).override_failure_message("\n".join(errors)).is_empty()


func test_boot_splash_and_icon_are_eduworld() -> void:
	assert_str(String(ProjectSettings.get_setting("application/boot_splash/image"))).is_equal(BrandAssets.SPLASH)
	assert_bool(bool(ProjectSettings.get_setting("application/boot_splash/show_image"))).is_true()
	assert_bool(bool(ProjectSettings.get_setting("application/boot_splash/use_filter"))).is_true()
	var bg: Color = ProjectSettings.get_setting("application/boot_splash/bg_color") as Color
	assert_bool(BrandAssets._same_color(bg, UiTokens.COMPONENT_BACKDROP_SKY_TOP)).is_true()
	assert_str(String(ProjectSettings.get_setting("application/config/icon"))).is_equal(BrandAssets.ICON)
	assert_str(String(ProjectSettings.get_setting("application/config/name"))).is_equal("EduWorld")
	var presets: ConfigFile = ConfigFile.new()
	assert_int(presets.load("res://export_presets.cfg")).is_equal(OK)
	assert_bool(bool(presets.get_value("preset.2.options", "html/export_icon"))).is_true()
	assert_str(String(presets.get_value("preset.0.options", "launcher_icons/main_192x192"))).is_equal(BrandAssets.ICON_ANDROID)
	assert_str(String(presets.get_value("preset.1.options", "icons/icon_1024x1024"))).is_equal(BrandAssets.ICON_IOS)


func test_icons_are_square_and_the_ios_icon_is_opaque() -> void:
	var ios: Image = Image.load_from_file(ProjectSettings.globalize_path(BrandAssets.ICON_IOS))
	assert_bool(ios.detect_alpha() == Image.ALPHA_NONE or ios.get_pixel(0, 0).a > 0.99).is_true()
	var icon: Image = Image.load_from_file(ProjectSettings.globalize_path(BrandAssets.ICON))
	assert_int(icon.get_width()).is_equal(icon.get_height())
	# Rounded corners of the project icon (Web favicon / apple-touch-icon) are transparent, the centre is not.
	assert_float(icon.get_pixel(0, 0).a).is_less(0.1)
	assert_float(icon.get_pixel(512, 512).a).is_greater(0.9)


func test_entry_screens_show_the_sky_logo_and_mascot() -> void:
	for path: String in ENTRY_SCENES:
		var screen: Control = auto_free((load(path) as PackedScene).instantiate()) as Control
		var scripts: PackedStringArray = []
		for node: Node in screen.find_children("*", "", true, false):
			var script: Script = node.get_script() as Script
			if script != null:
				scripts.append(script.resource_path.get_file())
		for expected: String in ["sky_backdrop.gd", "brand_lockup.gd", "cloudy_mascot.gd", "spark_mark.gd"]:
			assert_array(scripts).override_failure_message("%s lacks %s" % [path, expected]).contains([expected])
