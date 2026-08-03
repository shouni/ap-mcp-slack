package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	slackapi "github.com/slack-go/slack"
)

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
	// maxSearchPage is Slack's ceiling on search.messages' page parameter. Rejecting a
	// higher page here turns "Slack refuses this" into a message naming the actual
	// limit, and keeps the error identical whether or not a token is present.
	maxSearchPage = 100
)

// Sort values accepted by search_slack_messages' sort / sort_dir options.
const (
	SearchSortScore     = "score"
	SearchSortTimestamp = "timestamp"

	SearchSortDirAsc  = "asc"
	SearchSortDirDesc = "desc"
)

// SearchMessagesOptions configures Slack search.messages requests.
type SearchMessagesOptions struct {
	Query            string `json:"query,omitempty"`
	Limit            int    `json:"limit,omitempty"`
	Page             int    `json:"page,omitempty"`
	Sort             string `json:"sort,omitempty"`
	SortDirection    string `json:"sort_dir,omitempty"`
	TeamID           string `json:"team_id,omitempty"`
	IncludeRawBlocks bool   `json:"include_raw_blocks,omitempty"`
}

// SlackSearchMatch contains the fields returned for one search.messages match.
//
// It is deliberately not SlackMessageSummary: a search match is scoped to a whole
// workspace rather than to one already-known channel, so it must carry its own channel
// identity, while the thread bookkeeping (reply_count, reply_users) that history
// returns is absent from Slack's match payload.
type SlackSearchMatch struct {
	ChannelID   string `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
	IsPrivate   bool   `json:"is_private,omitempty"`
	User        string `json:"user,omitempty"`
	Username    string `json:"username,omitempty"`
	Text        string `json:"text,omitempty"`
	// Blocks and Attachments carry the raw Block Kit / attachment payload and are only
	// populated when the caller sets IncludeRawBlocks, matching the history tools: by
	// default their content is folded into Text instead.
	Blocks      any    `json:"blocks,omitempty"`
	Attachments any    `json:"attachments,omitempty"`
	TS          string `json:"ts,omitempty"`
	ThreadTS    string `json:"thread_ts,omitempty"`
	Permalink   string `json:"permalink,omitempty"`
}

// SearchMessagesResponse contains the relevant search.messages response fields.
//
// Paging is by page number rather than by cursor, so the fields describing it differ
// from the cursor-based listings: Page/PageCount/Total describe where in the result set
// this call landed, and NextPage is what to pass back to continue.
type SearchMessagesResponse struct {
	OK      bool               `json:"ok"`
	Query   string             `json:"query"`
	Matches []SlackSearchMatch `json:"matches"`
	Count   int                `json:"count"`
	// Total is how many messages Slack matched in the whole workspace, which is
	// normally far more than Count: it is what tells a caller whether the page they
	// are looking at is the answer or a sample of it.
	Total     int    `json:"total"`
	Page      int    `json:"page"`
	PageCount int    `json:"page_count"`
	HasMore   bool   `json:"has_more"`
	NextPage  int    `json:"next_page,omitempty"`
	Sort      string `json:"sort"`
	SortDir   string `json:"sort_dir"`
}

// SearchMessages runs a workspace-wide full-text search through search.messages.
//
// This is the only read here that is not scoped to a channel the caller already named.
// The alternative — listing channels and walking each one's history — cannot be made
// equivalent: it reads only channels the listing happened to return, within whatever
// history window the caller paged through, so a miss means "not in what we read"
// rather than "not in the workspace". Slack's own index is the only thing that can
// answer the latter.
//
// Unlike the cursor-based listings, this makes exactly one request per call rather than
// routing through collectPages. Two reasons: search.messages pages by page number and
// exposes no cursor, so there is nothing to resume from and nothing that can be dropped
// by stopping early; and results are ranked by relevance, so pages after the first are
// progressively worse matches that a caller should ask for deliberately rather than
// have this walk on their behalf. HasMore/NextPage say plainly that more exists.
//
// search.messages requires a user token with search:read — a bot token is rejected by
// Slack with not_allowed_token_type, which is surfaced as-is rather than guessed at
// from the token string.
func (w *webAPITransport) SearchMessages(ctx context.Context, opts SearchMessagesOptions) (*SearchMessagesResponse, error) {
	if err := w.requireToken(); err != nil {
		return nil, err
	}
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return nil, fmt.Errorf("slack: query is required")
	}
	limit, err := normalizeListLimit(opts.Limit, defaultSearchLimit, maxSearchLimit)
	if err != nil {
		return nil, err
	}
	page, err := normalizeSearchPage(opts.Page)
	if err != nil {
		return nil, err
	}
	sortBy, err := normalizeSearchSort(opts.Sort)
	if err != nil {
		return nil, err
	}
	sortDir, err := normalizeSearchSortDirection(opts.SortDirection)
	if err != nil {
		return nil, err
	}

	result, err := w.slackAPIClient.SearchMessagesContext(ctx, query, slackapi.SearchParameters{
		TeamID:        strings.TrimSpace(opts.TeamID),
		Sort:          sortBy,
		SortDirection: sortDir,
		Highlight:     slackapi.DEFAULT_SEARCH_HIGHLIGHT,
		Count:         limit,
		Page:          page,
	})
	if err != nil {
		return nil, fmt.Errorf("slack: search.messages failed: %w", err)
	}

	matches := summarizeSearchMatches(result.Matches, opts.IncludeRawBlocks)
	// Taken as a whole rather than field by field: SearchMessages embeds both Paging and
	// Pagination, and each has a Page, so an unqualified result.Page does not compile.
	paging := result.Paging
	// Slack echoes the page it served; fall back to the requested one for the empty
	// paging block it returns when nothing matched, so page never reads as 0.
	currentPage := paging.Page
	if currentPage <= 0 {
		currentPage = page
	}
	pageCount := paging.Pages
	hasMore := currentPage < pageCount
	nextPage := 0
	if hasMore {
		nextPage = currentPage + 1
	}

	return &SearchMessagesResponse{
		OK:        true,
		Query:     query,
		Matches:   matches,
		Count:     len(matches),
		Total:     result.Total,
		Page:      currentPage,
		PageCount: pageCount,
		HasMore:   hasMore,
		NextPage:  nextPage,
		Sort:      sortBy,
		SortDir:   sortDir,
	}, nil
}

func normalizeSearchPage(page int) (int, error) {
	if page == 0 {
		return slackapi.DEFAULT_SEARCH_PAGE, nil
	}
	if page < 1 {
		return 0, fmt.Errorf("slack: page must be 1 or greater")
	}
	if page > maxSearchPage {
		return 0, fmt.Errorf("slack: page must be %d or less", maxSearchPage)
	}
	return page, nil
}

func normalizeSearchSort(raw string) (string, error) {
	sortBy := strings.ToLower(strings.TrimSpace(raw))
	switch sortBy {
	case "":
		return SearchSortScore, nil
	case SearchSortScore, SearchSortTimestamp:
		return sortBy, nil
	default:
		return "", fmt.Errorf("slack: unsupported sort %q", raw)
	}
}

func normalizeSearchSortDirection(raw string) (string, error) {
	sortDir := strings.ToLower(strings.TrimSpace(raw))
	switch sortDir {
	case "":
		return SearchSortDirDesc, nil
	case SearchSortDirAsc, SearchSortDirDesc:
		return sortDir, nil
	default:
		return "", fmt.Errorf("slack: unsupported sort_dir %q", raw)
	}
}

func summarizeSearchMatches(matches []slackapi.SearchMessage, includeRawBlocks bool) []SlackSearchMatch {
	out := make([]SlackSearchMatch, 0, len(matches))
	for _, match := range matches {
		summary := SlackSearchMatch{
			ChannelID:   match.Channel.ID,
			ChannelName: match.Channel.Name,
			IsPrivate:   match.Channel.IsPrivate,
			User:        match.User,
			Username:    match.Username,
			Text:        match.Text,
			TS:          match.Timestamp,
			ThreadTS:    threadTSFromPermalink(match.Permalink),
			Permalink:   match.Permalink,
		}
		if strings.TrimSpace(summary.Text) == "" {
			summary.Text = fallbackText(match.Blocks, match.Attachments)
		}
		if includeRawBlocks {
			if len(match.Blocks.BlockSet) > 0 {
				summary.Blocks = match.Blocks.BlockSet
			}
			if len(match.Attachments) > 0 {
				summary.Attachments = match.Attachments
			}
		}
		out = append(out, summary)
	}
	return out
}

// threadTSFromPermalink recovers the thread a match belongs to from its permalink.
//
// Slack's search matches carry no thread_ts field, so without this a match that is a
// thread reply is indistinguishable from a top-level message, and the caller has no way
// to reach the surrounding conversation: get_slack_thread_replies needs the *parent's*
// ts, and the match only has its own. Slack does put thread_ts in the permalink query
// for replies, so it is read back from there. A permalink that does not parse, or has
// no thread_ts, simply yields "" — a top-level match is the ordinary case, not an error.
func threadTSFromPermalink(permalink string) string {
	if strings.TrimSpace(permalink) == "" {
		return ""
	}
	parsed, err := url.Parse(permalink)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get("thread_ts"))
}
