extends Control
## UiGallery (debug builds only, DESIGN_SYSTEM §9, F060-AC-10, F060-AC-14, F060-AC-17): every base component under
## the generated Theme; buttons switch density, locale (vi / en / xx-PS) and high contrast at runtime. The scene
## only calls service methods (AppState, Settings); it holds no business logic.

const DENSITY_BUTTONS: Dictionary = {"g1_3": "%DensityG13", "g4_5": "%DensityG45", "g6_7": "%DensityG67", "g8_9": "%DensityG89"}
const SAMPLE_UNIX_TIME: int = 1794753000
const SAMPLE_NUMBER: float = 1234.5
const SAMPLE_DURATION: int = 200

@onready var _format_sample: Label = %FormatSample


func _ready() -> void:
	if AppState.theme_service == null:
		AppState.start_ui(get_tree().root)
	for band: String in DENSITY_BUTTONS.keys():
		(get_node(String(DENSITY_BUTTONS[band])) as Button).pressed.connect(set_density.bind(band))
	(%LocaleVi as Button).pressed.connect(set_locale.bind("vi-VN"))
	(%LocaleEn as Button).pressed.connect(set_locale.bind("en-US"))
	(%LocalePseudo as Button).pressed.connect(set_locale.bind(L10n.pseudo_locale()))
	(%HighContrast as CheckButton).toggled.connect(set_high_contrast)
	EventBus.locale_changed.connect(_on_locale_changed)
	_refresh_format_sample()


func set_density(band: String) -> void:
	AppState.set_grade_band(band)


func set_locale(locale: String) -> void:
	Settings.set_locale(locale, false)


func set_high_contrast(enabled: bool) -> void:
	AppState.theme_service.set_high_contrast(enabled)


func _on_locale_changed(_locale: String) -> void:
	_refresh_format_sample()


func _refresh_format_sample() -> void:
	var moment: Dictionary = Time.get_datetime_dict_from_unix_time(SAMPLE_UNIX_TIME)
	_format_sample.text = tr("ui.gallery.format_sample").format({
		"number": LocaleFormat.number(SAMPLE_NUMBER),
		"date": LocaleFormat.date(moment),
		"time": LocaleFormat.time(moment),
		"duration": LocaleFormat.duration(SAMPLE_DURATION),
	})
