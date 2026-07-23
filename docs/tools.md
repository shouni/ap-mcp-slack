# ツールリファレンス (Tools Reference)

`ap-mcp-slack` が提供する各 MCP ツールの入力フィールド・OAuthスコープの詳細です。ツール一覧・導入手順・ビルド方法は [README.md](../README.md) を参照してください。

## `preview_slack_message` / `post_slack_message`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `text` | 必須 | Slackに投稿する本文。デフォルトでSlackのmrkdwnとして解釈されます。 |
| `blocks` | 任意 | 任意のSlack Block Kit blocks配列。指定する場合もアクセシビリティ用にtextを含めてください。 |
| `attachments` | 任意 | Slack attachments 配列。 |
| `thread_ts` | 任意 | スレッド返信にする場合の親メッセージts。Webhook側で利用可能な場合のみ有効です。 |
| `icon_emoji` | 任意 | 投稿者アイコンとして使うSlack絵文字名。例: `:robot_face:` |
| `unfurl_links` | 任意 | リンク展開の制御。 |
| `unfurl_media` | 任意 | メディア展開の制御。 |
| `mentions` | 任意 | メンション対象のSlackユーザーID配列（例: `["U0123456"]`）。本文の先頭に `<@ID>` 形式で追加されます。`blocks` を指定した場合、本文はフォールバック表示にしか使われないため、`blocks` 内で明示的にメンションしてください。 |
| `confirm` | 任意（`post_slack_message` のみ） | `true` にすると実際に投稿します。省略/`false` の場合は投稿せず、プレビューのみ返します。 |

## `preview_slack_message_as_user` / `post_slack_message_as_user`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `text` | 必須 | Slackに投稿する本文。デフォルトでSlackのmrkdwnとして解釈されます。 |
| `channel_id` | 任意 | 投稿先チャンネルID。省略時は `MCP_SLACK_CHANNEL_ID` を利用します。 |
| `blocks` | 任意 | 任意のSlack Block Kit blocks配列。指定する場合もアクセシビリティ用にtextを含めてください。 |
| `attachments` | 任意 | Slack attachments 配列。 |
| `thread_ts` | 任意 | スレッド返信にする場合の親メッセージts。 |
| `icon_emoji` | 任意 | 投稿者アイコンとして使うSlack絵文字名。例: `:robot_face:` |
| `unfurl_links` | 任意 | リンク展開の制御。 |
| `unfurl_media` | 任意 | メディア展開の制御。 |
| `mentions` | 任意 | メンション対象のSlackユーザーID配列（例: `["U0123456"]`）。本文の先頭に `<@ID>` 形式で追加されます。`preview_slack_message_as_user` / `post_slack_message_as_user`（`confirm`未指定時）ではさらに `users.info` で表示名解決した結果を `mentions` フィールド（`id`/`real_name`/`display_name`/`mention` など）として返します。 |
| `confirm` | 任意（`post_slack_message_as_user` のみ） | `true` にすると実際に投稿します。省略/`false` の場合は投稿せず、チャンネル名・メンション先・スレッド元メッセージを解決したプレビューのみ返します。 |

`preview_slack_message` / `preview_slack_message_as_user` は Slack へ投稿せず、source label 付与後の payload を返します。`preview_slack_message_as_user` は送信先チャンネル解決のため `channel_id` または `MCP_SLACK_CHANNEL_ID` が必要です。さらに `preview_slack_message_as_user` は、投稿前に一目で確認できるよう次の情報も追加で解決して返します（`conversations.info` / `users.info` / `conversations.replies` を追加で呼び出すため、対応するOAuthスコープが必要です）。

- `channel_name`: 送信先チャンネルの表示名（`channel_id` を `conversations.info` で解決）
- `mentions`: `mentions` フィールドで渡した各ユーザーIDの表示名（`users.info` で解決）
- `thread_parent`: `thread_ts` を指定した場合、返信先となる親メッセージの内容（`conversations.replies` で取得）

`post_slack_message_as_user` も `confirm` を省略/`false` にすると、実際には投稿せず上記と同じプレビュー情報（`channel_name`/`mentions`/`thread_parent`、`posted: false` と合わせて）を返します。内容を確認した上で同じ入力に `confirm: true` を足して再実行すると、実際に投稿されます（`posted: true` と `ts` が返ります）。

`post_slack_message`（Webhook側）も同様に `confirm` を省略/`false` にすると投稿せず `posted: false` を返しますが、Webhookには宛先チャンネルIDの概念がなく`conversations.info`等も呼ばないため、`mentions` は表示名解決されない生のSlackユーザーID配列のまま返り、`channel_name` / `thread_parent` は含まれません。

## `update_slack_message`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `ts` | 必須 | 更新対象メッセージのts。`post_slack_message_as_user` の戻り値を利用できます。 |
| `channel_id` | 任意 | 更新対象のチャンネルID。省略時は `MCP_SLACK_CHANNEL_ID` を利用します。 |
| `text` | 任意 | 更新後の本文。`blocks` または `attachments` を指定しない場合は必須です。 |
| `blocks` | 任意 | 更新後のSlack Block Kit blocks配列。指定すると既存のblocksを置き換えます。 |
| `attachments` | 任意 | 更新後のSlack attachments配列。指定すると既存のattachmentsを置き換えます。 |

`update_slack_message` で更新できるのは元の投稿者本人（`MCP_SLACK_USER_TOKEN` なら同じユーザー、Botトークンなら同じBot）が投稿したメッセージのみです。`post_slack_message_as_user` と同様、`text`/`blocks`/`attachments` は既存の内容を丸ごと置き換えます（一部だけの差分更新はできません）。

## `delete_slack_message`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `ts` | 必須 | 削除対象メッセージのts。`post_slack_message_as_user` の戻り値を利用できます。 |
| `channel_id` | 任意 | 削除対象のチャンネルID。省略時は `MCP_SLACK_CHANNEL_ID` を利用します。 |

## `list_slack_channels`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `types` | 任意 | 取得する会話種別。`public_channel`, `private_channel`, `mpim`, `im` を指定できます。省略時は Slack API のデフォルト `public_channel` です。 |
| `exclude_archived` | 任意 | `true` の場合、アーカイブ済みチャンネルを除外します。 |
| `limit` | 任意 | 最大取得件数。省略時は `200`、最大 `1000` です。 |
| `cursor` | 任意 | 続きから取得する場合の Slack pagination cursor。 |
| `team_id` | 任意 | Enterprise Grid の org-level token で対象ワークスペースを指定する場合に使います。 |
| `sort` | 任意 | 取得した結果に適用する返却前の並び順。`none`, `name_asc`, `name_desc`, `created_asc`, `created_desc` を指定できます。省略時は `name_asc` です。 |

Slack API の `conversations.list` には並び順を指定する引数がないため、`sort` は MCP サーバーが取得した結果にローカルで適用します。

`list_slack_channels` で `private_channel` を含めて取得するには、トークンに `groups:read` スコープが必要です（`public_channel` のみなら `channels:read` で足ります）。

## `list_joined_slack_channels`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `types` | 任意 | 取得する会話種別。`public_channel`, `private_channel`, `mpim`, `im` を指定できます。省略時は Slack API のデフォルト `public_channel` です。 |
| `exclude_archived` | 任意 | `true` の場合、アーカイブ済みチャンネルを除外します。 |
| `limit` | 任意 | 最大取得件数。省略時は `200`、最大 `1000` です。 |
| `cursor` | 任意 | 続きから取得する場合の Slack pagination cursor。 |
| `team_id` | 任意 | Enterprise Grid の org-level token で対象ワークスペースを指定する場合に使います。 |
| `sort` | 任意 | 取得した結果に適用する返却前の並び順。`none`, `name_asc`, `name_desc`, `created_asc`, `created_desc` を指定できます。省略時は `name_asc` です。 |

`list_slack_channels` がワークスペース全体を返すのに対し、`list_joined_slack_channels` は `users.conversations` を使うため、サーバー側でトークン所有者のメンバーシップに絞り込まれた結果のみが返ります。`MCP_SLACK_USER_TOKEN`（ユーザートークン）を設定していればそのユーザー本人が参加しているチャンネル、ボットトークンのみの場合はそのボットが参加しているチャンネルが対象です。

## `get_slack_channel_info`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `channel_id` | 任意 | 取得対象チャンネルID。省略時は `MCP_SLACK_CHANNEL_ID` を利用します。 |
| `include_num_members` | 任意 | `true` の場合、`num_members` を含めて取得します。 |
| `include_locale` | 任意 | `true` の場合、ロケール情報を含めて取得します。 |

`list_slack_channels` / `list_joined_slack_channels` がチャンネル一覧を返すのに対し、`get_slack_channel_info` はチャンネルIDが分かっている場合に、ワークスペース全体をページングせず単一チャンネルの詳細だけを取得できます。

## `get_slack_channel_history` / `get_slack_thread_replies`

`get_slack_channel_history` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `channel_id` | 任意 | 取得対象チャンネルID。省略時は `MCP_SLACK_CHANNEL_ID` を利用します。 |
| `limit` | 任意 | 最大取得件数。省略時は `100`、最大 `1000` です。 |
| `cursor` | 任意 | 続きから取得する場合の Slack pagination cursor。 |
| `oldest` | 任意 | このUnix timestampより後のメッセージのみ取得します。 |
| `latest` | 任意 | このUnix timestampより前のメッセージのみ取得します。 |
| `inclusive` | 任意 | `oldest` / `latest` と同じtimestampのメッセージも含めます。 |
| `include_all_metadata` | 任意 | `true` の場合、Slackのメッセージメタデータも取得対象にします。 |
| `include_raw_blocks` | 任意 | `true` の場合、Block Kit blocksとattachmentsの生データも取得対象にします。省略時はテキストとして要約されたものだけを返し、トークン消費を抑えます。 |

`get_slack_thread_replies` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `ts` | 必須 | 親メッセージのts。返信メッセージのtsではなくスレッド親のtsを指定してください。 |
| `channel_id` | 任意 | 取得対象チャンネルID。省略時は `MCP_SLACK_CHANNEL_ID` を利用します。 |
| `limit` | 任意 | 最大取得件数。省略時は `100`、最大 `1000` です。 |
| `cursor` | 任意 | 続きから取得する場合の Slack pagination cursor。 |
| `oldest` | 任意 | このUnix timestampより後の返信のみ取得します。 |
| `latest` | 任意 | このUnix timestampより前の返信のみ取得します。 |
| `inclusive` | 任意 | `oldest` / `latest` と同じtimestampの返信も含めます。 |
| `include_all_metadata` | 任意 | `true` の場合、Slackのメッセージメタデータも取得対象にします。 |
| `include_raw_blocks` | 任意 | `true` の場合、Block Kit blocksとattachmentsの生データも取得対象にします。省略時はテキストとして要約されたものだけを返し、トークン消費を抑えます。 |

`get_slack_channel_history` / `get_slack_thread_replies` は、public channel には `channels:history`、private channel には `groups:history` スコープが必要です。Botトークンで読む場合、対象チャンネルにbotが参加している必要があります。

## `list_slack_users` / `lookup_slack_user_by_email` / `resolve_slack_user`

`list_slack_users` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `query` | 任意 | `name` / `real_name` / `profile.display_name` / `email` に対する部分一致検索（大文字小文字を区別しません）。 |
| `limit` | 任意 | 最大取得件数。省略時は `200`、最大 `1000` です。 |
| `cursor` | 任意 | 続きから取得する場合の Slack pagination cursor。 |
| `team_id` | 任意 | Enterprise Grid の org-level token で対象ワークスペースを指定する場合に使います。 |
| `include_deleted` | 任意 | `true` の場合、deactivate済み(deleted)ユーザーも含めます。省略時は除外されます。 |

`lookup_slack_user_by_email` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `email` | 必須 | 検索対象のメールアドレス。 |

`resolve_slack_user` の主な入力:

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `name` | 任意 | 検索対象のユーザー名・real name・display nameのいずれか。`email` が指定された場合は無視されます。 |
| `email` | 任意 | 検索対象のメールアドレス。指定された場合は `name` より優先され、`users.lookupByEmail` で解決します。 |
| `team_id` | 任意 | Enterprise Grid の org-level token で対象ワークスペースを指定する場合に使います。`name` での検索時のみ利用します。 |

`email` が指定されない場合、`name` はまず `users.list` から取得したユーザーの `name` / `real_name` / `display_name` との完全一致（大文字小文字を区別しない）を探します。完全一致が1件もない場合は部分一致にフォールバックします。一致が1件のときのみ `status: "found"` として `user` と `<@U...>` 形式の `mention` を返します。0件なら `status: "not_found"`、複数件なら `status: "ambiguous"` として `candidates` に候補一覧を返し、誤送信を避けるため自動選択はしません。

## `get_slack_auth_info`

入力フィールドを取りません。設定されたトークン（`MCP_SLACK_USER_TOKEN` / `MCP_SLACK_TOKEN` / `MCP_SLACK_BOT_TOKEN`）が実際にどの Slack ワークスペース・ユーザー・Botとして認証されるかを、`team` / `user` / `bot_id` などで返します。他のツールと異なり OAuthスコープを一切必要としないため、「トークンは設定したのに他のツールがエラーになる」ときの切り分けに使えます。

## 必要な Slack トークンスコープ

| スコープ | 用途 |
| --- | --- |
| `chat:write` | `post_slack_message_as_user` / `update_slack_message` / `delete_slack_message` |
| `channels:read` | `list_slack_channels` / `list_joined_slack_channels` / `get_slack_channel_info`（`public_channel`） / `preview_slack_message_as_user` のチャンネル名解決 |
| `groups:read` | `list_slack_channels` / `list_joined_slack_channels` / `get_slack_channel_info` で `private_channel` を含める場合（`preview_slack_message_as_user` がprivateチャンネル宛の場合も同様） |
| `channels:history` | `get_slack_channel_history` / `get_slack_thread_replies` / `preview_slack_message_as_user`（`thread_ts` 指定時の親メッセージ表示）で public channel を読む場合 |
| `groups:history` | `get_slack_channel_history` / `get_slack_thread_replies` / `preview_slack_message_as_user`（`thread_ts` 指定時の親メッセージ表示）で private channel を読む場合 |
| `users:read` | `list_slack_users` / `resolve_slack_user`（name検索） / `preview_slack_message_as_user` の `mentions` 表示名解決 |
| `users:read.email` | `lookup_slack_user_by_email` / `resolve_slack_user`（email検索） |
| （不要） | `get_slack_auth_info` はOAuthスコープを問わずトークンの有効性のみ確認します |
