// Package logging is the one schema every log line follows, so
// Loki can filter to exactly one provider, agent, session, content kind,
// queue, request or RBAC decision.
//
// Every line carries the service (added by the base logger) and a
// low-cardinality scope; each scope has fixed fields that are emitted only
// through the typed helpers on Logger, never hand-typed at the call site.
// Values are JSON fields, never Loki labels: session and agent ids are
// high-cardinality.
package logging

import (
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Redacted replaces a configured secret value in content the helpers log.
const Redacted = "[REDACTED]"

// The scopes of the contract. They are low-cardinality, so they are safe to
// filter on in LogQL.
const (
	ScopeAPI   = "api"
	ScopeRBAC  = "rbac"
	ScopeQueue = "queue"
	ScopeAgent = "agent"
)

// Scoped returns l with its scope field set to scope. A feature module logs
// under its own name this way (app.Installer.Log), next to core's fixed
// scopes. Keep a scope low-cardinality, like the constants above.
func Scoped(l *zap.Logger, scope string) *zap.Logger {
	return l.With(zap.String("scope", scope))
}

// Logger emits contract lines. It is safe for concurrent use.
type Logger struct {
	zap      *zap.Logger
	redactor *redactor
}

// New wraps the base logger (which already adds the service) with the
// contract helpers. Every non-empty secrets value is redacted from the
// content the helpers log, because docs/roles.md says no role can read secrets
// and agent prompts and tool results are where they would leak.
func New(l *zap.Logger, secrets ...string) *Logger {
	return &Logger{zap: l, redactor: newRedactor(secrets)}
}

// Zap exposes the underlying logger for lines outside the contract, such as
// panics and server lifecycle.
func (g *Logger) Zap() *zap.Logger { return g.zap }

// Redact replaces each configured secret value in s with Redacted.
func (g *Logger) Redact(s string) string { return g.redactor.Apply(s) }

// APIRequest is the fixed fields of one api-scope line, one per HTTP
// request.
type APIRequest struct {
	RequestID string
	Method    string
	// Path is the request path as sent; Route is the matched pattern, empty
	// when nothing matched, so Path is what a 404 line has to go on.
	Path       string
	Route      string
	Permission string
	Status     int
	UserSub    string
	// ClientIP is the connection's peer. Proxy headers are never trusted.
	ClientIP string
	Duration time.Duration
}

// Access logs an api-scope request line.
func (g *Logger) Access(req APIRequest) {
	g.zap.Info("request",
		zap.String("scope", ScopeAPI),
		zap.String("request_id", req.RequestID),
		zap.String("method", req.Method),
		zap.String("path", req.Path),
		zap.String("route", req.Route),
		zap.String("permission", req.Permission),
		zap.Int("status", req.Status),
		zap.String("user_sub", req.UserSub),
		zap.String("client_ip", req.ClientIP),
		zap.Int64("duration_ms", req.Duration.Milliseconds()),
	)
}

// Decision is the outcome of an RBAC check.
type Decision string

// The two values an rbac line's decision field takes.
const (
	Allow Decision = "allow"
	Deny  Decision = "deny"
)

// RBACDecision is the fixed fields of one rbac-scope line, one per
// authorisation check.
type RBACDecision struct {
	Subject   string
	Domain    string
	Object    string
	Action    string
	Decision  Decision
	RequestID string
}

// Check logs an rbac-scope decision line.
func (g *Logger) Check(d RBACDecision) {
	g.zap.Info("rbac decision",
		zap.String("scope", ScopeRBAC),
		zap.String("subject", d.Subject),
		zap.String("domain", d.Domain),
		zap.String("object", d.Object),
		zap.String("action", d.Action),
		zap.String("decision", string(d.Decision)),
		zap.String("request_id", d.RequestID),
	)
}

// QueueJob is the fixed fields of one queue-scope line, one per job attempt
// outcome.
type QueueJob struct {
	Queue   string
	JobID   string
	Attempt int
	Outcome string
}

// Job logs a queue-scope line.
func (g *Logger) Job(j QueueJob) {
	g.zap.Info("queue job",
		zap.String("scope", ScopeQueue),
		zap.String("queue", j.Queue),
		zap.String("job_id", j.JobID),
		zap.Int("attempt", j.Attempt),
		zap.String("outcome", j.Outcome),
	)
}

// Kind is the kind of agent content an agent-scope line carries.
type Kind string

// The kinds of agent content.
const (
	KindThought    Kind = "thought"
	KindToolUse    Kind = "tool_use"
	KindToolResult Kind = "tool_result"
	KindMessage    Kind = "message"
	KindPrompt     Kind = "prompt"
	KindQuestion   Kind = "question"
)

// AgentEvent is the fixed fields of one agent-scope line. Its content is
// logged in the content field, redacted first.
type AgentEvent struct {
	Provider  string
	AgentID   string
	SessionID string
	Kind      Kind
	Content   string
}

// Event logs an agent-scope line, redacting secret values from the content.
func (g *Logger) Event(e AgentEvent) {
	g.zap.Info("agent event",
		zap.String("scope", ScopeAgent),
		zap.String("provider", e.Provider),
		zap.String("agent_id", e.AgentID),
		zap.String("session_id", e.SessionID),
		zap.String("kind", string(e.Kind)),
		zap.String("content", g.Redact(e.Content)),
	)
}

type redactor struct {
	// secrets, longest first, so a value contained in another redacts
	// instead of leaving a fragment behind.
	secrets []string
}

func newRedactor(secrets []string) *redactor {
	out := make([]string, 0, len(secrets))
	for _, s := range secrets {
		if s != "" {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return &redactor{secrets: out}
}

// Apply replaces each configured secret value in s with Redacted.
func (r *redactor) Apply(s string) string {
	for _, secret := range r.secrets {
		s = strings.ReplaceAll(s, secret, Redacted)
	}
	return s
}
