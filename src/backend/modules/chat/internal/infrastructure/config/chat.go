package config

import (
	"errors"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/policy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Defaults of CONFIG_DEFAULTS for the chat core (F115 T001; owners CHAT_SYSTEM §3.1, §5, VALKEY_DESIGN, CHILD_SAFETY).
const (
	DefaultPublishAckTimeout = 2 * time.Second       // chat_publish_ack_timeout
	DefaultPersisterBatch    = 500                   // chat_persister_batch: events per batch
	DefaultPersisterWait     = 50 * time.Millisecond // chat_persister_batch: max wait of a batch
	DefaultMaxDeliver        = 5                     // chat_persister_batch: max_deliver → DLQ
	DefaultAckWait           = 30 * time.Second      // EVENT_DESIGN §4.10: consumer ack_wait
	DefaultRecentBuffer      = 200                   // chat_recent_buffer: messages
	DefaultRecentTTL         = 10 * time.Minute      // chat_recent_buffer: TTL
	DefaultRatePerUser       = 30                    // chat_rate_per_user: messages per minute on every channel
	DefaultPolicyCache       = 5 * time.Minute       // chat_policy_cache
	DefaultFactsFresh        = 30 * time.Second      // channel facts (members, cards, blocks) [Proposed F115]
	DefaultCurfewFresh       = time.Minute           // curfew facts [Proposed F115]
	DefaultStaleTTL          = time.Hour             // last known facts kept for a PostgreSQL outage [Proposed F115]
	DefaultCmidTTL           = 10 * time.Minute      // ew:chat:cmid TTL (CHAT_SYSTEM §5.2)
	DefaultStreamMaxAge      = 72 * time.Hour        // CHAT_MESSAGES max_age (ADR-0035)
	DefaultPlatformCurfew    = "21:30-06:30"         // platform_curfew (ICT) until projected from identity
)

// Chat is the configuration of the chat core.
type Chat struct {
	Env               string
	PublishAckTimeout time.Duration
	PersisterBatch    int
	PersisterWait     time.Duration
	MaxDeliver        int
	AckWait           time.Duration
	RecentBuffer      int
	RecentTTL         time.Duration
	RatePerUser       int
	PolicyCache       time.Duration
	FactsFresh        time.Duration
	CurfewFresh       time.Duration
	StaleTTL          time.Duration
	CmidTTL           time.Duration
	StreamMaxAge      time.Duration
	StreamReplicas    int
	PlatformCurfew    policy.Window
	CurfewLocation    *time.Location
}

// ChatFrom reads the chat configuration: EW_ENV (test allows the labelled ContentValidator test double),
// EW_CHAT_PUBLISH_ACK_TIMEOUT, EW_CHAT_PERSISTER_BATCH, EW_CHAT_PERSISTER_BATCH_WAIT, EW_CHAT_PERSISTER_MAX_DELIVER,
// EW_CHAT_PERSISTER_ACK_WAIT, EW_CHAT_RECENT_BUFFER, EW_CHAT_RECENT_TTL, EW_CHAT_RATE_PER_USER, EW_CHAT_POLICY_CACHE,
// EW_CHAT_FACTS_FRESH, EW_CHAT_CURFEW_FRESH, EW_CHAT_STALE_TTL, EW_CHAT_CMID_TTL, EW_CHAT_STREAM_MAX_AGE,
// EW_NATS_STREAM_REPLICAS, EW_PLATFORM_CURFEW (initial platform_curfew until identity projects it) and
// EW_PLATFORM_TIMEZONE.
func ChatFrom(c *config.Config) (Chat, error) {
	loc, err := time.LoadLocation(c.StringOr("EW_PLATFORM_TIMEZONE", DefaultTimeZone))
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600) // no tzdata in the image: Asia/Ho_Chi_Minh has no DST
	}
	curfew, err := policy.ParseWindow(c.StringOr("EW_PLATFORM_CURFEW", DefaultPlatformCurfew), loc)
	if err != nil {
		return Chat{}, errors.New("chat config: EW_PLATFORM_CURFEW must be HH:MM-HH:MM")
	}
	out := Chat{
		Env:               c.StringOr("EW_ENV", ""),
		PublishAckTimeout: c.Duration("EW_CHAT_PUBLISH_ACK_TIMEOUT", DefaultPublishAckTimeout),
		PersisterBatch:    c.Int("EW_CHAT_PERSISTER_BATCH", DefaultPersisterBatch),
		PersisterWait:     c.Duration("EW_CHAT_PERSISTER_BATCH_WAIT", DefaultPersisterWait),
		MaxDeliver:        c.Int("EW_CHAT_PERSISTER_MAX_DELIVER", DefaultMaxDeliver),
		AckWait:           c.Duration("EW_CHAT_PERSISTER_ACK_WAIT", DefaultAckWait),
		RecentBuffer:      c.Int("EW_CHAT_RECENT_BUFFER", DefaultRecentBuffer),
		RecentTTL:         c.Duration("EW_CHAT_RECENT_TTL", DefaultRecentTTL),
		RatePerUser:       c.Int("EW_CHAT_RATE_PER_USER", DefaultRatePerUser),
		PolicyCache:       c.Duration("EW_CHAT_POLICY_CACHE", DefaultPolicyCache),
		FactsFresh:        c.Duration("EW_CHAT_FACTS_FRESH", DefaultFactsFresh),
		CurfewFresh:       c.Duration("EW_CHAT_CURFEW_FRESH", DefaultCurfewFresh),
		StaleTTL:          c.Duration("EW_CHAT_STALE_TTL", DefaultStaleTTL),
		CmidTTL:           c.Duration("EW_CHAT_CMID_TTL", DefaultCmidTTL),
		StreamMaxAge:      c.Duration("EW_CHAT_STREAM_MAX_AGE", DefaultStreamMaxAge),
		StreamReplicas:    c.Int("EW_NATS_STREAM_REPLICAS", 1),
		PlatformCurfew:    curfew,
		CurfewLocation:    loc,
	}
	switch {
	case out.PublishAckTimeout < 100*time.Millisecond || out.PublishAckTimeout > 10*time.Second:
		return out, errors.New("chat config: EW_CHAT_PUBLISH_ACK_TIMEOUT must be 100ms..10s")
	case out.PersisterBatch < 1 || out.PersisterBatch > 5000 || out.PersisterWait <= 0:
		return out, errors.New("chat config: EW_CHAT_PERSISTER_BATCH must be 1..5000 with a positive wait")
	case out.MaxDeliver < 1 || out.AckWait < time.Second:
		return out, errors.New("chat config: EW_CHAT_PERSISTER_MAX_DELIVER ≥ 1 and EW_CHAT_PERSISTER_ACK_WAIT ≥ 1s")
	case out.RecentBuffer < 1 || out.RecentBuffer > 1000 || out.RecentTTL < time.Minute:
		return out, errors.New("chat config: EW_CHAT_RECENT_BUFFER must be 1..1000 and EW_CHAT_RECENT_TTL ≥ 1m")
	case out.RatePerUser < 1:
		return out, errors.New("chat config: EW_CHAT_RATE_PER_USER must be ≥ 1")
	case out.PolicyCache < time.Second || out.FactsFresh < time.Second || out.CurfewFresh < time.Second:
		return out, errors.New("chat config: cache freshness must be ≥ 1s")
	case out.StaleTTL < out.PolicyCache || out.StaleTTL < out.FactsFresh || out.CmidTTL < time.Minute:
		return out, errors.New("chat config: EW_CHAT_STALE_TTL must cover the caches and EW_CHAT_CMID_TTL be ≥ 1m")
	case out.StreamMaxAge < time.Hour || out.StreamReplicas < 1:
		return out, errors.New("chat config: EW_CHAT_STREAM_MAX_AGE ≥ 1h and EW_NATS_STREAM_REPLICAS ≥ 1")
	}
	return out, nil
}
