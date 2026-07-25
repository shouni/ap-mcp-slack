package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"ap-mcp-slack/internal/client"
)

// MessageContent holds the fields shared by post_slack_message and
// post_slack_message_as_user (everything except thread_ts, whose semantics differ
// slightly between the webhook and Web API paths, and channel_id, which only the
// Web API path accepts).
type MessageContent struct {
	Text        string           `json:"text" jsonschema:"Slackに投稿する本文。デフォルトでSlackのmrkdwnとして解釈されます。"`
	Blocks      []map[string]any `json:"blocks,omitempty" jsonschema:"任意のSlack Block Kit blocks配列。指定する場合もアクセシビリティ用にtextを含めてください。"`
	Attachments []map[string]any `json:"attachments,omitempty" jsonschema:"任意のSlack attachments配列。"`
	IconEmoji   string           `json:"icon_emoji,omitempty" jsonschema:"投稿者アイコンとして使うSlack絵文字名。例: :robot_face:"`
	UnfurlLinks *bool            `json:"unfurl_links,omitempty" jsonschema:"リンク展開を制御します。"`
	UnfurlMedia *bool            `json:"unfurl_media,omitempty" jsonschema:"メディア展開を制御します。"`
	Mentions    []string         `json:"mentions,omitempty" jsonschema:"メンション対象のSlackユーザーID配列（例: [\"U0123456\"]）。本文の先頭に <@ID> 形式で追加されます。blocksを指定した場合、本文はフォールバック表示にしか使われないため、blocks内で明示的にメンションしてください。"`
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
	ThreadTS string `json:"thread_ts,omitempty" jsonschema:"スレッド返信にする場合の親メッセージts。Webhook側で利用可能な場合のみ有効です。"`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"true にすると実際に投稿します。false（省略時）の場合は投稿せず、投稿内容のプレビューのみを返します。プレビューを確認したうえで confirm=true を指定して再実行してください。"`
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

// PreviewSlackMessageOutput is the structured output for preview_slack_message.
type PreviewSlackMessageOutput struct {
	OK        bool           `json:"ok"`
	Transport string         `json:"transport"`
	Mentions  []string       `json:"mentions,omitempty"`
	Payload   client.Message `json:"payload"`
}

// PostSlackMessageAsUserInput is the input for post_slack_message_as_user.
type PostSlackMessageAsUserInput struct {
	ChannelID string `json:"channel_id,omitempty" jsonschema:"投稿先チャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	MessageContent
	ThreadTS string `json:"thread_ts,omitempty" jsonschema:"スレッド返信にする場合の親メッセージts。"`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"true にすると実際に投稿します。false（省略時）の場合は投稿せず、チャンネル名・メンション先・スレッド元メッセージを解決したプレビューのみを返します。プレビューを確認したうえで confirm=true を指定して再実行してください。"`
}

// PostSlackMessageAsUserOutput is the structured output for post_slack_message_as_user.
type PostSlackMessageAsUserOutput struct {
	OK bool `json:"ok"`
	// Posted is false when confirm was omitted/false: nothing was sent to Slack,
	// and the other fields describe what would be sent if confirm=true were set.
	Posted       bool                        `json:"posted"`
	ChannelID    string                      `json:"channel_id"`
	ChannelName  string                      `json:"channel_name,omitempty"`
	Mentions     []client.ResolvedMention    `json:"mentions,omitempty"`
	ThreadParent *client.SlackMessageSummary `json:"thread_parent,omitempty"`
	Payload      client.WebAPIMessage        `json:"payload"`
	TS           string                      `json:"ts,omitempty"`
}

// PreviewSlackMessageAsUserOutput is the structured output for preview_slack_message_as_user.
type PreviewSlackMessageAsUserOutput struct {
	OK        bool   `json:"ok"`
	Transport string `json:"transport"`
	ChannelID string `json:"channel_id"`
	// ChannelName is resolved through conversations.info so the caller can confirm
	// the human-readable destination (e.g. "#prj-foo") rather than just a channel ID.
	ChannelName string `json:"channel_name,omitempty"`
	// Mentions is resolved through users.info for each entry in the input's
	// mentions field, so the caller can confirm who will actually be notified.
	Mentions []client.ResolvedMention `json:"mentions,omitempty"`
	// ThreadParent is the message this preview would reply to, when thread_ts is
	// set, so the caller can confirm the reply lands on the intended thread.
	ThreadParent *client.SlackMessageSummary `json:"thread_parent,omitempty"`
	Payload      client.WebAPIMessage        `json:"payload"`
}

// UpdateSlackMessageInput is the input for update_slack_message.
type UpdateSlackMessageInput struct {
	ChannelID   string           `json:"channel_id,omitempty" jsonschema:"更新対象のチャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	TS          string           `json:"ts" jsonschema:"更新対象メッセージのts。post_slack_message_as_user の戻り値を利用できます。"`
	Text        string           `json:"text,omitempty" jsonschema:"更新後の本文。blocks または attachments を指定しない場合は必須です。"`
	Blocks      []map[string]any `json:"blocks,omitempty" jsonschema:"更新後のSlack Block Kit blocks配列。指定すると既存のblocksを置き換えます。"`
	Attachments []map[string]any `json:"attachments,omitempty" jsonschema:"更新後のSlack attachments配列。指定すると既存のattachmentsを置き換えます。"`
	Confirm     bool             `json:"confirm,omitempty" jsonschema:"true にすると実際に更新します。false（省略時）の場合は更新せず、更新対象メッセージの現在の内容と更新後の内容を返します。プレビューを確認したうえで confirm=true を指定して再実行してください。"`
}

// UpdateSlackMessageOutput is the structured output for update_slack_message.
type UpdateSlackMessageOutput struct {
	OK bool `json:"ok"`
	// Updated is false when confirm was omitted/false: the message was left alone,
	// and Current/Text describe the change that confirm=true would apply.
	Updated     bool   `json:"updated"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name,omitempty"`
	TS          string `json:"ts"`
	// Current is the message as it exists right now, so the caller can see what the
	// update would overwrite. See targetNote for why it may be absent.
	Current     *client.SlackMessageSummary `json:"current,omitempty"`
	CurrentNote string                      `json:"current_note,omitempty"`
	// Text is the requested new body before confirming, and Slack's echo of the
	// stored body afterwards.
	Text string `json:"text,omitempty"`
}

// DeleteSlackMessageInput is the input for delete_slack_message.
type DeleteSlackMessageInput struct {
	ChannelID string `json:"channel_id,omitempty" jsonschema:"削除対象のチャンネルID。省略時は MCP_SLACK_CHANNEL_ID を利用します。"`
	TS        string `json:"ts" jsonschema:"削除対象メッセージのts。post_slack_message_as_user の戻り値を利用できます。"`
	Confirm   bool   `json:"confirm,omitempty" jsonschema:"true にすると実際に削除します。false（省略時）の場合は削除せず、削除対象メッセージの内容を返します。削除は取り消せないため、プレビューを確認したうえで confirm=true を指定して再実行してください。"`
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
	targetNoteNotFound   = "対象メッセージが見つかりませんでした。ts とチャンネルを確認してください。"
	targetNoteUnreadable = "対象メッセージを取得できませんでした（履歴スコープ不足など）。内容を確認せずに実行することになります。"
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
// Failing to read the target is deliberately not an error. chat.update and chat.delete
// require no history scope, so a token may be fully authorized to destroy a message it
// cannot read; refusing to proceed would break that legitimate setup. The inability to
// show the old content is reported through Note instead, leaving the human to decide
// whether to confirm blind. A channel that cannot be resolved at all is still an
// error, since that means the destination itself is in doubt.
func (t *SlackTools) resolveTarget(ctx context.Context, channelID, ts string) (resolvedTarget, error) {
	channelInfo, err := t.client.GetChannelInfo(ctx, client.GetChannelInfoOptions{ChannelID: channelID})
	if err != nil {
		return resolvedTarget{}, err
	}

	target := resolvedTarget{
		ChannelID:   channelInfo.Channel.ID,
		ChannelName: channelInfo.Channel.Name,
	}

	message, err := t.client.GetMessage(ctx, target.ChannelID, ts)
	switch {
	case err != nil:
		target.Note = targetNoteUnreadable
	case message == nil:
		target.Note = targetNoteNotFound
	default:
		target.Message = message
	}
	return target, nil
}

func (t *SlackTools) previewSlackMessage(_ context.Context, _ *mcp.CallToolRequest, in PostSlackMessageInput) (*mcp.CallToolResult, PreviewSlackMessageOutput, error) {
	if err := in.validate(); err != nil {
		return nil, PreviewSlackMessageOutput{}, err
	}

	payload, err := t.client.PreviewMessage(in.toMessage(in.ThreadTS))
	if err != nil {
		return nil, PreviewSlackMessageOutput{}, err
	}

	return nil, PreviewSlackMessageOutput{
		OK:        true,
		Transport: "webhook",
		Mentions:  in.Mentions,
		Payload:   payload,
	}, nil
}

// postSlackMessage previews the webhook payload unconditionally, then only actually
// sends it (via client.PostMessage) when in.Confirm is set. This guarantees callers
// always see what would be posted before it happens, rather than relying on them to
// call preview_slack_message first.
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
// post_slack_message_as_user call would notify before it happens. Shared by
// previewSlackMessageAsUser and postSlackMessageAsUser.
func (t *SlackTools) buildWebAPIPreview(ctx context.Context, in PostSlackMessageAsUserInput) (PreviewSlackMessageAsUserOutput, error) {
	if err := in.validate(); err != nil {
		return PreviewSlackMessageAsUserOutput{}, err
	}

	payload, err := t.client.PreviewWebAPIMessage(in.toWebAPIMessage(in.ChannelID, in.ThreadTS))
	if err != nil {
		return PreviewSlackMessageAsUserOutput{}, err
	}

	out := PreviewSlackMessageAsUserOutput{
		OK:        true,
		Transport: "web_api",
		ChannelID: payload.ChannelID,
		Payload:   payload,
	}

	channelInfo, err := t.client.GetChannelInfo(ctx, client.GetChannelInfoOptions{ChannelID: payload.ChannelID})
	if err != nil {
		return PreviewSlackMessageAsUserOutput{}, err
	}
	out.ChannelName = channelInfo.Channel.Name

	if len(in.Mentions) > 0 {
		mentions, err := t.client.ResolveMentions(ctx, in.Mentions)
		if err != nil {
			return PreviewSlackMessageAsUserOutput{}, err
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
			return PreviewSlackMessageAsUserOutput{}, err
		}
		if len(replies.Messages) > 0 {
			out.ThreadParent = &replies.Messages[0]
		}
	}

	return out, nil
}

func (t *SlackTools) previewSlackMessageAsUser(ctx context.Context, _ *mcp.CallToolRequest, in PostSlackMessageAsUserInput) (*mcp.CallToolResult, PreviewSlackMessageAsUserOutput, error) {
	out, err := t.buildWebAPIPreview(ctx, in)
	if err != nil {
		return nil, PreviewSlackMessageAsUserOutput{}, err
	}
	return nil, out, nil
}

// postSlackMessageAsUser previews the chat.postMessage payload unconditionally
// (resolving channel name, mentions, and thread parent), then only actually sends it
// (via client.PostWebAPIMessage) when in.Confirm is set. This guarantees callers
// always see what would be posted, and who it would notify, before it happens, rather
// than relying on them to call preview_slack_message_as_user first.
func (t *SlackTools) postSlackMessageAsUser(ctx context.Context, _ *mcp.CallToolRequest, in PostSlackMessageAsUserInput) (*mcp.CallToolResult, PostSlackMessageAsUserOutput, error) {
	preview, err := t.buildWebAPIPreview(ctx, in)
	if err != nil {
		return nil, PostSlackMessageAsUserOutput{}, err
	}

	out := PostSlackMessageAsUserOutput{
		OK:           true,
		ChannelID:    preview.ChannelID,
		ChannelName:  preview.ChannelName,
		Mentions:     preview.Mentions,
		ThreadParent: preview.ThreadParent,
		Payload:      preview.Payload,
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
	if strings.TrimSpace(in.Text) == "" && len(in.Blocks) == 0 && len(in.Attachments) == 0 {
		return nil, UpdateSlackMessageOutput{}, fmt.Errorf("slack: text, blocks, or attachments is required")
	}

	target, err := t.resolveTarget(ctx, in.ChannelID, ts)
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
		Text:        in.Text,
	}
	if !in.Confirm {
		return nil, out, nil
	}

	updated, err := t.client.UpdateWebAPIMessage(ctx, client.UpdateWebAPIMessage{
		ChannelID:   target.ChannelID,
		TS:          ts,
		Text:        in.Text,
		Blocks:      in.Blocks,
		Attachments: in.Attachments,
	})
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
