// Package bridge turns GlitchTip alert webhooks into GitHub Issues for the team owning the failing module
// (F131 T037, F131-AC-18, MONITORING §9): one ticket per GlitchTip issue, labelled module:<name> (tag module)
// or service:<binary> (tag service, for errors outside a module) plus source:glitchtip.
//
// Privacy (AGENTS R14, DATA_PRIVACY): a ticket only carries the issue title and culprit (redacted and
// truncated), the issue link, project, environment, release and counts. Event payloads, request data, user
// data and tag values other than module/service are never fetched into a ticket.
package bridge

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
)

// Results of one GlitchTip issue (label result of eduworld_glitchtip_bridge_tickets_total).
const (
	ResultCreated    = "created"
	ResultDuplicate  = "duplicate"
	ResultIgnoredEnv = "ignored_env"
	ResultError      = "error"
)

// Marker is the body line that identifies the GlitchTip issue of a ticket (dedupe key).
const markerPrefix = "glitchtip-issue:"

// Limits of what a ticket copies (GitHub title limit is 256 characters).
const (
	maxTitle   = 180
	maxField   = 200
	maxWebhook = 1 << 20
	cacheSize  = 4096
)

// Tracker reads issue detail and tags (GlitchTip in production).
type Tracker interface {
	Issue(ctx context.Context, id string) (IssueDetail, error)
	Tags(ctx context.Context, id string) (map[string]string, error)
}

// Tickets searches and creates tickets (GitHub in production).
type Tickets interface {
	FindByMarker(ctx context.Context, marker string) (int, error)
	CreateIssue(ctx context.Context, title, body string, labels []string) (int, error)
}

// Options configure New.
type Options struct {
	Token        string   // webhook token expected in POST /hook/<token>
	Environments []string // GlitchTip environments that create tickets (default: production)
	Tracker      Tracker
	Tickets      Tickets
	Registerer   prometheus.Registerer
	Logger       *slog.Logger
}

// Bridge is the webhook handler.
type Bridge struct {
	tokenSum [32]byte
	envs     map[string]bool
	tracker  Tracker
	tickets  Tickets
	log      *slog.Logger
	total    *prometheus.CounterVec

	// mu serialises check-then-create so two deliveries of the same issue cannot both create a ticket; the
	// webhook rate is low (one alert per issue and rule interval).
	mu sync.Mutex
	// done caches GlitchTip issue IDs that already have a ticket (bounded LRU), because the GitHub search
	// index lags behind a fresh issue by up to a minute.
	done *lru.Cache[string, struct{}]
}

// New validates o and registers the ticket counter on o.Registerer.
func New(o Options) (*Bridge, error) {
	if o.Token == "" || o.Tracker == nil || o.Tickets == nil || o.Registerer == nil {
		return nil, errors.New("bridge: token, tracker, tickets and registerer are required")
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	envs := map[string]bool{}
	for _, e := range o.Environments {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			envs[e] = true
		}
	}
	if len(envs) == 0 {
		envs["production"] = true
	}
	done, err := lru.New[string, struct{}](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("bridge: cache: %w", err)
	}
	b := &Bridge{tokenSum: sha256.Sum256([]byte(o.Token)), envs: envs, tracker: o.Tracker, tickets: o.Tickets,
		log: o.Logger, done: done,
		total: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "eduworld_glitchtip_bridge_tickets_total",
			Help: "GlitchTip issues handled by glitchtip-bridge by result (created, duplicate, ignored_env, error).",
		}, []string{"result"})}
	if err := o.Registerer.Register(b.total); err != nil {
		return nil, fmt.Errorf("bridge: register metrics: %w", err)
	}
	for _, r := range []string{ResultCreated, ResultDuplicate, ResultIgnoredEnv, ResultError} {
		b.total.WithLabelValues(r)
	}
	return b, nil
}

// webhook is the Slack-compatible alert body GlitchTip sends to a "general webhook" recipient.
type webhook struct {
	Attachments []struct {
		Title     string `json:"title"`
		TitleLink string `json:"title_link"`
		Text      string `json:"text"` // culprit
		Fields    []struct {
			Title string `json:"title"`
			Value string `json:"value"`
		} `json:"fields"`
	} `json:"attachments"`
}

var issueLink = regexp.MustCompile(`/issues/([0-9]{1,20})/?(?:[?#].*)?$`)

// ServeHTTP handles POST /hook/<token>. A wrong or missing token answers 404 (the endpoint is not revealed);
// the path is never logged because it carries the token.
func (b *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutPrefix(r.URL.Path, "/hook/")
	if !ok || !b.validToken(token) {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhook+1))
	if err != nil || len(body) > maxWebhook {
		b.total.WithLabelValues(ResultError).Inc()
		http.Error(w, "unreadable body", http.StatusBadRequest)
		return
	}
	var hook webhook
	if err := json.Unmarshal(body, &hook); err != nil || len(hook.Attachments) == 0 {
		b.total.WithLabelValues(ResultError).Inc()
		b.log.WarnContext(r.Context(), "glitchtip webhook without attachments or not JSON")
		http.Error(w, "invalid webhook", http.StatusBadRequest)
		return
	}
	failed := false
	for _, a := range hook.Attachments {
		in := alert{title: a.Title, link: a.TitleLink, culprit: a.Text}
		for _, f := range a.Fields {
			switch strings.ToLower(strings.TrimSpace(f.Title)) {
			case "project":
				in.project = f.Value
			case "environment":
				in.env = f.Value
			case "release":
				in.release = f.Value
			}
		}
		res, err := b.handle(r.Context(), in)
		b.total.WithLabelValues(res).Inc()
		if err != nil {
			failed = true
			b.log.ErrorContext(r.Context(), "glitchtip issue not ticketed", "issue_id", in.id(), "err", err)
		}
	}
	if failed {
		http.Error(w, "ticket creation failed", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (b *Bridge) validToken(token string) bool {
	sum := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(sum[:], b.tokenSum[:]) == 1
}

type alert struct{ title, link, culprit, project, env, release string }

func (a alert) id() string {
	if m := issueLink.FindStringSubmatch(a.link); m != nil {
		return m[1]
	}
	return ""
}

// handle creates the ticket of one GlitchTip issue unless it is outside the ticketed environments or
// already has one.
func (b *Bridge) handle(ctx context.Context, a alert) (string, error) {
	if !b.envs[strings.ToLower(strings.TrimSpace(a.env))] {
		return ResultIgnoredEnv, nil
	}
	id := a.id()
	if id == "" {
		return ResultError, errors.New("issue link without an issue id")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done.Contains(id) {
		return ResultDuplicate, nil
	}
	marker := markerPrefix + id
	n, err := b.tickets.FindByMarker(ctx, marker)
	if err != nil {
		return ResultError, fmt.Errorf("search ticket: %w", err)
	}
	if n > 0 {
		b.done.Add(id, struct{}{})
		return ResultDuplicate, nil
	}
	detail, err := b.tracker.Issue(ctx, id)
	if err != nil {
		return ResultError, fmt.Errorf("glitchtip issue: %w", err)
	}
	tags, err := b.tracker.Tags(ctx, id)
	if err != nil {
		return ResultError, fmt.Errorf("glitchtip tags: %w", err)
	}
	owner := ownerLabel(tags, a.project)
	title := a.title
	if strings.TrimSpace(title) == "" {
		title = detail.Title
	}
	if _, err := b.tickets.CreateIssue(ctx, "[GlitchTip] "+clean(title, maxTitle), ticketBody(a, detail, owner, marker),
		[]string{owner, "source:glitchtip"}); err != nil {
		return ResultError, fmt.Errorf("create ticket: %w", err)
	}
	b.done.Add(id, struct{}{})
	return ResultCreated, nil
}

var labelValue = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,39}$`)

// ownerLabel picks the owning team: tag module (module:<name>), else tag service (service:<binary>), else the
// GlitchTip project (one project per binary), else service:unknown. Values outside [a-z0-9_.-] are refused so a
// tag cannot inject an arbitrary label.
func ownerLabel(tags map[string]string, project string) string {
	if v := strings.ToLower(strings.TrimSpace(tags["module"])); labelValue.MatchString(v) {
		return "module:" + v
	}
	for _, v := range []string{tags["service"], project} {
		if v = strings.ToLower(strings.TrimSpace(v)); labelValue.MatchString(v) {
			return "service:" + v
		}
	}
	return "service:unknown"
}

func ticketBody(a alert, d IssueDetail, owner, marker string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "New GlitchTip issue in `%s`.\n\n", clean(a.env, 40))
	if link := safeLink(a.link); link != "" {
		fmt.Fprintf(&sb, "- Issue: %s\n", link)
	}
	culprit := a.culprit
	if strings.TrimSpace(culprit) == "" {
		culprit = d.Culprit
	}
	if c := clean(culprit, maxField); c != "" {
		fmt.Fprintf(&sb, "- Culprit: `%s`\n", strings.ReplaceAll(c, "`", "'"))
	}
	if p := clean(a.project, 64); p != "" {
		fmt.Fprintf(&sb, "- Project: `%s`\n", p)
	}
	if r := clean(a.release, 100); r != "" {
		fmt.Fprintf(&sb, "- Release: `%s`\n", r)
	}
	if _, err := strconv.ParseUint(d.Count, 10, 64); err == nil {
		fmt.Fprintf(&sb, "- Events: %s\n", d.Count)
	}
	if fs := clean(d.FirstSeen, 40); fs != "" {
		fmt.Fprintf(&sb, "- First seen: %s\n", fs)
	}
	fmt.Fprintf(&sb, "- Owner: `%s`\n\n", owner)
	sb.WriteString("Created by glitchtip-bridge (F131-AC-18). Event payloads and request data are not copied here; " +
		"open the GlitchTip issue (VPN) for the stack trace and `trace_id`.\n\n")
	sb.WriteString(marker + "\n")
	return sb.String()
}

// clean redacts tokens, emails and IPs (same patterns as the backend logs), flattens whitespace and truncates
// to max runes.
func clean(s string, max int) string {
	s = strings.Join(strings.Fields(observability.RedactString(s)), " ")
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

// safeLink keeps an http(s) issue link without credentials or query string.
func safeLink(s string) string {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || len(s) > 500 {
		return ""
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	return u.String()
}
