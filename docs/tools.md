# Tools Reference

Input fields and OAuth scopes for each MCP tool that `ap-mcp-slack` provides. For the tool list, setup and build instructions, see [README.md](../README.md).

## When tools are registered

A tool is registered only when the credentials for its transport are configured.

| Configuration | Registered tools |
| --- | --- |
| `MCP_SLACK_WEBHOOK_URL` only | `post_slack_message` |
| Token only (`MCP_SLACK_USER_TOKEN` / `MCP_SLACK_TOKEN` / `MCP_SLACK_BOT_TOKEN`) | Every Web API tool |
| Both | All |
| Neither | The server exits with an error at startup |

The one exception is `search_slack_messages`. `search.messages` can only be called with a user token, but the token type cannot be reliably told from the string, so it is registered under the same condition as the other Web API tools, and with a bot token Slack's `not_allowed_token_type` is returned as is.

This is because the model on the MCP client side chooses from "the list of tools it was shown". Listing a tool whose transport has no credentials leads to the worst possible order: the model picks it, builds a payload, has a human approve the preview, and only then fails on send. Tools that cannot work are never shown in the first place.

## Previewing is what `confirm` does by default

Every tool that changes Slack (`post_slack_message` / `post_slack_message_as_user` / `update_slack_message` / `delete_slack_message`) **writes nothing to Slack** when `confirm` is omitted or `false`, and returns the payload that would actually be sent as a preview.

There is deliberately no separate `preview_*` tool. With two routes to the same payload, safety would depend on which one the model picks. The preview is "the default behaviour of the tool the model already chose", not a preliminary step that has to be called separately.

## `post_slack_message`

| Field | Required | Description |
| --- | :---: | --- |
| `text` | Required | The message body to post to Slack. Interpreted as Slack mrkdwn by default. |
| `blocks` | Optional | An optional Slack Block Kit blocks array. Include `text` for accessibility even when specifying it. |
| `attachments` | Optional | A Slack attachments array. |
| `thread_ts` | Optional | The parent message ts when replying in a thread. Effective only when the webhook supports it. |
| `icon_emoji` | Optional | The Slack emoji name to use as the poster's icon. Example: `:robot_face:` |
| `unfurl_links` | Optional | Controls link unfurling. |
| `unfurl_media` | Optional | Controls media unfurling. |
| `mentions` | Optional | An array of Slack user IDs to mention (e.g. `["U0123456"]`). Prepended to the body as `<@ID>`. Combining it with `blocks` is an error (see below). |
| `confirm` | Optional | `true` actually posts. When omitted or `false`, nothing is posted and only a preview is returned. |

## `post_slack_message_as_user`

| Field | Required | Description |
| --- | :---: | --- |
| `text` | Required | The message body to post to Slack. Interpreted as Slack mrkdwn by default. |
| `channel_id` | Optional | The channel ID to post to. Defaults to `MCP_SLACK_CHANNEL_ID`. |
| `blocks` | Optional | An optional Slack Block Kit blocks array. Include `text` for accessibility even when specifying it. |
| `attachments` | Optional | A Slack attachments array. |
| `thread_ts` | Optional | The parent message ts when replying in a thread. |
| `icon_emoji` | Optional | The Slack emoji name to use as the poster's icon. Example: `:robot_face:` |
| `unfurl_links` | Optional | Controls link unfurling. |
| `unfurl_media` | Optional | Controls media unfurling. |
| `mentions` | Optional | An array of Slack user IDs to mention (e.g. `["U0123456"]`). Prepended to the body as `<@ID>`. Combining it with `blocks` is an error (see below). In a preview (when `confirm` is not set), the display names resolved through `users.info` are also returned in the `mentions` field (`id` / `real_name` / `display_name` / `mention` and so on). |
| `confirm` | Optional | `true` actually posts. When omitted or `false`, nothing is posted and only a preview with the channel name, mentioned users and thread parent message resolved is returned. |

The `post_slack_message_as_user` preview needs `channel_id` or `MCP_SLACK_CHANNEL_ID` to resolve the destination channel. So that it can be checked at a glance before posting, it resolves and returns the following in addition to `payload` (what is actually sent, after the source label is applied). Since this calls `conversations.info` / `users.info` / `conversations.replies` in addition, the matching OAuth scopes are required.

- `channel_name`: the display name of the destination channel (`channel_id` resolved through `conversations.info`)
- `mentions`: the display name of each user ID passed in the `mentions` field (resolved through `users.info`)
- `thread_parent`: when `thread_ts` is given, the content of the parent message being replied to (fetched through `conversations.replies`)

After checking the content, rerunning the same input with `confirm: true` added actually posts it (`posted: true` and `ts` are returned). `payload` is identical with or without `confirm`, and the payload seen in the preview is exactly what is sent to Slack.

`post_slack_message` (the webhook side) likewise posts nothing and returns `posted: false` when `confirm` is omitted or `false`, but a webhook has no notion of a destination channel ID and calls nothing like `conversations.info`, so `mentions` comes back as the raw array of Slack user IDs without display names, and `channel_name` / `thread_parent` are not included.

Note that `post_slack_message` requires `MCP_SLACK_WEBHOOK_URL` even without `confirm` (when nothing is sent to Slack). A preview is what a human approves, and having them approve a payload that cannot be delivered at all would mean a misconfiguration only surfaces "after approval".

### Combining `mentions` with `blocks`

Specifying `mentions` and `blocks` together is an error. `mentions` inserts `<@ID>` at the start of `text`, but when `blocks` is present Slack renders that as the message body and uses `text` only as the notification fallback. Combining them would therefore give "a notification goes out, but the mention is nowhere to be seen in the message body". When using `blocks`, write `<@ID>` yourself wherever you want it inside `blocks` (where it belongs is the caller's decision, and the server does not guess).

## `update_slack_message`

| Field | Required | Description |
| --- | :---: | --- |
| `ts` | Required | The ts of the message to update. The return value of `post_slack_message_as_user` can be used. |
| `channel_id` | Optional | The channel ID of the message to update. Defaults to `MCP_SLACK_CHANNEL_ID`. |
| `text` | Optional | The new message body. Required unless `blocks` or `attachments` is given. |
| `blocks` | Optional | The new Slack Block Kit blocks array. Replaces the existing blocks when given. |
| `attachments` | Optional | The new Slack attachments array. Replaces the existing attachments when given. |
| `confirm` | Optional | `true` actually updates. When omitted or `false`, nothing is updated and only a preview is returned. |

`update_slack_message` can only update messages posted by the original poster (the same user for `MCP_SLACK_USER_TOKEN`, the same bot for a bot token). As with `post_slack_message_as_user`, `text` / `blocks` / `attachments` replace the existing content wholesale (a partial, diff-style update is not possible).

Like the posting tools, it has a `confirm` gate. When `confirm` is omitted or `false`, nothing is updated, and along with `updated: false` it returns:

- `channel_name`: the display name of the target channel
- `current`: the message content **before** the update (what the overwrite will erase)
- `payload`: the updated payload about to be applied (what is actually sent, after the source label is applied, including `blocks` / `attachments`)
- `text`: the updated body about to be applied

`payload` is returned too because `text` alone is not enough. In an update that specifies only `blocks`, `text` stays empty, so showing only `text` would make "the content after the overwrite" look empty, and a human would approve without having seen it.

## `delete_slack_message`

| Field | Required | Description |
| --- | :---: | --- |
| `ts` | Required | The ts of the message to delete. The return value of `post_slack_message_as_user` can be used. |
| `channel_id` | Optional | The channel ID of the message to delete. Defaults to `MCP_SLACK_CHANNEL_ID`. |
| `confirm` | Optional | `true` actually deletes. When omitted or `false`, nothing is deleted and only a preview is returned. |

Deletion is the least reversible operation this server offers (Slack has no undo), so it has the same `confirm` gate as the posting tools. When `confirm` is omitted or `false`, nothing is deleted, and it returns `deleted: false`, `channel_name`, and `target` (the content of the message that would be deleted).

### When the target cannot be read

The `update_slack_message` / `delete_slack_message` previews fetch the channel name through `conversations.info` and the target message through `conversations.history` (`conversations.replies` for a thread reply). But `chat.update` / `chat.delete` themselves need neither a history scope nor `channels:read` / `groups:read`. In other words, a token that "can delete but cannot read" legitimately exists.

So a failure to resolve either one does not block the operation; the unresolved item is omitted and the reason is put in `current_note` / `target_note`.

| Situation | Behaviour |
| --- | --- |
| The message cannot be read (missing history scope, etc.) | `current` / `target` is omitted and the note states the reason |
| The target is a thread reply that cannot be reached within the first 1000 replies | `current` / `target` is omitted and the note says the search was cut short (**this is distinct from "not found"**: the ts may be correct and running it may succeed) |
| The channel name cannot be resolved (missing `channels:read` / `groups:read`, etc.) | `channel_name` is omitted, `channel_id` (or the value of `MCP_SLACK_CHANNEL_ID` when omitted) is returned as is, and the note states the reason |
| `channel_id` is not given and `MCP_SLACK_CHANNEL_ID` is not set | Error (no destination can be determined, so there is nothing to operate on) |

Whether there is a destination is decided from the configuration, so even when `channel_id` is omitted in favour of `MCP_SLACK_CHANNEL_ID`, a failing `conversations.info` does not block the operation.

A note is the signal that **you are about to run this without having seen the content**, so always check it before approving. Making this an error would leave this server with no way at all to delete that message, so it is deliberately downgraded and the decision is left to a human.

The target is identified by an exact ts match. `conversations.history` returns only top-level messages, so given a thread reply's ts, Slack returns "the newest top-level message at or before that ts". Showing that on the delete confirmation would show a completely different message, so when the ts does not match, it falls back to `conversations.replies` and walks the thread.

`conversations.replies` returns the thread **from its start** whichever ts in the thread is passed, so the walk follows the cursor (100 per page × at most 10 pages = 1000). An implementation that read only one page would report "not found" when the reply is the 101st or later, giving the wrong instruction "check the ts" right before a delete that would actually succeed.

## `list_slack_channels`

| Field | Required | Description |
| --- | :---: | --- |
| `types` | Optional | The conversation types to fetch. Accepts `public_channel`, `private_channel`, `mpim`, `im`. Defaults to the Slack API default, `public_channel`. |
| `exclude_archived` | Optional | When `true`, archived channels are excluded. |
| `limit` | Optional | The maximum number of items. Defaults to `200`, at most `1000`. Because of the remainder of what Slack returns in one page, the number returned can slightly exceed it (see below). |
| `cursor` | Optional | The Slack pagination cursor to continue from. |
| `team_id` | Optional | Used to name the target workspace with an Enterprise Grid org-level token. |
| `sort` | Optional | The order applied to the fetched results before returning them. Accepts `none`, `name_asc`, `name_desc`, `created_asc`, `created_desc`. Defaults to `name_asc`. |

The Slack API's `conversations.list` has no argument for ordering, so `sort` is applied locally by the MCP server to the fetched results.

To include `private_channel` in `list_slack_channels`, the token needs the `groups:read` scope (`channels:read` is enough for `public_channel` only).

## `list_joined_slack_channels`

| Field | Required | Description |
| --- | :---: | --- |
| `types` | Optional | The conversation types to fetch. Accepts `public_channel`, `private_channel`, `mpim`, `im`. Defaults to the Slack API default, `public_channel`. |
| `exclude_archived` | Optional | When `true`, archived channels are excluded. |
| `limit` | Optional | The maximum number of items. Defaults to `200`, at most `1000`. Because of the remainder of what Slack returns in one page, the number returned can slightly exceed it (see below). |
| `cursor` | Optional | The Slack pagination cursor to continue from. |
| `team_id` | Optional | Used to name the target workspace with an Enterprise Grid org-level token. |
| `sort` | Optional | The order applied to the fetched results before returning them. Accepts `none`, `name_asc`, `name_desc`, `created_asc`, `created_desc`. Defaults to `name_asc`. |

Whereas `list_slack_channels` returns the whole workspace, `list_joined_slack_channels` uses `users.conversations`, so only results narrowed server-side to the token owner's memberships come back. With `MCP_SLACK_USER_TOKEN` (a user token) set, these are the channels that user has joined; with only a bot token, the channels the bot has joined. Its input fields are the same as `list_slack_channels`.

### How `limit` relates to `next_cursor`

Slack's pages are followed in order until `limit` is reached, and `next_cursor` returns the cursor to continue from. Here, the items Slack returned in a page are **all returned** even past `limit` (`limit` is requested rounded to the remainder per page, so the excess is very small).

Slack's cursor points "after that whole page", so cutting a page off midway and discarding the rest would make the returned `next_cursor` skip over the discarded items, leaving them unreachable however you page afterwards. Not dropping anything takes priority over honouring `limit` exactly. `list_slack_users` behaves the same way for the same reason.

Also, one tool call issues requests for **at most 20 pages**. This prevents walking the whole workspace without ever reaching `limit`, as when `list_slack_users` is given a `query` that matches few users (the per-request timeout does not limit the **number** of requests). Reaching the cap is not an error either: `next_cursor` is returned as usual, so pass it in another call to continue.

Likewise, when a Slack rate limit (HTTP 429) is hit midway through the page walk, the pages already fetched are returned rather than discarded. `next_cursor` then points to **the refused page**, so nothing is skipped, and the response's `retry_after_seconds` holds the wait Slack asked for. If you need more, wait that many seconds and pass `next_cursor`. Failing with an error and discarding the fetched pages would only make the retry spend again the request budget that was just used up. Only a rate limit on the first page is an error, since there is no result to hand back (the message includes Slack's retry after).

## `get_slack_channel_info`

| Field | Required | Description |
| --- | :---: | --- |
| `channel_id` | Optional | The channel ID to fetch. Defaults to `MCP_SLACK_CHANNEL_ID`. |
| `include_num_members` | Optional | When `true`, `num_members` is included. |
| `include_locale` | Optional | When `true`, locale information is included. |

Whereas `list_slack_channels` / `list_joined_slack_channels` return a list of channels, `get_slack_channel_info` fetches just one channel's details when its ID is known, without paging through the whole workspace.

## `get_slack_channel_history` / `get_slack_thread_replies`

Main inputs of `get_slack_channel_history`:

| Field | Required | Description |
| --- | :---: | --- |
| `channel_id` | Optional | The channel ID to fetch. Defaults to `MCP_SLACK_CHANNEL_ID`. |
| `limit` | Optional | The maximum number of items. Defaults to `100`, at most `1000`. |
| `cursor` | Optional | The Slack pagination cursor to continue from. |
| `oldest` | Optional | Fetch only messages after this Unix timestamp. |
| `latest` | Optional | Fetch only messages before this Unix timestamp. |
| `inclusive` | Optional | Also include messages with the same timestamp as `oldest` / `latest`. |
| `include_all_metadata` | Optional | When `true`, Slack message metadata is fetched too. |
| `include_raw_blocks` | Optional | When `true`, the raw Block Kit blocks and attachments are fetched too. By default only the text summary is returned, which keeps token use down. |

Main inputs of `get_slack_thread_replies`:

| Field | Required | Description |
| --- | :---: | --- |
| `ts` | Required | The parent message ts. Give the ts of the thread parent, not of a reply. |
| `channel_id` | Optional | The channel ID to fetch. Defaults to `MCP_SLACK_CHANNEL_ID`. |
| `limit` | Optional | The maximum number of items. Defaults to `100`, at most `1000`. |
| `cursor` | Optional | The Slack pagination cursor to continue from. |
| `oldest` | Optional | Fetch only replies after this Unix timestamp. |
| `latest` | Optional | Fetch only replies before this Unix timestamp. |
| `inclusive` | Optional | Also include replies with the same timestamp as `oldest` / `latest`. |
| `include_all_metadata` | Optional | When `true`, Slack message metadata is fetched too. |
| `include_raw_blocks` | Optional | When `true`, the raw Block Kit blocks and attachments are fetched too. By default only the text summary is returned, which keeps token use down. |

`get_slack_channel_history` / `get_slack_thread_replies` need the `channels:history` scope for public channels and `groups:history` for private channels. When reading with a bot token, the bot must be a member of the target channel.

## `search_slack_messages`

| Field | Required | Description |
| --- | :---: | --- |
| `query` | Required | The search query. Slack's search syntax works as is (see below). |
| `limit` | Optional | The maximum number of items per page. Defaults to `20`, at most `100`. |
| `page` | Optional | The page number to fetch (1-based). Defaults to `1`, at most `100`. |
| `sort` | Optional | The sort key: `score` (relevance) or `timestamp` (posting time). Defaults to `score`. |
| `sort_dir` | Optional | The sort direction: `asc` or `desc`. Defaults to `desc`. |
| `team_id` | Optional | Used to name the target workspace with an Enterprise Grid org-level token. |
| `include_raw_blocks` | Optional | When `true`, the raw Block Kit blocks and attachments are returned too. By default only the text summary is returned, which keeps token use down. |

Whereas `get_slack_channel_history` reads one channel's timeline, this searches the whole workspace through Slack's search index without naming a channel. Listing channels and reading each one's history only covers "the paged range of the channels that were listed", so not finding something that way is no evidence that "it is not in the workspace". Use this tool when you need a workspace-wide search.

### Query syntax

Slack's search modifiers work as is. Narrowing with modifiers is more precise than adding search terms.

| Example | Meaning |
| --- | --- |
| `in:#general` | Limit to the given channel |
| `from:@alice` | Limit to posts by the given user |
| `before:2026-01-31` / `after:2026-01-01` / `on:2026-01-15` | Limit by date |
| `has:link` / `has:reaction` | Limit to posts with a link or reaction |
| `"exact phrase"` | Match the whole phrase |

### Paging and returned fields

Paging is by page number, not by cursor. Each call makes exactly one `search.messages` call; to continue, pass the response's `next_page` as the next `page`. Unlike the other listing tools, the server does not walk multiple pages for you. The results are ordered by relevance, so later pages match more weakly (and are therefore something the caller should ask for explicitly).

| Field | Description |
| --- | --- |
| `matches` | The matched messages. Includes `channel_id` / `channel_name` / `is_private` / `user` / `ts` / `text` / `permalink`. |
| `matches[].thread_ts` | When a match is a thread reply, the **parent message's ts**. Can be passed to `get_slack_thread_replies` as is. Slack's search results do not return thread_ts themselves, so it is recovered from the permalink (empty for top-level posts). |
| `total` | The total number of matches across the workspace. Usually far larger than `count` (the number on this page), which helps judge "whether what you are looking at is the answer itself or only part of it". |
| `page` / `page_count` / `has_more` / `next_page` | The page position and whether there is more. |

This tool can only be used with **a user token + `search:read`**. With a bot token Slack returns `not_allowed_token_type` (it is returned as an error rather than an empty result, so that "search is not allowed" is never reported as "nothing found").

## `list_slack_users` / `lookup_slack_user_by_email` / `resolve_slack_user`

Main inputs of `list_slack_users`:

| Field | Required | Description |
| --- | :---: | --- |
| `query` | Optional | A case-insensitive substring search over `name` / `real_name` / `profile.display_name` / `email`. |
| `limit` | Optional | The maximum number of items. Defaults to `200`, at most `1000`. Because of the remainder of what Slack returns in one page, the number returned can slightly exceed it (see above). |
| `cursor` | Optional | The Slack pagination cursor to continue from. |
| `team_id` | Optional | Used to name the target workspace with an Enterprise Grid org-level token. |
| `include_deleted` | Optional | When `true`, deactivated (deleted) users are included too. Excluded by default. |

Main inputs of `lookup_slack_user_by_email`:

| Field | Required | Description |
| --- | :---: | --- |
| `email` | Required | The email address to look up. |

Main inputs of `resolve_slack_user`:

| Field | Required | Description |
| --- | :---: | --- |
| `name` | Optional | The user name, real name or display name to look up. Ignored when `email` is given. |
| `email` | Optional | The email address to look up. Takes precedence over `name` when given, and is resolved through `users.lookupByEmail`. |
| `team_id` | Optional | Used to name the target workspace with an Enterprise Grid org-level token. Used only when searching by `name`. |

When `email` is not given, `name` is first matched exactly (case-insensitively) against the `name` / `real_name` / `display_name` of the users fetched from `users.list`. If there is no exact match, it falls back to a substring match. Only when exactly one user matches does it return `status: "found"` with `user` and a `mention` in `<@U...>` form. With no match it returns `status: "not_found"`; with several, `status: "ambiguous"` with the candidates in `candidates`, and it never picks one automatically, to avoid sending to the wrong person.

A `name` search scans up to 4000 users (200 per page × 20 pages) and then stops. When it hits that cap, `search_truncated: true` is set. This flag separates "not in the workspace" from "could not finish looking": with `true`, `not_found` only means **not within the scanned range**. A Slack rate limit midway through the scan is treated the same way: it is not an error, and `search_truncated: true` is set. If you need to search more widely, pass a `query` to `list_slack_users` and page through it yourself.

Note that when an exact match settles it to one user, `search_truncated` is not set even if the scan was cut short (no unscanned member could be a better match). When a substring match hits exactly one user, `search_truncated: true` is set if the scan was cut short.

## `get_slack_auth_info`

Takes no input fields. Returns which Slack workspace, user and bot the configured token (`MCP_SLACK_USER_TOKEN` / `MCP_SLACK_TOKEN` / `MCP_SLACK_BOT_TOKEN`) actually authenticates as, through `team` / `user` / `bot_id` and so on. Unlike the other tools it needs no OAuth scope at all, so it helps isolate the problem when "the token is set but the other tools fail".

## Required Slack token scopes

| Scope | Used by |
| --- | --- |
| `chat:write` | `post_slack_message_as_user` / `update_slack_message` / `delete_slack_message` |
| `channels:read` | `list_slack_channels` / `list_joined_slack_channels` / `get_slack_channel_info` (`public_channel`) / channel name resolution in `post_slack_message_as_user` |
| `groups:read` | `list_slack_channels` / `list_joined_slack_channels` / `get_slack_channel_info` when including `private_channel` (likewise when `post_slack_message_as_user` targets a private channel) |
| `channels:history` | Reading public channels in `get_slack_channel_history` / `get_slack_thread_replies` / `post_slack_message_as_user` (showing the parent message when `thread_ts` is given) |
| `groups:history` | Reading private channels in `get_slack_channel_history` / `get_slack_thread_replies` / `post_slack_message_as_user` (showing the parent message when `thread_ts` is given) |
| `search:read` | `search_slack_messages`. **User tokens only**; a bot token gets `not_allowed_token_type` |
| `users:read` | `list_slack_users` / `resolve_slack_user` (name search) / display name resolution of `mentions` in `post_slack_message_as_user` |
| `users:read.email` | `lookup_slack_user_by_email` / `resolve_slack_user` (email search) |
| (none) | `get_slack_auth_info` checks only that the token is valid, whatever its OAuth scopes |

`update_slack_message` / `delete_slack_message` work with `chat:write` alone. The read scopes are used only to show the target message and channel name in the preview; without them the corresponding preview items are simply omitted, and the operation itself is not blocked (see [When the target cannot be read](#when-the-target-cannot-be-read)).
