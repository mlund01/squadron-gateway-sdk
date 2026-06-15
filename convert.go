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
		MissionId:         r.MissionID,
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
		MissionID:         p.MissionId,
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
		MissionId:   r.MissionID,
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
		MissionID:   p.MissionId,
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
	out := &pb.PostMessageRequest{Payload: r.Payload}
	for _, a := range r.Attachments {
		out.Attachments = append(out.Attachments, &pb.FileAttachment{
			Filename: a.Filename,
			MimeType: a.MimeType,
			Content:  a.Content,
		})
	}
	return out
}

func postMessageFromProto(p *pb.PostMessageRequest) PostMessageRequest {
	if p == nil {
		return PostMessageRequest{}
	}
	req := PostMessageRequest{Payload: p.Payload}
	for _, a := range p.Attachments {
		req.Attachments = append(req.Attachments, FileAttachment{
			Filename: a.Filename,
			MimeType: a.MimeType,
			Content:  a.Content,
		})
	}
	return req
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
