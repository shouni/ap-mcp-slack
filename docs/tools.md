# ツールリファレンス (Tools Reference)

`ap-mcp-slack` が提供する各 MCP ツールの入力フィールド・OAuthスコープの詳細です。ツール一覧・導入手順・ビルド方法は [README.md](../README.md) を参照してください。

## ツールの登録条件

ツールは、対応するトランスポートの認証情報が設定されている場合にのみ登録されます。

| 設定 | 登録されるツール |
| --- | --- |
| `MCP_SLACK_WEBHOOK_URL` のみ | `post_slack_message` |
| トークンのみ（`MCP_SLACK_USER_TOKEN` / `MCP_SLACK_TOKEN` / `MCP_SLACK_BOT_TOKEN`） | Web API 系の全ツール |
| 両方 | すべて |
| どちらも未設定 | 起動時にエラー終了します |

これは、MCPクライアント側のモデルが「広告されているツール一覧」から選択するためです。認証情報のないトランスポートのツールを一覧に載せると、モデルがそれを選び、payloadを組み立て、人間にプレビューを承認させたうえで送信時に初めて失敗する、という最悪の順序になります。使えないツールは最初から見せません。

## プレビューは `confirm` の既定動作です

Slackを変更するツール（`post_slack_message` / `post_slack_message_as_user` / `update_slack_message` / `delete_slack_message`）はすべて、`confirm` を省略/`false` にすると**Slackに一切書き込まず**、実際に送信される payload をプレビューとして返します。

`preview_*` という独立したツールは意図的に用意していません。同じ payload に到達する経路が2つあると、モデルがどちらを選ぶかに安全性が依存してしまいます。プレビューは「モデルがすでに選んだツールの既定の動作」であり、別に呼び出しておくべき前段の手順ではありません。

## `post_slack_message`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `text` | 必須 | Slackに投稿する本文。デフォルトでSlackのmrkdwnとして解釈されます。 |
| `blocks` | 任意 | 任意のSlack Block Kit blocks配列。指定する場合もアクセシビリティ用にtextを含めてください。 |
| `attachments` | 任意 | Slack attachments 配列。 |
| `thread_ts` | 任意 | スレッド返信にする場合の親メッセージts。Webhook側で利用可能な場合のみ有効です。 |
| `icon_emoji` | 任意 | 投稿者アイコンとして使うSlack絵文字名。例: `:robot_face:` |
| `unfurl_links` | 任意 | リンク展開の制御。 |
| `unfurl_media` | 任意 | メディア展開の制御。 |
| `mentions` | 任意 | メンション対象のSlackユーザーID配列（例: `["U0123456"]`）。本文の先頭に `<@ID>` 形式で追加されます。`blocks` との併用はエラーになります（後述）。 |
| `confirm` | 任意 | `true` にすると実際に投稿します。省略/`false` の場合は投稿せず、プレビューのみ返します。 |

## `post_slack_message_as_user`

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
| `mentions` | 任意 | メンション対象のSlackユーザーID配列（例: `["U0123456"]`）。本文の先頭に `<@ID>` 形式で追加されます。`blocks` との併用はエラーになります（後述）。プレビュー時（`confirm`未指定時）はさらに `users.info` で表示名解決した結果を `mentions` フィールド（`id`/`real_name`/`display_name`/`mention` など）として返します。 |
| `confirm` | 任意 | `true` にすると実際に投稿します。省略/`false` の場合は投稿せず、チャンネル名・メンション先・スレッド元メッセージを解決したプレビューのみ返します。 |

`post_slack_message_as_user` のプレビューは、送信先チャンネル解決のため `channel_id` または `MCP_SLACK_CHANNEL_ID` を必要とします。投稿前に一目で確認できるよう、`payload`（source label 付与後の実送信内容）に加えて次の情報も解決して返します（`conversations.info` / `users.info` / `conversations.replies` を追加で呼び出すため、対応するOAuthスコープが必要です）。

- `channel_name`: 送信先チャンネルの表示名（`channel_id` を `conversations.info` で解決）
- `mentions`: `mentions` フィールドで渡した各ユーザーIDの表示名（`users.info` で解決）
- `thread_parent`: `thread_ts` を指定した場合、返信先となる親メッセージの内容（`conversations.replies` で取得）

内容を確認した上で同じ入力に `confirm: true` を足して再実行すると、実際に投稿されます（`posted: true` と `ts` が返ります）。`payload` は `confirm` の有無にかかわらず同一で、プレビューで見た payload がそのまま Slack に送られます。

`post_slack_message`（Webhook側）も同様に `confirm` を省略/`false` にすると投稿せず `posted: false` を返しますが、Webhookには宛先チャンネルIDの概念がなく`conversations.info`等も呼ばないため、`mentions` は表示名解決されない生のSlackユーザーID配列のまま返り、`channel_name` / `thread_parent` は含まれません。

なお `post_slack_message` は `confirm` 未指定（Slackへ何も送らない場合）でも `MCP_SLACK_WEBHOOK_URL` を要求します。プレビューは人間が承認する対象であり、そもそも配送不可能な payload を承認させてしまうと、設定ミスが発覚するのが「承認後」になるためです。

### `mentions` と `blocks` の併用について

`mentions` と `blocks` を同時に指定するとエラーになります。`mentions` は `text` の先頭に `<@ID>` を挿入しますが、Slackは `blocks` があるとそちらを本文として描画し、`text` は通知フォールバックにしか使いません。つまり併用すると「通知は飛ぶが、メッセージ本文のどこにもメンションが見えない」という状態になります。`blocks` を使う場合は、`blocks` 内の任意の位置に `<@ID>` を自分で書いてください（どこに置くべきかは呼び出し側の判断であり、サーバー側では推測しません）。

## `update_slack_message`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `ts` | 必須 | 更新対象メッセージのts。`post_slack_message_as_user` の戻り値を利用できます。 |
| `channel_id` | 任意 | 更新対象のチャンネルID。省略時は `MCP_SLACK_CHANNEL_ID` を利用します。 |
| `text` | 任意 | 更新後の本文。`blocks` または `attachments` を指定しない場合は必須です。 |
| `blocks` | 任意 | 更新後のSlack Block Kit blocks配列。指定すると既存のblocksを置き換えます。 |
| `attachments` | 任意 | 更新後のSlack attachments配列。指定すると既存のattachmentsを置き換えます。 |
| `confirm` | 任意 | `true` にすると実際に更新します。省略/`false` の場合は更新せず、プレビューのみ返します。 |

`update_slack_message` で更新できるのは元の投稿者本人（`MCP_SLACK_USER_TOKEN` なら同じユーザー、Botトークンなら同じBot）が投稿したメッセージのみです。`post_slack_message_as_user` と同様、`text`/`blocks`/`attachments` は既存の内容を丸ごと置き換えます（一部だけの差分更新はできません）。

投稿系ツールと同じく `confirm` ゲートがあります。`confirm` を省略/`false` にすると更新は行わず、`updated: false` と合わせて次を返します。

- `channel_name`: 対象チャンネルの表示名
- `current`: **更新前の**メッセージ内容（上書きで消える内容）
- `payload`: 適用しようとしている更新後の payload（source label 付与後、`blocks` / `attachments` を含む実送信内容）
- `text`: 適用しようとしている更新後の本文

`text` だけでは足りないため `payload` も返します。`blocks` のみを指定した更新では `text` は空のままなので、`text` だけを見せると「上書き後の内容が空」に見え、人間が中身を確認せずに承認することになります。

## `delete_slack_message`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `ts` | 必須 | 削除対象メッセージのts。`post_slack_message_as_user` の戻り値を利用できます。 |
| `channel_id` | 任意 | 削除対象のチャンネルID。省略時は `MCP_SLACK_CHANNEL_ID` を利用します。 |
| `confirm` | 任意 | `true` にすると実際に削除します。省略/`false` の場合は削除せず、プレビューのみ返します。 |

削除はこのサーバーが提供する操作の中で最も取り返しがつかない（Slackにundoはありません）ため、投稿系と同じ `confirm` ゲートを持ちます。`confirm` を省略/`false` にすると削除は行わず、`deleted: false` と `channel_name`、そして `target`（削除されるメッセージそのものの内容）を返します。

### 対象が取得できない場合

`update_slack_message` / `delete_slack_message` のプレビューは、`conversations.info` でチャンネル名を、`conversations.history`（スレッド返信の場合は `conversations.replies`）で対象メッセージを取得します。しかし `chat.update` / `chat.delete` 自体は履歴スコープも `channels:read` / `groups:read` も必要としません。つまり「消せるが読めない」トークンが正当に存在します。

そのためどちらの解決に失敗しても操作はブロックされず、解決できなかった項目を省略したうえで `current_note` / `target_note` に理由を入れて返します。

| 状況 | 挙動 |
| --- | --- |
| メッセージを読めない（履歴スコープ不足など） | `current` / `target` を省略し、note に理由を記載 |
| チャンネル名を解決できない（`channels:read` / `groups:read` 不足など） | `channel_name` を省略し、`channel_id` はそのまま返して note に理由を記載 |
| `channel_id` 未指定かつ `MCP_SLACK_CHANNEL_ID` 未設定 | エラー（宛先が特定できず、操作対象が存在しない） |

note が入っている場合は**内容を確認しないまま実行することになる**というシグナルなので、承認前に必ず確認してください。ここでエラーにしてしまうと、そのメッセージを削除する手段がこのサーバーから一切失われるため、あえて降格して人間に判断を委ねています。

対象の特定は ts の完全一致で行います。`conversations.history` はトップレベルのメッセージしか返さないため、スレッド返信の ts を渡すと Slack は「直近の古い親メッセージ」を返してきます。それを削除確認画面に出すと全く別のメッセージを見せることになるので、ts が一致しない場合は `conversations.replies` にフォールバックしてスレッド内を走査します。

## `list_slack_channels`

| フィールド | 必須 | 説明 |
| --- | :---: | --- |
| `types` | 任意 | 取得する会話種別。`public_channel`, `private_channel`, `mpim`, `im` を指定できます。省略時は Slack API のデフォルト `public_channel` です。 |
| `exclude_archived` | 任意 | `true` の場合、アーカイブ済みチャンネルを除外します。 |
| `limit` | 任意 | 最大取得件数。省略時は `200`、最大 `1000` です。Slackが1ページで返す件数の端数により、返却件数がこれを僅かに超えることがあります（後述）。 |
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
| `limit` | 任意 | 最大取得件数。省略時は `200`、最大 `1000` です。Slackが1ページで返す件数の端数により、返却件数がこれを僅かに超えることがあります（後述）。 |
| `cursor` | 任意 | 続きから取得する場合の Slack pagination cursor。 |
| `team_id` | 任意 | Enterprise Grid の org-level token で対象ワークスペースを指定する場合に使います。 |
| `sort` | 任意 | 取得した結果に適用する返却前の並び順。`none`, `name_asc`, `name_desc`, `created_asc`, `created_desc` を指定できます。省略時は `name_asc` です。 |

`list_slack_channels` がワークスペース全体を返すのに対し、`list_joined_slack_channels` は `users.conversations` を使うため、サーバー側でトークン所有者のメンバーシップに絞り込まれた結果のみが返ります。`MCP_SLACK_USER_TOKEN`（ユーザートークン）を設定していればそのユーザー本人が参加しているチャンネル、ボットトークンのみの場合はそのボットが参加しているチャンネルが対象です。入力フィールドは `list_slack_channels` と同一です。

### `limit` と `next_cursor` の関係

`limit` に達するまで Slack のページを順に辿り、`next_cursor` には続きから取得するためのカーソルを返します。このとき、Slackが1ページで返してきた項目は `limit` を超えても**すべて返します**（`limit` はページ単位の残数に丸めて要求するため、超過分はごく僅かです）。

Slackのカーソルは「そのページ全体の後ろ」を指すため、ページの途中で打ち切って残りを捨てると、返した `next_cursor` でその捨てた項目を飛び越してしまい、以降どうページングしても取得できなくなります。`limit` を厳密に守るより取りこぼさないほうを優先しています。同じ理由で `list_slack_users` も同様に振る舞います。

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
| `limit` | 任意 | 最大取得件数。省略時は `200`、最大 `1000` です。Slackが1ページで返す件数の端数により、返却件数がこれを僅かに超えることがあります（後述）。 |
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

`name` 検索は 5000 人までを走査して打ち切ります。打ち切りに達した場合は `search_truncated: true` が付きます。これは「ワークスペースにいない」と「探しきれなかった」を区別するためのフラグで、`true` のときの `not_found` は **走査した範囲にいなかった** という意味しかありません。より広く探す必要がある場合は `list_slack_users` に `query` を渡して自分でページングしてください。なお完全一致で1件に決まった場合は、走査を打ち切っていても `search_truncated` は付きません（未走査のメンバーがこれより良い一致になることはないため）。部分一致で1件だけ当たった場合は、打ち切られていれば `search_truncated: true` が付きます。

## `get_slack_auth_info`

入力フィールドを取りません。設定されたトークン（`MCP_SLACK_USER_TOKEN` / `MCP_SLACK_TOKEN` / `MCP_SLACK_BOT_TOKEN`）が実際にどの Slack ワークスペース・ユーザー・Botとして認証されるかを、`team` / `user` / `bot_id` などで返します。他のツールと異なり OAuthスコープを一切必要としないため、「トークンは設定したのに他のツールがエラーになる」ときの切り分けに使えます。

## 必要な Slack トークンスコープ

| スコープ | 用途 |
| --- | --- |
| `chat:write` | `post_slack_message_as_user` / `update_slack_message` / `delete_slack_message` |
| `channels:read` | `list_slack_channels` / `list_joined_slack_channels` / `get_slack_channel_info`（`public_channel`） / `post_slack_message_as_user` のチャンネル名解決 |
| `groups:read` | `list_slack_channels` / `list_joined_slack_channels` / `get_slack_channel_info` で `private_channel` を含める場合（`post_slack_message_as_user` がprivateチャンネル宛の場合も同様） |
| `channels:history` | `get_slack_channel_history` / `get_slack_thread_replies` / `post_slack_message_as_user`（`thread_ts` 指定時の親メッセージ表示）で public channel を読む場合 |
| `groups:history` | `get_slack_channel_history` / `get_slack_thread_replies` / `post_slack_message_as_user`（`thread_ts` 指定時の親メッセージ表示）で private channel を読む場合 |
| `users:read` | `list_slack_users` / `resolve_slack_user`（name検索） / `post_slack_message_as_user` の `mentions` 表示名解決 |
| `users:read.email` | `lookup_slack_user_by_email` / `resolve_slack_user`（email検索） |
| （不要） | `get_slack_auth_info` はOAuthスコープを問わずトークンの有効性のみ確認します |

`update_slack_message` / `delete_slack_message` は `chat:write` のみでも動作します。読み取り系スコープは対象メッセージやチャンネル名をプレビューに出すためだけに使われ、無い場合はプレビューの該当項目が省略されるだけで操作自体はブロックされません（[対象が取得できない場合](#対象が取得できない場合)を参照）。
