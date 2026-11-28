package twitter

import (
	"context"
	"strings"
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

func TestExtractTweetsNoteTweetAndURLs(t *testing.T) {
	body := `{
		"result": {
			"rest_id": "2001",
			"views": {"count": "15800", "state": "EnabledWithCount"},
			"core": {"user_results": {"result": {"legacy": {"screen_name": "carol", "name": "Carol"}}}},
			"legacy": {
				"full_text": "Truncated preview... https://t.co/short1",
				"entities": {
					"urls": [{"url": "https://t.co/short1", "expanded_url": "https://blog.example.com/deep-dive"}]
				}
			},
			"note_tweet": {
				"note_tweet_results": {
					"result": {
						"text": "Full long note tweet with lots of detail and link https://t.co/short1",
						"entity_set": {
							"urls": [{"url": "https://t.co/short1", "expanded_url": "https://blog.example.com/deep-dive"}]
						}
					}
				}
			}
		}
	}`
	tweets := extractTweets(body)
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}
	tw := tweets[0]
	wantText := "Full long note tweet with lots of detail and link https://blog.example.com/deep-dive"
	if tw.Text != wantText {
		t.Errorf("Text = %q, want %q", tw.Text, wantText)
	}
	if len(tw.Links) != 1 || tw.Links[0] != "https://blog.example.com/deep-dive" {
		t.Errorf("Links = %+v, want [https://blog.example.com/deep-dive]", tw.Links)
	}
	if tw.ViewCount != 15800 {
		t.Errorf("ViewCount = %d, want 15800", tw.ViewCount)
	}
}

func TestExtractTweetsXArticle(t *testing.T) {
	body := `{
		"result": {
			"rest_id": "3001",
			"core": {"user_results": {"result": {"legacy": {"screen_name": "dave", "name": "Dave"}}}},
			"legacy": {
				"full_text": "https://t.co/art1",
				"entities": {
					"urls": [{"url": "https://t.co/art1", "expanded_url": "https://x.com/i/article/999"}]
				}
			},
			"article": {
				"article_results": {
					"result": {
						"title": "How Local-First Readers Win",
						"preview_text": "Short preview",
						"cover_media": {"media_info": {"original_img_url": "https://pbs.twimg.com/cover.jpg"}},
						"content_state": {
							"blocks": [
								{"type": "header-two", "text": "Introduction"},
								{"type": "unstyled", "text": "Local-first software keeps your data fast and private."}
							]
						}
					}
				}
			}
		}
	}`
	tweets := extractTweets(body)
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet, got %d", len(tweets))
	}
	tw := tweets[0]
	if !tw.IsArticle {
		t.Fatal("expected IsArticle = true")
	}
	if tw.ArticleTitle != "How Local-First Readers Win" {
		t.Errorf("ArticleTitle = %q", tw.ArticleTitle)
	}
	if tw.ArticleURL != "https://x.com/i/article/999" {
		t.Errorf("ArticleURL = %q", tw.ArticleURL)
	}
	if tw.ArticleText != "## Introduction\n\nLocal-first software keeps your data fast and private." {
		t.Errorf("ArticleText = %q", tw.ArticleText)
	}
	if len(tw.Media) != 1 || tw.Media[0] != "https://pbs.twimg.com/cover.jpg" {
		t.Errorf("Media = %+v", tw.Media)
	}
}

// cannedSender returns a fixed list of captured items for one request.
type cannedSender struct {
	req   *capture.BrowserRequest
	items []any
}

func (c *cannedSender) ResolveBrowserConnID(string) (string, error) { return "conn-1", nil }

func (c *cannedSender) SendMessage(_ context.Context, req *capture.BrowserRequest, _ string) (*capture.BrowserResponse, error) {
	c.req = req
	return &capture.BrowserResponse{
		Success: true,
		Result:  capture.PageResult{Items: c.items},
	}, nil
}

func TestTimelineSendsPatternsAndBodyBudget(t *testing.T) {
	cs := &cannedSender{items: []any{map[string]any{
		"url":  "https://x.com/i/api/graphql/abc/HomeTimeline",
		"body": sampleBody,
	}}}
	tweets, err := Timeline(context.Background(), cs, nil, "for_you", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tweets) != 2 {
		t.Fatalf("expected 2 tweets, got %d", len(tweets))
	}
	if cs.req == nil {
		t.Fatal("expected an intercept request")
	}
	patterns, ok := cs.req.Params["patterns"].([]string)
	if !ok || len(patterns) != 2 || patterns[0] != "HomeTimeline" || patterns[1] != "HomeLatestTimeline" {
		t.Errorf("patterns = %#v, want [HomeTimeline HomeLatestTimeline]", cs.req.Params["patterns"])
	}
	if got := cs.req.Params["maxBodyBytes"]; got != interceptMaxBodyBytes {
		t.Errorf("maxBodyBytes = %v, want %d", got, interceptMaxBodyBytes)
	}
}

func TestTruncatedMatchedBodyReportsBodyLimit(t *testing.T) {
	cs := &cannedSender{items: []any{map[string]any{
		"url":           "https://x.com/i/api/graphql/abc/HomeTimeline",
		"body":          `{"data":{"home":`,
		"bodyBytes":     400000,
		"bodyTruncated": true,
	}}}
	_, err := Timeline(context.Background(), cs, nil, "for_you", Options{})
	if err == nil {
		t.Fatal("expected an error for a truncated body")
	}
	if !strings.Contains(err.Error(), "truncated at") || !strings.Contains(err.Error(), "body budget") {
		t.Errorf("error should explain the body limit, got %q", err)
	}
}
