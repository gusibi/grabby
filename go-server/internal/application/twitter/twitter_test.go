package twitter

import (
	"context"
	"testing"

	"go-server/internal/domain/capture"
)

type fakeSender struct {
	req *capture.BrowserRequest
}

func (f *fakeSender) ResolveBrowserConnID(string) (string, error) { return "conn-1", nil }

func (f *fakeSender) SendMessage(_ context.Context, req *capture.BrowserRequest, _ string) (*capture.BrowserResponse, error) {
	f.req = req
	return &capture.BrowserResponse{
		Success: true,
		Result: capture.PageResult{Items: []any{map[string]any{
			"url":  "https://x.com/i/api/graphql/abc/Likes?variables=...",
			"body": sampleBody,
		}}},
	}, nil
}

func TestLikesUsesHistoryURL(t *testing.T) {
	fs := &fakeSender{}
	tweets, err := Likes(context.Background(), fs, nil, Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.req == nil {
		t.Fatal("expected an intercept request")
	}
	if got, want := fs.req.URL, "https://x.com/i/history/likes"; got != want {
		t.Errorf("url: got %q want %q", got, want)
	}
	if len(tweets) != 2 {
		t.Errorf("expected 2 tweets, got %d", len(tweets))
	}
}

// A trimmed-down sample mirroring X's GraphQL tweet "result" nesting.
const sampleBody = `{
  "data": {
    "search_by_raw_query": {
      "search_timeline": {
        "timeline": {
          "instructions": [
            {
              "type": "TimelineAddEntries",
              "entries": [
                {
                  "entryId": "tweet-1",
                  "content": {
                    "itemContent": {
                      "tweet_results": {
                        "result": {
                          "rest_id": "1001",
                          "core": {
                            "user_results": {
                              "result": {
                                "legacy": { "screen_name": "alice", "name": "Alice" }
                              }
                            }
                          },
                          "legacy": {
                            "full_text": "hello world",
                            "created_at": "Mon Jun 09 12:00:00 +0000 2025",
                            "favorite_count": 12,
                            "retweet_count": 3,
                            "reply_count": 1
                          }
                        }
                      }
                    }
                  }
                },
                {
                  "entryId": "tweet-2",
                  "content": {
                    "itemContent": {
                      "tweet_results": {
                        "result": {
                          "rest_id": "1002",
                          "core": {
                            "user_results": {
                              "result": { "legacy": { "screen_name": "bob", "name": "Bob" } }
                            }
                          },
                          "legacy": {
                            "full_text": "second tweet",
                            "favorite_count": 0,
                            "extended_entities": {
                              "media": [ { "media_url_https": "https://pbs.twimg.com/x.jpg" } ]
                            }
                          }
                        }
                      }
                    }
                  }
                }
              ]
            }
          ]
        }
      }
    }
  }
}`

func TestExtractTweets(t *testing.T) {
	tweets := extractTweets(sampleBody)
	if len(tweets) != 2 {
		t.Fatalf("expected 2 tweets, got %d", len(tweets))
	}

	first := tweets[0]
	if first.ID != "1001" {
		t.Errorf("id: got %q want 1001", first.ID)
	}
	if first.Text != "hello world" {
		t.Errorf("text: got %q", first.Text)
	}
	if first.Author != "alice" || first.AuthorName != "Alice" {
		t.Errorf("author: got %q/%q", first.Author, first.AuthorName)
	}
	if first.FavoriteCount != 12 || first.RetweetCount != 3 || first.ReplyCount != 1 {
		t.Errorf("counts: %+v", first)
	}
	if first.URL != "https://x.com/alice/status/1001" {
		t.Errorf("url: got %q", first.URL)
	}

	second := tweets[1]
	if len(second.Media) != 1 || second.Media[0] != "https://pbs.twimg.com/x.jpg" {
		t.Errorf("media: got %+v", second.Media)
	}
}

func TestExtractTweetsIgnoresNonTweets(t *testing.T) {
	// A body with no full_text anywhere should yield nothing, not panic.
	if got := extractTweets(`{"data":{"foo":{"bar":[1,2,3]}}}`); len(got) != 0 {
		t.Errorf("expected 0 tweets, got %d", len(got))
	}
	if got := extractTweets("not json"); got != nil {
		t.Errorf("expected nil on bad json, got %+v", got)
	}
}

func TestMatchesOp(t *testing.T) {
	if !matchesOp("https://x.com/i/api/graphql/abc/SearchTimeline?variables=...", []string{"SearchTimeline"}) {
		t.Error("should match SearchTimeline")
	}
	if matchesOp("https://x.com/i/api/graphql/abc/HomeTimeline", []string{"SearchTimeline"}) {
		t.Error("should not match HomeTimeline for SearchTimeline filter")
	}
}
