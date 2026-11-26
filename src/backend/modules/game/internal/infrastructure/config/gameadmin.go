package config

import (
	"errors"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS for the Game Operations Console (F078 T003; owners CONTENT_MODEL §5–8, ADMIN_PORTAL §5).
const (
	DefaultScheduledPublishMinLeadMinutes = 5  // scheduled_publish_min_lead
	DefaultAssetsPerVersionMax            = 50 // assets_per_version_max
	DefaultPreviewTokenTTLMinutes         = 30 // preview_token_ttl
	DefaultRenderMaxAttempts              = 5  // game_render_max_attempts [Proposed F078]
	DefaultRenderBackoffSeconds           = 60 // game_render_retry_backoff [Proposed F078]
	ScheduledPublishInterval              = 30 * time.Second
	MediaPipelineInterval                 = 30 * time.Second
	RenderRetryInterval                   = time.Minute
	GameAdminJobBatch                     = 50
	// DefaultReportLinkTTLMinutes is report_link_ttl (15 minutes, owner SCHOOL_LMS): the signed download link of a
	// translation file (F082 Clarifications 2026-11-16).
	DefaultReportLinkTTLMinutes = 15
	// TranslationJobsInterval and TranslationBackfillInterval are the ticks of the translation jobs (F082).
	TranslationJobsInterval     = 5 * time.Second
	TranslationBackfillInterval = time.Minute
)

// DefaultTextRequiredLanguages is game_text_required_languages (vi, en; ADR-0056).
var DefaultTextRequiredLanguages = []string{"vi", "en"}

// GameAdmin is the configuration of the console, the game media pipeline and the renders.
type GameAdmin struct {
	Policy            command.GameAdminPolicy
	RenderMaxAttempts int
	RenderBackoff     time.Duration
	// S3 is the object storage of game media (EW_S3_*); empty endpoint = presign answers 503 and the pipeline is idle.
	S3Endpoint, S3Region, S3AccessKey, S3SecretKey string
	S3Timeout                                      time.Duration
	// CDNBase is EW_CDN_BASE_URL (CDN URLs of ready media and renders; empty = no URL).
	CDNBase string
	// ReportLinkTTL is report_link_ttl (EW_GAME_REPORT_LINK_TTL_MINUTES): validity of a translation file download link.
	ReportLinkTTL time.Duration
	// ClamdAddr is the clamd address of EW_MEDIA_SCAN=clamd://host:port ("" = EW_MEDIA_SCAN=disabled, local only).
	ClamdAddr string
}

// GameAdminFrom reads EW_GAME_SCHEDULED_PUBLISH_MIN_LEAD_MINUTES, EW_GAME_TEXT_REQUIRED_LANGUAGES,
// EW_GAME_ASSETS_PER_VERSION_MAX, EW_GAME_PREVIEW_TOKEN_TTL_MINUTES, EW_GAME_RENDER_MAX_ATTEMPTS,
// EW_GAME_RENDER_BACKOFF_SECONDS and the shared EW_S3_*, EW_CDN_BASE_URL, EW_MEDIA_SCAN (CONFIG_DEFAULTS values when
// unset; the platform settings pipeline of module identity carries no console key yet — same pattern as F075/F077).
func GameAdminFrom(c *config.Config) (GameAdmin, error) {
	langs := DefaultTextRequiredLanguages
	if v := strings.TrimSpace(c.StringOr("EW_GAME_TEXT_REQUIRED_LANGUAGES", "")); v != "" {
		langs = nil
		for _, l := range strings.Split(v, ",") {
			if l = strings.TrimSpace(l); l != "" {
				langs = append(langs, l)
			}
		}
	}
	g := GameAdmin{Policy: command.GameAdminPolicy{
		MinLead:             time.Duration(c.Int("EW_GAME_SCHEDULED_PUBLISH_MIN_LEAD_MINUTES", DefaultScheduledPublishMinLeadMinutes)) * time.Minute,
		RequiredLanguages:   langs,
		AssetsPerVersionMax: c.Int("EW_GAME_ASSETS_PER_VERSION_MAX", DefaultAssetsPerVersionMax),
		PreviewTTL:          time.Duration(c.Int("EW_GAME_PREVIEW_TOKEN_TTL_MINUTES", DefaultPreviewTokenTTLMinutes)) * time.Minute,
		ScheduleBatch:       GameAdminJobBatch},
		RenderMaxAttempts: c.Int("EW_GAME_RENDER_MAX_ATTEMPTS", DefaultRenderMaxAttempts),
		RenderBackoff:     time.Duration(c.Int("EW_GAME_RENDER_BACKOFF_SECONDS", DefaultRenderBackoffSeconds)) * time.Second,
		S3Endpoint:        c.StringOr("EW_S3_ENDPOINT", ""), S3Region: c.StringOr("EW_S3_REGION", ""),
		S3AccessKey: c.StringOr("EW_S3_ACCESS_KEY", ""), S3SecretKey: c.StringOr("EW_S3_SECRET_KEY", ""),
		S3Timeout:     c.Duration("EW_S3_TIMEOUT", 0),
		ReportLinkTTL: time.Duration(c.Int("EW_GAME_REPORT_LINK_TTL_MINUTES", DefaultReportLinkTTLMinutes)) * time.Minute, CDNBase: strings.TrimRight(c.StringOr("EW_CDN_BASE_URL", ""), "/")}
	switch scan := c.StringOr("EW_MEDIA_SCAN", "disabled"); {
	case scan == "disabled":
	case strings.HasPrefix(scan, "clamd://"):
		g.ClamdAddr = strings.TrimPrefix(scan, "clamd://")
	default:
		return g, errors.New("game config: EW_MEDIA_SCAN must be clamd://host:port or disabled")
	}
	for _, l := range langs {
		if l != "vi" && l != "en" {
			return g, errors.New("game config: EW_GAME_TEXT_REQUIRED_LANGUAGES must list vi and/or en")
		}
	}
	switch {
	case g.Policy.MinLead < time.Minute || g.Policy.MinLead > 7*24*time.Hour:
		return g, errors.New("game config: EW_GAME_SCHEDULED_PUBLISH_MIN_LEAD_MINUTES must be 1..10080")
	case g.Policy.AssetsPerVersionMax < 1 || g.Policy.AssetsPerVersionMax > 500:
		return g, errors.New("game config: EW_GAME_ASSETS_PER_VERSION_MAX must be 1..500")
	case g.Policy.PreviewTTL < time.Minute || g.Policy.PreviewTTL > 24*time.Hour:
		return g, errors.New("game config: EW_GAME_PREVIEW_TOKEN_TTL_MINUTES must be 1..1440")
	case g.RenderMaxAttempts < 1 || g.RenderMaxAttempts > 20:
		return g, errors.New("game config: EW_GAME_RENDER_MAX_ATTEMPTS must be 1..20")
	case g.RenderBackoff < time.Second || g.RenderBackoff > time.Hour:
		return g, errors.New("game config: EW_GAME_RENDER_BACKOFF_SECONDS must be 1..3600")
	}
	return g, nil
}
