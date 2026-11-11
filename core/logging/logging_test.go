package logging_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/andrewmyhre/go-web-base/core/logging"
)

// newTestLogger returns a Logger over buf, built the way production builds
// it: a JSON zap core with the service already on every line.
func newTestLogger(t *testing.T, buf *bytes.Buffer, secrets ...string) *logging.Logger {
	t.Helper()
	base := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(buf),
		zapcore.DebugLevel,
	)).With(zap.String("service", testService))
	return logging.New(base, secrets...)
}

// lastEntry decodes the last line buf holds.
func lastEntry(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, buf.String())
	}
	return entry
}

// assertContract pins every field name on the line: want maps the exact
// set of contract keys (scope and its fixed fields, plus service) to their
// values. A renamed field, a dropped one, or an extra one fails.
func assertContract(t *testing.T, entry map[string]any, want map[string]any) {
	t.Helper()
	for key, val := range want {
		if got, ok := entry[key]; !ok {
			t.Fatalf("line has no %q field: %v", key, entry)
		} else if got != val {
			t.Fatalf("%q = %v, want %v", key, got, val)
		}
	}
	for key := range entry {
		switch key {
		case "ts", "level", "msg": // zap's own keys
		default:
			if _, ok := want[key]; !ok {
				t.Fatalf("line has a field %q outside the contract: %v", key, entry)
			}
		}
	}
	if entry["level"] != "info" {
		t.Fatalf("level = %v, want info", entry["level"])
	}
}

func TestAPIScopePinsEveryFieldName(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(t, &buf).Access(logging.APIRequest{
		RequestID:  "0123456789abcdef",
		Method:     "GET",
		Path:       "/projects/p-1",
		Route:      "/projects/{id}",
		Permission: "Projects: Read",
		Status:     200,
		UserSub:    "user-1",
		ClientIP:   "10.1.2.3",
		Duration:   12 * time.Millisecond,
	})
	assertContract(t, lastEntry(t, &buf), map[string]any{
		"scope":       "api",
		"service":     testService,
		"request_id":  "0123456789abcdef",
		"method":      "GET",
		"path":        "/projects/p-1",
		"route":       "/projects/{id}",
		"permission":  "Projects: Read",
		"status":      float64(200),
		"user_sub":    "user-1",
		"client_ip":   "10.1.2.3",
		"duration_ms": float64(12),
	})
}

func TestRBACScopePinsEveryFieldName(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(t, &buf).Check(logging.RBACDecision{
		Subject:   "user-1",
		Domain:    "projects",
		Object:    "42",
		Action:    "read",
		Decision:  logging.Deny,
		RequestID: "0123456789abcdef",
	})
	assertContract(t, lastEntry(t, &buf), map[string]any{
		"scope":      "rbac",
		"service":    testService,
		"subject":    "user-1",
		"domain":     "projects",
		"object":     "42",
		"action":     "read",
		"decision":   "deny",
		"request_id": "0123456789abcdef",
	})
}

func TestRBACDecisionAllow(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(t, &buf).Check(logging.RBACDecision{
		Subject: "user-1", Domain: "projects", Object: "42",
		Action: "read", Decision: logging.Allow, RequestID: "rid",
	})
	assertContract(t, lastEntry(t, &buf), map[string]any{
		"scope":      "rbac",
		"service":    testService,
		"subject":    "user-1",
		"domain":     "projects",
		"object":     "42",
		"action":     "read",
		"decision":   "allow",
		"request_id": "rid",
	})
}
func TestQueueScopePinsEveryFieldName(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(t, &buf).Job(logging.QueueJob{
		Queue:   "workflow-runs",
		JobID:   "job-1",
		Attempt: 2,
		Outcome: "failed",
	})
	assertContract(t, lastEntry(t, &buf), map[string]any{
		"scope":   "queue",
		"service": testService,
		"queue":   "workflow-runs",
		"job_id":  "job-1",
		"attempt": float64(2),
		"outcome": "failed",
	})
}

func TestAgentScopePinsEveryFieldName(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(t, &buf).Event(logging.AgentEvent{
		Provider:  "anthropic",
		AgentID:   "agent-1",
		SessionID: "session-1",
		Kind:      logging.KindToolResult,
		Content:   "ran the tests",
	})
	assertContract(t, lastEntry(t, &buf), map[string]any{
		"scope":      "agent",
		"service":    testService,
		"provider":   "anthropic",
		"agent_id":   "agent-1",
		"session_id": "session-1",
		"kind":       "tool_result",
		"content":    "ran the tests",
	})
}

func TestEveryKindIsPinned(t *testing.T) {
	kinds := []struct {
		kind logging.Kind
		want string
	}{
		{logging.KindThought, "thought"},
		{logging.KindToolUse, "tool_use"},
		{logging.KindToolResult, "tool_result"},
		{logging.KindMessage, "message"},
		{logging.KindPrompt, "prompt"},
		{logging.KindQuestion, "question"},
	}
	for _, k := range kinds {
		t.Run(k.want, func(t *testing.T) {
			var buf bytes.Buffer
			newTestLogger(t, &buf).Event(logging.AgentEvent{
				Provider: "p", AgentID: "a", SessionID: "s", Kind: k.kind,
			})
			if got := lastEntry(t, &buf)["kind"]; got != k.want {
				t.Fatalf("kind = %v, want %q", got, k.want)
			}
		})
	}
}

func TestAgentContentRedactsKnownSecrets(t *testing.T) {
	const secret = "sk-live-9f8e7d6c5b4a"
	var buf bytes.Buffer
	newTestLogger(t, &buf, secret).Event(logging.AgentEvent{
		Provider:  "openai",
		AgentID:   "agent-1",
		SessionID: "session-1",
		Kind:      logging.KindPrompt,
		Content:   "call the API with key " + secret + " and report back",
	})
	entry := lastEntry(t, &buf)
	if content, _ := entry["content"].(string); !strings.Contains(content, logging.Redacted) {
		t.Fatalf("content = %q, want the redaction marker %q", content, logging.Redacted)
	}
	if strings.Contains(buf.String(), secret) {
		t.Fatalf("log line leaks the known secret:\n%s", buf.String())
	}
}

func TestRedactLongestSecretWins(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(t, &buf, "abc", "abcdef")
	if got := l.Redact("xxabcdefyy"); got != "xx"+logging.Redacted+"yy" {
		t.Fatalf("Redact = %q, want the longer secret redacted whole", got)
	}
}

func TestRedactWithoutSecretsIsPassthrough(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(t, &buf)
	if got := l.Redact("nothing secret here"); got != "nothing secret here" {
		t.Fatalf("Redact = %q, want the input unchanged", got)
	}
	l.Event(logging.AgentEvent{
		Provider: "p", AgentID: "a", SessionID: "s",
		Kind: logging.KindMessage, Content: "hello world",
	})
	entry := lastEntry(t, &buf)
	if content, _ := entry["content"].(string); content != "hello world" {
		t.Fatalf("content = %q, want the content unchanged", content)
	}
}

// testService is an arbitrary service name: the app supplies the real one.
const testService = "test-service"
