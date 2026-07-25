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
	targetNoteNotFound    = "対象メッセージが見つかりませんでした。ts とチャンネルを確認してください。"
	targetNoteUnreadable  = "対象メッセージを取得できませんでした（履歴スコープ不足など）。内容を確認せずに実行することになります。"
	targetNoteUnnamedChan = "チャンネル名を解決できませんでした（channels:read / groups:read スコープ不足など）。channel_id が意図した宛先か確認してください。"
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

	message, err := t.client.GetMessage(ctx, target.ChannelID, ts)
	switch {
	case err != nil:
		notes = append(notes, targetNoteUnreadable)
	case message == nil:
		notes = append(notes, targetNoteNotFound)
	default:
		target.Message = message
	}

	target.Note = strings.Join(notes, " ")
	return target, nil
}

// postSlackMessage previews the webhook payload unconditionally, then only actually
// sends it (via client.PostMessage) when in.Confirm is set. This guarantees callers
// always see what would be posted before it happens, without a separate preview tool
// they might skip.
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
// (via client.PostWebAPIMessage) when in.Confirm is set. This guarantees callers always
// see what would be posted, and who it would notify, before it happens, without a
// separate preview tool they might skip.
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
