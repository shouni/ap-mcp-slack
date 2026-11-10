// Package tools registers MCP tools.
package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/shouni/ap-mcp-slack/internal/client"
)

// ServerInstructions is advertised to the MCP client once, at initialize time, and
// typically lands in the model's system context. It carries only what holds across
// the whole tool surface — the confirm gate's two-step flow, the default channel,
// and how to continue paginated results — so the per-tool descriptions can stay
// about choosing between tools. Like those descriptions, it names no environment
// variables: configuration is the operator's side of the contract, not the model's.
const ServerInstructions = `Tools that write to Slack (post, update, delete) do nothing when confirm is omitted and return only a preview of exactly what would be sent or changed. Call them first without confirm, have the user check the preview, then rerun the same input with confirm=true. The preview's payload is exactly what is sent on confirmation. When channel_id is omitted, the default channel configured on the server is used, and if none is configured it is an error. Listing and history tools have more results only when next_cursor (next_page for workspace-wide search) is returned; when retry_after_seconds is present, wait that many seconds before fetching the rest.`

// SlackTools provides Slack-related MCP tools.
type SlackTools struct {
	client *client.SlackClient
}

// NewSlackTools creates SlackTools.
func NewSlackTools(c *client.SlackClient) *SlackTools {
	return &SlackTools{client: c}
}

// The annotation constructors below cover the three behaviour classes the tools fall
// into, so every registration declares which class it is in rather than repeating
// hint fields. OpenWorldHint is false throughout: every tool talks to the one Slack
// workspace the server was configured for, never to an open set of external systems.
// Each call returns a fresh value so no two tools alias the same hint storage.

// readOnlyAnnotations marks a tool that never writes to Slack. Clients use
// ReadOnlyHint to skip confirmation UI, so it must only ever go on tools where that
// is safe by construction.
func readOnlyAnnotations() *mcp.ToolAnnotations {
	closedWorld := false
	return &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &closedWorld}
}

// additiveWriteAnnotations marks a tool that creates new content but never destroys
// existing content (the post tools). Not idempotent: each confirmed call posts
// another message.
func additiveWriteAnnotations() *mcp.ToolAnnotations {
	additive := false
	closedWorld := false
	return &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &closedWorld}
}

// destructiveWriteAnnotations marks a tool that irreversibly replaces or removes
// existing content (update/delete). Idempotent: repeating the same confirmed call
// leaves Slack exactly as the first call did — a repeated update rewrites the same
// content, a repeated delete finds the message already gone.
func destructiveWriteAnnotations() *mcp.ToolAnnotations {
	destructive := true
	closedWorld := false
	return &mcp.ToolAnnotations{DestructiveHint: &destructive, IdempotentHint: true, OpenWorldHint: &closedWorld}
}

// Register registers the Slack tools each configured transport can actually serve.
//
// Registration is gated rather than unconditional because an MCP client's model
// picks tools from the advertised list and has no way to know which ones the
// process was given credentials for. Advertising a webhook tool to a token-only
// deployment invites the model to choose it, build a payload, get a human to approve
// the preview, and only then fail — so the tool is simply not offered.
//
// For the same reason there are no separate preview_* tools: every mutating tool
// returns its preview when confirm is unset, so previewing is the default path through
// the tool a model already picked, not a second tool it has to know to reach for
// first. A distinct preview tool could only add a way to skip the gate.
func (t *SlackTools) Register(server *mcp.Server) {
	if t.client.WebhookConfigured() {
		t.registerWebhookTools(server)
	}
	if t.client.WebAPIConfigured() {
		t.registerWebAPITools(server)
	}
}

// registerWebhookTools registers the tools backed by MCP_SLACK_WEBHOOK_URL.
func (t *SlackTools) registerWebhookTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_slack_message",
		Title:       "Post to Slack (webhook)",
		Description: "Posts a message to the Slack Incoming Webhook at MCP_SLACK_WEBHOOK_URL. With confirm=false (the default), nothing is posted and only a preview of the payload to be sent is returned.",
		Annotations: additiveWriteAnnotations(),
	}, t.postSlackMessage)
}

// registerWebAPITools registers the tools backed by a Slack Web API token.
//
// Descriptions say what a tool does and how to choose between neighbours, and leave out
// which environment variable supplied the token. A tool is only advertised once its
// credentials are present, so naming them tells the model something it cannot act on and
// cannot choose differently because of — while costing context on every request. Setup
// and OAuth scopes belong in docs/tools.md, which the operator reads.
func (t *SlackTools) registerWebAPITools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "post_slack_message_as_user",
		Title:       "Post to Slack (as the user)",
		Description: "Posts a message to Slack (chat.postMessage), in the token owner's own name. With confirm=false (the default), nothing is posted and a preview with the channel name, mentioned users and thread parent message resolved is returned. When posted, it returns channel_id and ts, and ts can be passed to update_slack_message / delete_slack_message as is.",
		Annotations: additiveWriteAnnotations(),
	}, t.postSlackMessageAsUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_slack_message",
		Title:       "Update Slack message",
		Description: "Rewrites the content of a posted message (chat.update). Only the original poster's own messages can be rewritten, and text/blocks/attachments replace the existing content wholesale (no partial updates). With confirm=false (the default), nothing is updated and a preview showing the current body next to the updated payload is returned.",
		Annotations: destructiveWriteAnnotations(),
	}, t.updateSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_slack_message",
		Title:       "Delete Slack message",
		Description: "Deletes a message (chat.delete). Deletion cannot be undone. With confirm=false (the default), nothing is deleted and the content of the target message is returned as a preview.",
		Annotations: destructiveWriteAnnotations(),
	}, t.deleteSlackMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_channel_info",
		Title:       "Get channel info",
		Description: "Fetches the details of a single channel from its channel ID (conversations.info). When the ID is already known, use this before paging through a listing.",
		Annotations: readOnlyAnnotations(),
	}, t.getSlackChannelInfo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_slack_channels",
		Title:       "List channels",
		Description: "Lists the channels of the whole workspace (conversations.list). When only the channels the token owner has joined are wanted, list_joined_slack_channels is faster and more reliable. The order is applied locally to the fetched results.",
		Annotations: readOnlyAnnotations(),
	}, t.listSlackChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_joined_slack_channels",
		Title:       "List joined channels",
		Description: "Lists only the channels the token owner has joined (users.conversations). The filtering is done on Slack's side, so it is more efficient than filtering list_slack_channels locally. Use this when asked for \"the channels I am in\".",
		Annotations: readOnlyAnnotations(),
	}, t.listJoinedSlackChannels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_channel_history",
		Title:       "Get channel history",
		Description: "Fetches a channel's message history, newest first (conversations.history). Only top-level messages are included. To read replies in a thread, use get_slack_thread_replies.",
		Annotations: readOnlyAnnotations(),
	}, t.getSlackChannelHistory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_thread_replies",
		Title:       "Get thread replies",
		Description: "Fetches the replies of the given thread (conversations.replies). Pass the thread parent's ts as ts (the thread_ts in get_slack_channel_history results is that value).",
		Annotations: readOnlyAnnotations(),
	}, t.getSlackThreadReplies)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_slack_messages",
		Title:       "Search messages",
		Description: "Full-text search across the whole workspace through Slack's search index (search.messages). Use this to find something that \"must have come up somewhere\" without naming a channel. When the channel is known and you want to read it in order, get_slack_channel_history fits better. Slack's search syntax (in: / from: / before: / after: / has:) works as is in query. Results come back one page at a time, ordered by relevance; to continue, pass next_page as page.",
		Annotations: readOnlyAnnotations(),
	}, t.searchSlackMessages)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_slack_users",
		Title:       "List and search users",
		Description: "Lists and searches workspace members (users.list). Deactivated users are excluded by default. To pin down one specific person as a recipient, use resolve_slack_user rather than picking from this list.",
		Annotations: readOnlyAnnotations(),
	}, t.listSlackUsers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "lookup_slack_user_by_email",
		Title:       "Look up user by email",
		Description: "Fetches a single user by exact email address match (users.lookupByEmail). To search by name, or to get candidates when nothing is found, use resolve_slack_user.",
		Annotations: readOnlyAnnotations(),
	}, t.lookupSlackUserByEmail)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resolve_slack_user",
		Title:       "Resolve user",
		Description: "Uniquely identifies a Slack user from name or email. Use this when deciding whom to mention. With several candidates it does not pick one automatically and returns status=\"ambiguous\" with the candidates, so do not post while it is still ambiguous. With status=\"found\", the mention can be embedded in the body as is. When search_truncated=true, not_found only means \"not within the range that was searched\".",
		Annotations: readOnlyAnnotations(),
	}, t.resolveSlackUser)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_slack_auth_info",
		Title:       "Check auth info",
		Description: "Returns which workspace, user and bot the configured token authenticates as (auth.test). It needs no OAuth scope, so it helps isolate the problem when other tools fail for missing scopes.",
		Annotations: readOnlyAnnotations(),
	}, t.getSlackAuthInfo)
}
