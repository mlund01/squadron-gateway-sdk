package gateway

import (
	"time"

	pb "github.com/mlund01/squadron-gateway-sdk/proto"
)

// Wire conversions between the SDK's Go-friendly types and the
// generated protobuf messages. Kept in one file so both the gateway
// (plugin) side and the squadron (host) side share the same encoding.

const wireTimeFormat = time.RFC3339Nano

func recordToProto(r HumanInputRecord) *pb.HumanInputRecord {
	return &pb.HumanInputRecord{
		Id:                r.ID,
		RunUrl:            r.RunURL,
		MissionId:         r.MissionID,
		ThreadId:          r.ThreadID,
		MissionName:       r.MissionName,
		TaskId:            r.TaskID,
		TaskName:          r.TaskName,
		ToolCallId:        r.ToolCallID,
		Question:          r.Question,
		ShortSummary:      r.ShortSummary,
		AdditionalContext: r.AdditionalContext,
		Choices:           append([]string(nil), r.Choices...),
		MultiSelect:       r.MultiSelect,
		State:             string(r.State),
		RequestedAt:       formatTime(r.RequestedAt),
		ResolvedAt:        formatTime(r.ResolvedAt),
		Response:          r.Response,
		ResponderUserId:   r.ResponderUserID,
	}
}

func recordFromProto(p *pb.HumanInputRecord) HumanInputRecord {
	if p == nil {
		return HumanInputRecord{}
	}
	return HumanInputRecord{
		ID:                p.Id,
		RunURL:            p.RunUrl,
		MissionID:         p.MissionId,
		ThreadID:          p.ThreadId,
		MissionName:       p.MissionName,
		TaskID:            p.TaskId,
		TaskName:          p.TaskName,
		ToolCallID:        p.ToolCallId,
		Question:          p.Question,
		ShortSummary:      p.ShortSummary,
		AdditionalContext: p.AdditionalContext,
		Choices:           append([]string(nil), p.Choices...),
		MultiSelect:       p.MultiSelect,
		State:             HumanInputState(p.State),
		RequestedAt:       parseTime(p.RequestedAt),
		ResolvedAt:        parseTime(p.ResolvedAt),
		Response:          p.Response,
		ResponderUserID:   p.ResponderUserId,
	}
}

func notificationToProto(r NotificationRecord) *pb.NotificationRecord {
	return &pb.NotificationRecord{
		RunUrl:      r.RunURL,
		MissionId:   r.MissionID,
		ThreadId:    r.ThreadID,
		MissionName: r.MissionName,
		Event:       r.Event,
		Title:       r.Title,
		Message:     r.Message,
		OccurredAt:  formatTime(r.OccurredAt),
		Error:       r.Error,
		Channel:     r.Channel,
	}
}

func notificationFromProto(p *pb.NotificationRecord) NotificationRecord {
	if p == nil {
		return NotificationRecord{}
	}
	return NotificationRecord{
		RunURL:      p.RunUrl,
		MissionID:   p.MissionId,
		ThreadID:    p.ThreadId,
		MissionName: p.MissionName,
		Event:       p.Event,
		Title:       p.Title,
		Message:     p.Message,
		OccurredAt:  parseTime(p.OccurredAt),
		Error:       p.Error,
		Channel:     p.Channel,
	}
}

func postMessageToProto(r PostMessageRequest) *pb.PostMessageRequest {
	return &pb.PostMessageRequest{Payload: r.Payload, Attachments: attachmentsToProto(r.Attachments), ThreadId: r.ThreadID, RunUrl: r.RunURL,
		MissionId: r.MissionID, TaskId: r.TaskID}
}

func postMessageFromProto(p *pb.PostMessageRequest) PostMessageRequest {
	if p == nil {
		return PostMessageRequest{}
	}
	return PostMessageRequest{Payload: p.Payload, Attachments: attachmentsFromProto(p.Attachments), ThreadID: p.ThreadId, RunURL: p.RunUrl,
		MissionID: p.MissionId, TaskID: p.TaskId}
}

func editMessageToProto(r EditMessageRequest) *pb.EditMessageRequest {
	return &pb.EditMessageRequest{MessageId: r.MessageID, Payload: r.Payload, Attachments: attachmentsToProto(r.Attachments), ThreadId: r.ThreadID, RunUrl: r.RunURL,
		MissionId: r.MissionID, TaskId: r.TaskID}
}

func editMessageFromProto(p *pb.EditMessageRequest) EditMessageRequest {
	if p == nil {
		return EditMessageRequest{}
	}
	return EditMessageRequest{MessageID: p.MessageId, Payload: p.Payload, Attachments: attachmentsFromProto(p.Attachments), ThreadID: p.ThreadId, RunURL: p.RunUrl,
		MissionID: p.MissionId, TaskID: p.TaskId}
}

func attachmentsToProto(in []FileAttachment) []*pb.FileAttachment {
	var out []*pb.FileAttachment
	for _, a := range in {
		out = append(out, &pb.FileAttachment{Filename: a.Filename, MimeType: a.MimeType, Content: a.Content})
	}
	return out
}

func attachmentsFromProto(in []*pb.FileAttachment) []FileAttachment {
	var out []FileAttachment
	for _, a := range in {
		out = append(out, FileAttachment{Filename: a.Filename, MimeType: a.MimeType, Content: a.Content})
	}
	return out
}

func messageToolSpecToProto(s MessageToolSpec) *pb.MessageToolSpecResponse {
	return &pb.MessageToolSpecResponse{Description: s.Description, ParamsSchemaJson: s.ParamsSchema}
}

func messageToolSpecFromProto(p *pb.MessageToolSpecResponse) MessageToolSpec {
	if p == nil {
		return MessageToolSpec{}
	}
	return MessageToolSpec{Description: p.Description, ParamsSchema: p.ParamsSchemaJson}
}

func filterToProto(f HumanInputFilter) *pb.HumanInputFilter {
	return &pb.HumanInputFilter{
		State:       string(f.State),
		MissionId:   f.MissionID,
		Since:       formatTime(f.Since),
		OldestFirst: f.OldestFirst,
		Limit:       int32(f.Limit),
		Offset:      int32(f.Offset),
	}
}

func filterFromProto(p *pb.HumanInputFilter) HumanInputFilter {
	if p == nil {
		return HumanInputFilter{}
	}
	return HumanInputFilter{
		State:       HumanInputState(p.State),
		MissionID:   p.MissionId,
		Since:       parseTime(p.Since),
		OldestFirst: p.OldestFirst,
		Limit:       int(p.Limit),
		Offset:      int(p.Offset),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(wireTimeFormat)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(wireTimeFormat, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
