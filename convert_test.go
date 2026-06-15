package gateway

import (
	"testing"
	"time"
)

// recordRoundTrip tests pin the SDK's wire encoding. Every field on
// HumanInputRecord and HumanInputFilter must survive Go → proto → Go
// — when a field is added to the Go type without a corresponding
// proto entry (or vice versa), one of these tests will fail.
//
// This is the regression net for the multi-select feature: a stray
// edit to either side of the conversion will lose MultiSelect, and
// gateways will silently render single-pick UIs against multi-pick
// records.

func TestRecordRoundTripPreservesEveryField(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Nanosecond)
	resolved := now.Add(time.Minute)
	original := HumanInputRecord{
		ID:                "id-1",
		MissionID:         "m-1",
		MissionName:       "research",
		TaskID:            "t-1",
		TaskName:          "scrape",
		ToolCallID:        "tc-1",
		Question:          "Pick any",
		ShortSummary:      "Pick",
		AdditionalContext: "context",
		Choices:           []string{"A", "B", "C"},
		MultiSelect:       true,
		State:             HumanInputStateOpen,
		RequestedAt:       now,
		ResolvedAt:        resolved,
		Response:          `["A","C"]`,
		ResponderUserID:   "discord:alice",
	}

	got := recordFromProto(recordToProto(original))

	// Compare timestamps separately because formatTime → parseTime is
	// RFC3339Nano which truncates sub-nanosecond precision.
	if !got.RequestedAt.Equal(original.RequestedAt) {
		t.Errorf("RequestedAt: got %v, want %v", got.RequestedAt, original.RequestedAt)
	}
	if !got.ResolvedAt.Equal(original.ResolvedAt) {
		t.Errorf("ResolvedAt: got %v, want %v", got.ResolvedAt, original.ResolvedAt)
	}

	// Zero-out timestamps so the rest can be compared field-by-field.
	got.RequestedAt = time.Time{}
	got.ResolvedAt = time.Time{}
	original.RequestedAt = time.Time{}
	original.ResolvedAt = time.Time{}

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"ID", got.ID, original.ID},
		{"MissionID", got.MissionID, original.MissionID},
		{"MissionName", got.MissionName, original.MissionName},
		{"TaskID", got.TaskID, original.TaskID},
		{"TaskName", got.TaskName, original.TaskName},
		{"ToolCallID", got.ToolCallID, original.ToolCallID},
		{"Question", got.Question, original.Question},
		{"ShortSummary", got.ShortSummary, original.ShortSummary},
		{"AdditionalContext", got.AdditionalContext, original.AdditionalContext},
		{"MultiSelect", got.MultiSelect, original.MultiSelect},
		{"State", got.State, original.State},
		{"Response", got.Response, original.Response},
		{"ResponderUserID", got.ResponderUserID, original.ResponderUserID},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
	if len(got.Choices) != len(original.Choices) {
		t.Fatalf("Choices length mismatch: got %d, want %d", len(got.Choices), len(original.Choices))
	}
	for i := range got.Choices {
		if got.Choices[i] != original.Choices[i] {
			t.Errorf("Choices[%d]: got %q, want %q", i, got.Choices[i], original.Choices[i])
		}
	}
}

func TestFilterRoundTripPreservesSinceCursor(t *testing.T) {
	since := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	original := HumanInputFilter{
		State:       HumanInputStateOpen,
		MissionID:   "m-1",
		Since:       since,
		OldestFirst: true,
		Limit:       50,
		Offset:      10,
	}

	got := filterFromProto(filterToProto(original))

	if got.State != original.State {
		t.Errorf("State: got %v, want %v", got.State, original.State)
	}
	if got.MissionID != original.MissionID {
		t.Errorf("MissionID mismatch")
	}
	if !got.Since.Equal(original.Since) {
		t.Errorf("Since: got %v, want %v (catch-up cursor MUST round-trip exactly)", got.Since, original.Since)
	}
	if got.OldestFirst != original.OldestFirst {
		t.Errorf("OldestFirst mismatch")
	}
	if got.Limit != original.Limit || got.Offset != original.Offset {
		t.Errorf("Limit/Offset mismatch: got %d/%d, want %d/%d",
			got.Limit, got.Offset, original.Limit, original.Offset)
	}
}

func TestRecordRoundTripHandlesZeroValuesGracefully(t *testing.T) {
	// A new request that hasn't been resolved yet has a zero
	// ResolvedAt time — that must survive the round trip as zero, not
	// become "0001-01-01" parsed back as a real time.
	got := recordFromProto(recordToProto(HumanInputRecord{
		ToolCallID: "tc",
		State:      HumanInputStateOpen,
	}))
	if !got.ResolvedAt.IsZero() {
		t.Errorf("zero ResolvedAt must round-trip as zero; got %v", got.ResolvedAt)
	}
	if !got.RequestedAt.IsZero() {
		t.Errorf("zero RequestedAt must round-trip as zero; got %v", got.RequestedAt)
	}
}

func TestFilterRoundTripHandlesZeroSince(t *testing.T) {
	got := filterFromProto(filterToProto(HumanInputFilter{}))
	if !got.Since.IsZero() {
		t.Errorf("empty Since must round-trip as zero; got %v", got.Since)
	}
}

func TestFilterFromProtoNilIsSafe(t *testing.T) {
	// Defensive — gRPC can deliver nil messages on stream reset.
	got := filterFromProto(nil)
	if got != (HumanInputFilter{}) {
		t.Errorf("nil filter should yield zero-value HumanInputFilter, got %+v", got)
	}
}

func TestRecordFromProtoNilIsSafe(t *testing.T) {
	got := recordFromProto(nil)
	// Zero-value HumanInputRecord has nil Choices and zero scalar
	// fields; spot-check the salient bits rather than struct ==
	// (HumanInputRecord can't be compared with == because of the slice).
	if got.ID != "" || got.ToolCallID != "" || got.MultiSelect || len(got.Choices) != 0 {
		t.Errorf("nil record should yield zero-value HumanInputRecord, got %+v", got)
	}
}

func TestNotificationRoundTripPreservesEveryField(t *testing.T) {
	occurred := time.Now().UTC().Truncate(time.Nanosecond)
	original := NotificationRecord{
		MissionID:   "m-1",
		MissionName: "critical",
		Event:       "mission_failed",
		Title:       "Mission failed",
		Message:     "task crashed",
		OccurredAt:  occurred,
		Error:       "boom",
		Channel:     "#ops-alerts",
	}

	got := notificationFromProto(notificationToProto(original))

	if !got.OccurredAt.Equal(original.OccurredAt) {
		t.Errorf("OccurredAt: got %v, want %v", got.OccurredAt, original.OccurredAt)
	}
	got.OccurredAt = time.Time{}
	original.OccurredAt = time.Time{}
	if got != original {
		t.Errorf("notification round-trip lost a field: got %+v, want %+v", got, original)
	}
}

func TestNotificationRoundTripHandlesZeroOccurredAt(t *testing.T) {
	// mission_completed with no explicit timestamp must round-trip the
	// zero time as zero, not "0001-01-01".
	got := notificationFromProto(notificationToProto(NotificationRecord{
		Event: "mission_completed",
	}))
	if !got.OccurredAt.IsZero() {
		t.Errorf("zero OccurredAt must round-trip as zero; got %v", got.OccurredAt)
	}
}

func TestNotificationFromProtoNilIsSafe(t *testing.T) {
	got := notificationFromProto(nil)
	if got != (NotificationRecord{}) {
		t.Errorf("nil notification should yield zero-value NotificationRecord, got %+v", got)
	}
}

func TestPostMessageRoundTripPreservesPayload(t *testing.T) {
	original := PostMessageRequest{Payload: `{"text":"deploy done","channel":"#ops"}`}
	got := postMessageFromProto(postMessageToProto(original))
	if got != original {
		t.Errorf("payload must survive round-trip verbatim: got %+v, want %+v", got, original)
	}
}

func TestPostMessageFromProtoNilIsSafe(t *testing.T) {
	got := postMessageFromProto(nil)
	if got != (PostMessageRequest{}) {
		t.Errorf("nil request should yield zero-value PostMessageRequest, got %+v", got)
	}
}

func TestMessageToolSpecRoundTripPreservesEveryField(t *testing.T) {
	original := MessageToolSpec{
		Description: "Post to Discord. text supports markdown.",
		ParamsSchema: `{"type":"object","properties":{"text":{"type":"string"}},` +
			`"required":["text"]}`,
	}
	got := messageToolSpecFromProto(messageToolSpecToProto(original))
	if got != original {
		t.Errorf("spec round-trip lost a field: got %+v, want %+v", got, original)
	}
}

func TestMessageToolSpecFromProtoNilIsSafe(t *testing.T) {
	got := messageToolSpecFromProto(nil)
	if got != (MessageToolSpec{}) {
		t.Errorf("nil response should yield zero-value MessageToolSpec, got %+v", got)
	}
}
