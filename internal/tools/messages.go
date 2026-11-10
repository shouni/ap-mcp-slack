package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shouni/ap-mcp-slack/internal/client"
)

// MessageContent holds the fields shared by post_slack_message and
// post_slack_message_as_user (everything except thread_ts, whose semantics differ
// slightly between the webhook and Web API paths, and channel_id, which only the
// Web API path accepts).
type MessageContent struct {
	Text        string           `json:"text" jsonschema:"The message body to post to Slack. Interpreted as Slack mrkdwn by default."`
	Blocks      []map[string]any `json:"blocks,omitempty" jsonschema:"An optional Slack Block Kit blocks array. Include text for accessibility even when specifying it."`
	Attachments []map[string]any `json:"attachments,omitempty" jsonschema:"An optional Slack attachments array."`
	IconEmoji   string           `json:"icon_emoji,omitempty" jsonschema:"The Slack emoji name to use as the poster's icon. Example: :robot_face:"`
	UnfurlLinks *bool            `json:"unfurl_links,omitempty" jsonschema:"Controls link unfurling."`
	UnfurlMedia *bool            `json:"unfurl_media,omitempty" jsonschema:"Controls media unfurling."`
	Mentions    []string         `json:"mentions,omitempty" jsonschema:"An array of Slack user IDs to mention (e.g. [\"U0123456\"]). Prepended to the body as <@ID>. When blocks is given, the body is used only as the fallback display, so mention them explicitly inside blocks."`
}

// validate rejects input combinations that would quietly do something other than what
// the caller asked for.
//
// Mentions are prepended to Text, but Slack renders blocks in place of text whenever
// blocks are present and keeps text only as the notification fallback. A
// mentions+blocks call would therefore ping the named people while showing no mention
// anywhere in the visible message. The schema documents this, but schema prose is
// exactly what a model skips, so the combination is refused here instead. Where in a
// caller's block layout the mention belongs is theirs to decide, not something to
// guess at.
func (m MessageContent) validate() error {
	if len(m.Blocks) > 0 && mentionPrefix(m.Mentions) != "" {
		return fmt.Errorf("slack: mentions cannot be combined with blocks: put the <@ID> tag inside blocks instead, since Slack renders blocks in place of text")
	}
	return nil
}

// toMessage builds the Incoming Webhook payload, which carries threadTS as a
// separate top-level field.
func (m MessageContent) toMessage(threadTS string) client.Message {
	return client.Message{
		Text:        m.textWithMentions(),
		Blocks:      m.Blocks,
		Attachments: m.Attachments,
		ThreadTS:    threadTS,
		IconEmoji:   m.IconEmoji,
		UnfurlLinks: m.UnfurlLinks,
		UnfurlMedia: m.UnfurlMedia,
	}
}

// toWebAPIMessage builds the chat.postMessage payload, which additionally carries
// the destination channelID.
func (m MessageContent) toWebAPIMessage(channelID, threadTS string) client.WebAPIMessage {
	return client.WebAPIMessage{
		ChannelID:   channelID,
		Text:        m.textWithMentions(),
		Blocks:      m.Blocks,
		Attachments: m.Attachments,
		ThreadTS:    threadTS,
		IconEmoji:   m.IconEmoji,
		UnfurlLinks: m.UnfurlLinks,
		UnfurlMedia: m.UnfurlMedia,
	}
}

// textWithMentions prepends Mentions to Text as <@ID> tags so they actually notify
// the named users, since Slack only highlights/notifies on mention tags present in
// the delivered message content.
func (m MessageContent) textWithMentions() string {
	prefix := mentionPrefix(m.Mentions)
	if prefix == "" {
		return m.Text
	}
	return prefix + m.Text
}

// mentionPrefix formats mentions (Slack user IDs) as a single "<@U1> <@U2>\n" line,
// skipping blank entries. It returns "" if mentions has no non-blank entries.
func mentionPrefix(mentions []string) string {
	var b strings.Builder
	for _, id := range mentions {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString("<@")
		b.WriteString(id)
		b.WriteByte('>')
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String() + "\n"
}

// PostSlackMessageInput is the input for post_slack_message.
type PostSlackMessageInput struct {
	MessageContent
	ThreadTS string `json:"thread_ts,omitempty" jsonschema:"The parent message ts when replying in a thread. Effective only when the webhook supports it."`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"true actually posts. When false (the default), nothing is posted and only a preview of the message is returned. Check the preview, then rerun with confirm=true."`
}

// PostSlackMessageOutput is the structured output for post_slack_message.
type PostSlackMessageOutput struct {
	OK bool `json:"ok"`
	// Posted is false when confirm was omitted/false: nothing was sent to Slack,
	// and the other fields describe what would be sent if confirm=true were set.
	Posted     bool           `json:"posted"`
	Mentions   []string       `json:"mentions,omitempty"`
	Payload    client.Message `json:"payload"`
	StatusCode int            `json:"status_code,omitempty"`
	Body       string         `json:"body,omitempty"`
}

// PostSlackMessageAsUserInput is the input for post_slack_message_as_user.
type PostSlackMessageAsUserInput struct {
	ChannelID string `json:"channel_id,omitempty" jsonschema:"The channel ID to post to. Defaults to MCP_SLACK_CHANNEL_ID."`
	MessageContent
	ThreadTS string `json:"thread_ts,omitempty" jsonschema:"The parent message ts when replying in a thread."`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"true actually posts. When false (the default), nothing is posted and only a preview with the channel name, mentioned users and thread parent message resolved is returned. Check the preview, then rerun with confirm=true."`
}

// PostSlackMessageAsUserOutput is the structured output for post_slack_message_as_user.
type PostSlackMessageAsUserOutput struct {
	OK bool `json:"ok"`
	// Posted is false when confirm was omitted/false: nothing was sent to Slack,
	// and the other fields describe what would be sent if confirm=true were set.
	Posted    bool   `json:"posted"`
	ChannelID string `json:"channel_id"`
	// ChannelName is resolved through conversations.info so the caller can confirm
	// the human-readable destination (e.g. "#prj-foo") rather than just a channel ID.
	ChannelName string `json:"channel_name,omitempty"`
	// Mentions is resolved through users.info for each entry in the input's
	// mentions field, so the caller can confirm who will actually be notified.
	Mentions []client.ResolvedMention `json:"mentions,omitempty"`
	// ThreadParent is the message this post would reply to, when thread_ts is set, so
	// the caller can confirm the reply lands on the intended thread.
	ThreadParent *client.SlackMessageSummary `json:"thread_parent,omitempty"`
	Payload      client.WebAPIMessage        `json:"payload"`
	TS           string                      `json:"ts,omitempty"`
}

// UpdateSlackMessageInput is the input for update_slack_message.
type UpdateSlackMessageInput struct {
	ChannelID   string           `json:"channel_id,omitempty" jsonschema:"The channel ID of the message to update. Defaults to MCP_SLACK_CHANNEL_ID."`
	TS          string           `json:"ts" jsonschema:"The ts of the message to update. The return value of post_slack_message_as_user can be used."`
	Text        string           `json:"text,omitempty" jsonschema:"The new message body. Required unless blocks or attachments is given."`
	Blocks      []map[string]any `json:"blocks,omitempty" jsonschema:"The new Slack Block Kit blocks array. Replaces the existing blocks when given."`
	Attachments []map[string]any `json:"attachments,omitempty" jsonschema:"The new Slack attachments array. Replaces the existing attachments when given."`
	Confirm     bool             `json:"confirm,omitempty" jsonschema:"true actually updates. When false (the default), nothing is updated, and the current content of the target message and the updated content are returned. Check the preview, then rerun with confirm=true."`
}

// UpdateSlackMessageOutput is the structured output for update_slack_message.
type UpdateSlackMessageOutput struct {
	OK bool `json:"ok"`
	// Updated is false when confirm was omitted/false: the message was left alone,
	// and Current/Payload describe the change that confirm=true would apply.
	Updated     bool   `json:"updated"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name,omitempty"`
	TS          string `json:"ts"`
	// Current is the message as it exists right now, so the caller can see what the
	// update would overwrite. See targetNote for why it may be absent.
	Current     *client.SlackMessageSummary `json:"current,omitempty"`
	CurrentNote string                      `json:"current_note,omitempty"`
	// Payload is the resolved replacement content, including the source-label footer
	// and any blocks/attachments. Text alone cannot stand in for it: a blocks-only
	// update leaves Text empty, which would show a human an apparently empty
	// replacement for the message they are about to overwrite.
	Payload client.UpdateWebAPIMessage `json:"payload"`
	// Text is the requested new body before confirming, and Slack's echo of the
	// stored body afterwards.
	Text string `json:"text,omitempty"`
}

// DeleteSlackMessageInput is the input for delete_slack_message.
type DeleteSlackMessageInput struct {
	ChannelID string `json:"channel_id,omitempty" jsonschema:"The channel ID of the message to delete. Defaults to MCP_SLACK_CHANNEL_ID."`
	TS        string `json:"ts" jsonschema:"The ts of the message to delete. The return value of post_slack_message_as_user can be used."`
	Confirm   bool   `json:"confirm,omitempty" jsonschema:"true actually deletes. When false (the default), nothing is deleted and the content of the target message is returned. Deletion cannot be undone, so check the preview, then rerun with confirm=true."`
}

// DeleteSlackMessageOutput is the structured output for delete_slack_message.
type DeleteSlackMessageOutput struct {
	OK bool `json:"ok"`
	// Deleted is false when confirm was omitted/false: nothing was removed, and
	// Target describes what confirm=true would remove.
	Deleted     bool   `json:"deleted"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name,omitempty"`
	TS          string `json:"ts"`
	// Target is the message that would be (or was) deleted. See targetNote for why
	// it may be absent.
	Target     *client.SlackMessageSummary `json:"target,omitempty"`
	TargetNote string                      `json:"target_note,omitempty"`
}

// targetNote explains an absent Current/Target in an update/delete preview, so a
// caller is told that the old content is unknown rather than being left to read the
// missing field as "the message is empty".
const (
	targetNoteNotFound    = "The target message was not found. Check the ts and the channel."
	targetNoteUnreadable  = "The target message could not be read (missing history scope, etc.). Running this means acting without having seen its content."
	targetNoteUnnamedChan = "The channel name could not be resolved (missing channels:read / groups:read scope, etc.). Check that channel_id is the intended destination."
	// targetNoteSearchTruncated is deliberately not targetNoteNotFound: the ts may be
	// perfectly valid and the operation may well succeed, so telling the caller to
	// re-check it would send them after the wrong problem.
	targetNoteSearchTruncated = "The search for the target message was cut short (it may be a reply in a very long thread). Its content may not be shown even when the ts is correct, so running this means acting without having seen its content."
)

// resolvedTarget describes the message an update or delete would act on.
type resolvedTarget struct {
	ChannelID   string
	ChannelName string
	Message     *client.SlackMessageSummary
	Note        string
}

// resolveTarget looks up the destination channel's name and the message at ts, so
// update/delete can show which message in which channel is about to change.
//
// Neither lookup failing is an error. chat.update and chat.delete require neither a
// history scope nor a channels:read/groups:read scope, so a token may be fully
// authorized to destroy a message whose body — or whose channel's name — it cannot
// read; refusing to proceed would break that legitimate setup, leaving the caller with
// a message they can delete through no tool this server offers. What could not be
// resolved is reported through Note instead, leaving the human to decide whether to
// confirm on partial information.
//
// Whether a destination exists at all is therefore settled first, through
// ResolveChannelID, and is the only thing here that can fail. Inferring it from a failed
// conversations.info instead would read every unreadable channel as a missing one: the
// lookup resolves the default channel internally, so its failure says nothing about
// whether a default was configured.
func (t *SlackTools) resolveTarget(ctx context.Context, channelID, ts string) (resolvedTarget, error) {
	resolvedChannelID, err := t.client.ResolveChannelID(channelID)
	if err != nil {
		return resolvedTarget{}, err
	}

	target := resolvedTarget{ChannelID: resolvedChannelID}
	var notes []string

	if channelInfo, err := t.client.GetChannelInfo(ctx, client.GetChannelInfoOptions{ChannelID: resolvedChannelID}); err == nil {
		target.ChannelID = channelInfo.Channel.ID
		target.ChannelName = channelInfo.Channel.Name
	} else {
		notes = append(notes, targetNoteUnnamedChan)
	}

	message, searchTruncated, err := t.client.GetMessage(ctx, target.ChannelID, ts)
	switch {
	case err != nil:
		notes = append(notes, targetNoteUnreadable)
	case message != nil:
		target.Message = message
	case searchTruncated:
		notes = append(notes, targetNoteSearchTruncated)
	default:
		notes = append(notes, targetNoteNotFound)
	}

	target.Note = strings.Join(notes, " ")
	return target, nil
}

// postSlackMessage previews the webhook payload unconditionally, then only actually
// sends it (via client.PostMessage) when in.Confirm is set. Why previewing is a mode
// of this tool rather than a tool of its own is explained on Register.
func (t *SlackTools) postSlackMessage(ctx context.Context, _ *mcp.CallToolRequest, in PostSlackMessageInput) (*mcp.CallToolResult, PostSlackMessageOutput, error) {
	if err := in.validate(); err != nil {
		return nil, PostSlackMessageOutput{}, err
	}

	payload, err := t.client.PreviewMessage(in.toMessage(in.ThreadTS))
	if err != nil {
		return nil, PostSlackMessageOutput{}, err
	}

	out := PostSlackMessageOutput{
		OK:       true,
		Mentions: in.Mentions,
		Payload:  payload,
	}
	if !in.Confirm {
		return nil, out, nil
	}

	postOut, err := t.client.PostMessage(ctx, in.toMessage(in.ThreadTS))
	if err != nil {
		return nil, PostSlackMessageOutput{}, err
	}
	out.Posted = true
	out.StatusCode = postOut.StatusCode
	out.Body = postOut.Body
	return nil, out, nil
}

// buildWebAPIPreview builds the chat.postMessage payload for in without sending it,
// resolving the destination channel name, mentions' display names, and (if in.ThreadTS
// is set) the thread's parent message, so a caller can confirm exactly who and where a
// post_slack_message_as_user call would notify before it happens.
func (t *SlackTools) buildWebAPIPreview(ctx context.Context, in PostSlackMessageAsUserInput) (PostSlackMessageAsUserOutput, error) {
	if err := in.validate(); err != nil {
		return PostSlackMessageAsUserOutput{}, err
	}

	payload, err := t.client.PreviewWebAPIMessage(in.toWebAPIMessage(in.ChannelID, in.ThreadTS))
	if err != nil {
		return PostSlackMessageAsUserOutput{}, err
	}

	out := PostSlackMessageAsUserOutput{
		OK:        true,
		ChannelID: payload.ChannelID,
		Payload:   payload,
	}

	channelInfo, err := t.client.GetChannelInfo(ctx, client.GetChannelInfoOptions{ChannelID: payload.ChannelID})
	if err != nil {
		return PostSlackMessageAsUserOutput{}, err
	}
	out.ChannelName = channelInfo.Channel.Name

	if len(in.Mentions) > 0 {
		mentions, err := t.client.ResolveMentions(ctx, in.Mentions)
		if err != nil {
			return PostSlackMessageAsUserOutput{}, err
		}
		out.Mentions = mentions
	}

	if threadTS := strings.TrimSpace(in.ThreadTS); threadTS != "" {
		replies, err := t.client.GetConversationReplies(ctx, client.ConversationRepliesOptions{
			ChannelID: payload.ChannelID,
			TS:        threadTS,
			Limit:     1,
		})
		if err != nil {
			return PostSlackMessageAsUserOutput{}, err
		}
		if len(replies.Messages) > 0 {
			out.ThreadParent = &replies.Messages[0]
		}
	}

	return out, nil
}

// postSlackMessageAsUser previews the chat.postMessage payload unconditionally
// (resolving channel name, mentions, and thread parent), then only actually sends it
// (via client.PostWebAPIMessage) when in.Confirm is set, so callers always see what
// would be posted — and who it would notify — before it happens.
func (t *SlackTools) postSlackMessageAsUser(ctx context.Context, _ *mcp.CallToolRequest, in PostSlackMessageAsUserInput) (*mcp.CallToolResult, PostSlackMessageAsUserOutput, error) {
	out, err := t.buildWebAPIPreview(ctx, in)
	if err != nil {
		return nil, PostSlackMessageAsUserOutput{}, err
	}
	if !in.Confirm {
		return nil, out, nil
	}

	postOut, err := t.client.PostWebAPIMessage(ctx, in.toWebAPIMessage(in.ChannelID, in.ThreadTS))
	if err != nil {
		return nil, PostSlackMessageAsUserOutput{}, err
	}
	out.Posted = true
	out.TS = postOut.TS
	return nil, out, nil
}

// updateSlackMessage resolves the target message's current content unconditionally,
// then only rewrites it when in.Confirm is set, mirroring the confirm gate on the post
// tools. An update silently discards whatever the message said before, so the caller
// is shown the old body alongside the new one before that becomes irreversible.
func (t *SlackTools) updateSlackMessage(ctx context.Context, _ *mcp.CallToolRequest, in UpdateSlackMessageInput) (*mcp.CallToolResult, UpdateSlackMessageOutput, error) {
	ts := strings.TrimSpace(in.TS)
	if ts == "" {
		return nil, UpdateSlackMessageOutput{}, fmt.Errorf("slack: ts is required")
	}
	// Checked here as well as in PreviewUpdateWebAPIMessage so a malformed call is
	// rejected before resolveTarget spends two Slack round trips on it.
	if strings.TrimSpace(in.Text) == "" && len(in.Blocks) == 0 && len(in.Attachments) == 0 {
		return nil, UpdateSlackMessageOutput{}, fmt.Errorf("slack: text, blocks, or attachments is required")
	}

	target, err := t.resolveTarget(ctx, in.ChannelID, ts)
	if err != nil {
		return nil, UpdateSlackMessageOutput{}, err
	}

	replacement := client.UpdateWebAPIMessage{
		ChannelID:   target.ChannelID,
		TS:          ts,
		Text:        in.Text,
		Blocks:      in.Blocks,
		Attachments: in.Attachments,
	}
	// Resolving the replacement before the confirm gate validates the new content and
	// applies the source-label footer, so the preview shows the payload that a
	// confirmed call would send rather than the caller's unprocessed input.
	payload, err := t.client.PreviewUpdateWebAPIMessage(replacement)
	if err != nil {
		return nil, UpdateSlackMessageOutput{}, err
	}

	out := UpdateSlackMessageOutput{
		OK:          true,
		ChannelID:   target.ChannelID,
		ChannelName: target.ChannelName,
		TS:          ts,
		Current:     target.Message,
		CurrentNote: target.Note,
		Payload:     payload,
		Text:        in.Text,
	}
	if !in.Confirm {
		return nil, out, nil
	}

	updated, err := t.client.UpdateWebAPIMessage(ctx, replacement)
	if err != nil {
		return nil, UpdateSlackMessageOutput{}, err
	}
	out.Updated = true
	out.TS = updated.TS
	out.Text = updated.Text
	return nil, out, nil
}

// deleteSlackMessage resolves the target message unconditionally, then only removes it
// when in.Confirm is set. Deletion is the least reversible operation this server
// exposes — Slack keeps no undo — so it gets the same gate as posting, with the body
// of the doomed message included so a human can check it really is the intended one.
func (t *SlackTools) deleteSlackMessage(ctx context.Context, _ *mcp.CallToolRequest, in DeleteSlackMessageInput) (*mcp.CallToolResult, DeleteSlackMessageOutput, error) {
	ts := strings.TrimSpace(in.TS)
	if ts == "" {
		return nil, DeleteSlackMessageOutput{}, fmt.Errorf("slack: ts is required")
	}

	target, err := t.resolveTarget(ctx, in.ChannelID, ts)
	if err != nil {
		return nil, DeleteSlackMessageOutput{}, err
	}

	out := DeleteSlackMessageOutput{
		OK:          true,
		ChannelID:   target.ChannelID,
		ChannelName: target.ChannelName,
		TS:          ts,
		Target:      target.Message,
		TargetNote:  target.Note,
	}
	if !in.Confirm {
		return nil, out, nil
	}

	deleted, err := t.client.DeleteWebAPIMessage(ctx, target.ChannelID, ts)
	if err != nil {
		return nil, DeleteSlackMessageOutput{}, err
	}
	out.Deleted = true
	out.TS = deleted.TS
	return nil, out, nil
}
