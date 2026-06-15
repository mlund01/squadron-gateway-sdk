// Package gateway is the SDK for building Squadron gateways: subprocess
// integrations that bridge Squadron to external systems (Discord, Slack,
// PagerDuty, custom dashboards, …).
//
// Squadron loads a gateway the same way it loads a plugin — declared in
// HCL with `gateway "name" { source = "github.com/..."; version = "..." }`,
// downloaded from the GitHub release on first load, and run as a managed
// subprocess for the lifetime of the squadron process.
//
// The protocol is bidirectional gRPC:
//
//   - Squadron pushes events to the gateway (OnHumanInputRequested,
//     OnHumanInputResolved, …) so the gateway can mirror Squadron state
//     into its external system.
//   - The gateway pulls / mutates state by calling Squadron's API
//     (ListHumanInputs, ResolveHumanInput, …) so user actions in the
//     external system flow back to Squadron.
//
// Catch-up after disconnects is the gateway's responsibility: persist a
// checkpoint timestamp locally, and on startup call ListHumanInputs
// with `since=<checkpoint>` to backfill anything missed before live
// pushes resume.
package gateway

import (
	"context"
	"time"
)

// Handshake config — must match between squadron's loader and any
// gateway binary that wants squadron to consume it.
//
// Squadron runs hashicorp/go-plugin's handshake against this; mismatched
// magic cookies cause the subprocess to abort cleanly with a helpful
// error rather than running detached.
const (
	HandshakeProtocolVersion  uint = 1
	HandshakeMagicCookieKey        = "SQUADRON_GATEWAY"
	HandshakeMagicCookieValue      = "squadron-gateway-v1"
)

// PluginName is the well-known plugin name registered with go-plugin.
// Both sides reference the same string when looking up the gRPC plugin.
const PluginName = "gateway"

// HumanInputState is the lifecycle state of an ask_human request.
type HumanInputState string

const (
	HumanInputStateOpen     HumanInputState = "open"
	HumanInputStateResolved HumanInputState = "resolved"
)

// HumanInputRecord is the canonical view of a single ask_human request,
// mirrored from squadron's store.
//
// Timestamps are RFC3339Nano strings on the wire so the proto schema
// stays language-neutral, but the SDK exposes them as time.Time on the
// Go interface for convenience. Empty timestamps decode to the zero
// time.Time.
type HumanInputRecord struct {
	ID                string
	MissionID         string
	MissionName       string
	TaskID            string
	TaskName          string
	ToolCallID        string
	Question          string
	ShortSummary      string
	AdditionalContext string
	Choices           []string
	// MultiSelect is true when the human may pick 1+ choices rather
	// than exactly one. When true, the resolved Response is a JSON-
	// encoded string array (e.g. `["A","C"]`); when false, Response is
	// the single chosen string. Always false for free-text questions.
	MultiSelect       bool
	State             HumanInputState
	RequestedAt       time.Time
	ResolvedAt        time.Time // zero when State is open
	Response          string
	ResponderUserID   string
}

// NotificationRecord describes a single mission-lifecycle notification
// pushed to the gateway. Notifications are one-way and informational —
// unlike human-input requests there is nothing for the user to resolve.
type NotificationRecord struct {
	MissionID   string
	MissionName string
	// Event is one of "mission_completed" or "mission_failed".
	Event      string
	Title      string
	Message    string
	OccurredAt time.Time
	// Error is set when Event is "mission_failed", empty otherwise.
	Error string
	// Channel is an optional per-mission destination override. When
	// empty the gateway posts to its globally configured default channel.
	Channel string
}

// PostMessageRequest carries the raw, gateway-schema-shaped JSON the agent
// produced for the builtins.gateway.post tool (text, channel override, rich
// layout) plus any squadron-resolved file attachments. The gateway parses
// Payload and uploads each attachment's bytes directly — it never fetches a URL.
type PostMessageRequest struct {
	Payload     string
	Attachments []FileAttachment
}

// FileAttachment is a squadron-local file (memory/scratchpad/packet) that
// squadron has already read and is shipping as raw bytes for the gateway to
// upload to its external system.
type FileAttachment struct {
	Filename string
	MimeType string
	Content  []byte
}

// MessageToolSpec describes the builtins.gateway.post tool for one gateway.
type MessageToolSpec struct {
	// Description is appended to the tool description so the LLM knows how to
	// format messages for this gateway.
	Description string
	// ParamsSchema is an optional JSON Schema (object) for the tool's
	// parameters. Empty → squadron's default { message } shape.
	ParamsSchema string
}

// HumanInputFilter narrows a ListHumanInputs call. Zero-valued fields
// are not applied (so a fresh HumanInputFilter{} returns everything).
type HumanInputFilter struct {
	// State narrows by lifecycle state. Empty means both open + resolved.
	State HumanInputState
	// MissionID restricts to a specific mission run.
	MissionID string
	// Since is the catch-up cursor: rows whose requested_at OR
	// resolved_at is at or after this time. Use for reconnect / startup
	// backfill — pass the latest event timestamp the gateway has
	// already processed locally.
	Since time.Time
	// OldestFirst flips the default newest-first ordering. Use this
	// when replaying for catch-up so events are processed in the order
	// they happened.
	OldestFirst bool
	// Limit caps the page size. 0 means "use server default".
	Limit int
	// Offset skips that many results from the start of the page.
	Offset int
}

// ResolveResult describes the outcome of a ResolveHumanInput call.
//
// The two boolean flags exist because squadron treats both
// already-resolved and not-found as non-error outcomes — they're
// distinguishable expected results, not exceptions. Wrapping in an
// error type would force gateways to deal with errors.Is / errors.As
// for the common case of "I just want to know if my click landed".
type ResolveResult struct {
	// Record is the canonical row after the call. For not-found this
	// is the zero value.
	Record HumanInputRecord
	// AlreadyResolved is true when the request was resolved by some
	// other actor before this call landed. Display "already answered
	// by X" rather than confirming the gateway's user's submission.
	AlreadyResolved bool
	// NotFound is true when the tool_call_id is unknown to squadron.
	// Most commonly: the row was trimmed from the store, or a stale
	// event made it into the gateway's queue. Treat as a no-op.
	NotFound bool
}

// SquadronAPI is the squadron-side surface that gateways call into.
// The list deliberately starts small; new capabilities (mission
// management, dataset queries, cost reporting, …) are added here over
// time without breaking existing gateways.
type SquadronAPI interface {
	ListHumanInputs(ctx context.Context, filter HumanInputFilter) ([]HumanInputRecord, int /* total */, error)
	ResolveHumanInput(ctx context.Context, toolCallID, response, responderUserID string) (ResolveResult, error)
}

// Gateway is the contract every gateway implementation must satisfy.
// Squadron calls these methods over the gRPC subprocess channel.
//
// The squadron-supplied SquadronAPI is delivered to the gateway via
// Configure. Implementations should hold onto it for the lifetime of
// the gateway and use it to pull state on startup (catch-up) and to
// write resolutions when the external system's user answers a request.
type Gateway interface {
	// Configure runs once at gateway startup. Settings come from the
	// HCL `settings = { ... }` map. The api handle is squadron's
	// SquadronAPI implementation reachable over the broker stream.
	//
	// Implementations should perform their startup catch-up here
	// (ListHumanInputs with a `Since` derived from local checkpoint).
	Configure(ctx context.Context, settings map[string]string, api SquadronAPI) error

	// OnHumanInputRequested is invoked once per new request that
	// squadron observes. Gateways post the question to their external
	// system and arrange for the user's reply to flow back via
	// SquadronAPI.ResolveHumanInput.
	OnHumanInputRequested(ctx context.Context, rec HumanInputRecord) error

	// OnHumanInputResolved is invoked once per resolution event,
	// regardless of who originated the resolution (this gateway, a
	// commander operator, another gateway, the agent timing out).
	// Gateways update their external surface to reflect the answer.
	OnHumanInputResolved(ctx context.Context, rec HumanInputRecord) error

	// OnNotification is invoked when a mission reaches a terminal state
	// (completed, failed, stopped) and the mission opted into gateway
	// notifications. Gateways post an informational message to their
	// external system; there is nothing for the user to act on.
	OnNotification(ctx context.Context, rec NotificationRecord) error

	// PostMessage posts a message to the gateway's external system. The
	// payload is the raw, gateway-schema-shaped JSON the agent produced for
	// the builtins.gateway.post tool; the gateway parses it itself.
	PostMessage(ctx context.Context, req PostMessageRequest) error

	// MessageToolSpec returns the description + optional JSON Schema squadron
	// uses to present the builtins.gateway.post tool to the LLM. Return a zero
	// MessageToolSpec to accept squadron's default { message } shape.
	MessageToolSpec(ctx context.Context) (MessageToolSpec, error)

	// Shutdown is invoked once when squadron is tearing the subprocess
	// down. Release external resources here (close the Discord
	// session, flush queues, persist checkpoint).
	Shutdown(ctx context.Context) error
}
