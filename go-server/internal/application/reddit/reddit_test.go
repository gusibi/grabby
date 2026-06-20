package reddit

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

// sampleThread mirrors Reddit's comments-page .json: a 2-element array where
// [0] is the post listing (one child) and [1] is the comments listing. Trimmed
// to the fields the parser reads.
const sampleThread = `[
  {
    "kind": "Listing",
    "data": {
      "children": [
        {
          "kind": "t3",
          "data": {
            "id": "abc123",
            "title": "Go 1.24 released",
            "author": "gopher",
            "subreddit": "golang",
            "selftext": "Discussion thread for the new release.",
            "url": "https://go.dev/blog/go1.24",
            "permalink": "/r/golang/comments/abc123/go_124_released/",
            "score": 542,
            "num_comments": 31,
            "created_utc": 1718000000.5
          }
        }
      ]
    }
  },
  {
    "kind": "Listing",
    "data": {
      "children": [
        {
          "kind": "t1",
          "data": {
            "id": "c1",
            "author": "alice",
            "body": "Finally generics are stable.",
            "score": 12,
            "created_utc": 1718000100.0,
            "replies": {
              "kind": "Listing",
              "data": {
                "children": [
                  {
                    "kind": "t1",
                    "data": {
                      "id": "c1a",
                      "author": "bob",
                      "body": "they have been since 1.18",
                      "score": 3,
                      "created_utc": 1718000200.0,
                      "replies": ""
                    }
                  }
                ]
              }
            }
          }
        },
        {
          "kind": "t1",
          "data": {
            "id": "c2",
            "author": "carol",
            "body": "Anyone tried the new range-over-func?",
            "score": 5,
            "created_utc": 1718000300.0,
            "replies": ""
          }
        },
        {
          "kind": "more",
          "data": { "count": 28, "children": ["c3", "c4"] }
        }
      ]
    }
  }
]`

func TestParseThread(t *testing.T) {
	thread, err := parseThread(sampleThread)
	if err != nil {
		t.Fatalf("parseThread: %v", err)
	}

	if thread.Post.ID != "abc123" {
		t.Errorf("post id: got %q want abc123", thread.Post.ID)
	}
	if thread.Post.Title != "Go 1.24 released" {
		t.Errorf("post title: got %q", thread.Post.Title)
	}
	if thread.Post.Author != "gopher" || thread.Post.Subreddit != "golang" {
		t.Errorf("post author/sub: got %q/%q", thread.Post.Author, thread.Post.Subreddit)
	}
	if thread.Post.Score != 542 || thread.Post.NumComments != 31 {
		t.Errorf("post counts: got %d/%d", thread.Post.Score, thread.Post.NumComments)
	}
	if thread.Post.CreatedUTC != 1718000000.5 {
		t.Errorf("post created_utc: got %v", thread.Post.CreatedUTC)
	}
	if thread.Post.URL != "https://www.reddit.com/r/golang/comments/abc123/go_124_released/" {
		t.Errorf("post permalink: got %q", thread.Post.URL)
	}
	if thread.Post.ContentURL != "https://go.dev/blog/go1.24" {
		t.Errorf("post content url: got %q", thread.Post.ContentURL)
	}

	if len(thread.Comments) != 2 {
		t.Fatalf("expected 2 top-level comments (more-placeholder skipped), got %d", len(thread.Comments))
	}
	first := thread.Comments[0]
	if first.ID != "c1" || first.Author != "alice" || first.Body != "Finally generics are stable." {
		t.Errorf("first comment: %+v", first)
	}
	if first.Score != 12 {
		t.Errorf("first score: got %d", first.Score)
	}
	if len(first.Replies) != 1 {
		t.Fatalf("expected 1 nested reply, got %d", len(first.Replies))
	}
	if first.Replies[0].ID != "c1a" || first.Replies[0].Author != "bob" {
		t.Errorf("nested reply: %+v", first.Replies[0])
	}
	if len(thread.Comments[1].Replies) != 0 {
		t.Errorf("second comment should have no replies, got %d", len(thread.Comments[1].Replies))
	}
}

func TestParseThreadBadJSON(t *testing.T) {
	if _, err := parseThread("not json"); err == nil {
		t.Error("expected error on non-json body")
	}
	if _, err := parseThread(`{"data":1}`); err == nil {
		t.Error("expected error on non-array body")
	}
}

func TestJsonURLFor(t *testing.T) {
	cases := map[string]string{
		"https://www.reddit.com/r/golang/comments/abc/go_124/":      "https://www.reddit.com/r/golang/comments/abc/go_124.json",
		"https://www.reddit.com/r/golang/comments/abc/go_124":       "https://www.reddit.com/r/golang/comments/abc/go_124.json",
		"https://old.reddit.com/r/x/comments/yy/title/?ref=foo":     "https://old.reddit.com/r/x/comments/yy/title.json?ref=foo",
	}
	for in, want := range cases {
		if got := jsonURLFor(in); got != want {
			t.Errorf("jsonURLFor(%q): got %q want %q", in, got, want)
		}
	}
}

// sampleListing mirrors a subreddit /new.json response: a single Listing with
// children posts and an `after` cursor.
const sampleListing = `{
  "kind": "Listing",
  "data": {
    "after": "t3_abc456",
    "children": [
      {
        "kind": "t3",
        "data": {
          "id": "p1",
          "title": "First post",
          "author": "alice",
          "subreddit": "golang",
          "selftext": "",
          "url": "https://example.com/article1",
          "permalink": "/r/golang/comments/p1/first_post/",
          "score": 100,
          "num_comments": 5,
          "created_utc": 1718000000.0
        }
      },
      {
        "kind": "t3",
        "data": {
          "id": "p2",
          "title": "Second post",
          "author": "bob",
          "subreddit": "golang",
          "selftext": "A self post body.",
          "url": "https://www.reddit.com/r/golang/comments/p2/second_post/",
          "permalink": "/r/golang/comments/p2/second_post/",
          "score": 42,
          "num_comments": 0,
          "created_utc": 1718000100.0
        }
      }
    ]
  }
}`

func TestParseListing(t *testing.T) {
	posts, after, err := parseListing(sampleListing)
	if err != nil {
		t.Fatalf("parseListing: %v", err)
	}
	if after != "t3_abc456" {
		t.Errorf("after: got %q want t3_abc456", after)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if posts[0].ID != "p1" || posts[0].Title != "First post" {
		t.Errorf("first post: %+v", posts[0])
	}
	if posts[0].ContentURL != "https://example.com/article1" {
		t.Errorf("first content_url: got %q", posts[0].ContentURL)
	}
	if posts[1].Body != "A self post body." {
		t.Errorf("second body: got %q", posts[1].Body)
	}
	if posts[1].URL != "https://www.reddit.com/r/golang/comments/p2/second_post/" {
		t.Errorf("second permalink: got %q", posts[1].URL)
	}
}

func TestParseListingExhausted(t *testing.T) {
	// No `after` => listing exhausted (after == "").
	body := `{"kind":"Listing","data":{"children":[]}}`
	posts, after, err := parseListing(body)
	if err != nil {
		t.Fatalf("parseListing: %v", err)
	}
	if len(posts) != 0 || after != "" {
		t.Errorf("expected empty + no cursor, got %d posts after=%q", len(posts), after)
	}
}

func TestParseListingBadJSON(t *testing.T) {
	if _, _, err := parseListing("not json"); err == nil {
		t.Error("expected error on non-json body")
	}
}

// searchURLFor builds the .json endpoint for a Reddit search, mirroring
// FetchSearch's URL construction. Kept as a pure helper so it can be tested
// without a browser.
func searchURLFor(query, subreddit, sort, after string) (pageURL, reqURL string) {
	q := make(url.Values)
	q.Set("q", query)
	q.Set("raw_json", "1")
	if sort != "" {
		q.Set("sort", sort)
	}
	if subreddit != "" {
		pageURL = "https://www.reddit.com/r/" + subreddit + "/search/?restrict_sr=1&" + q.Encode()
		reqURL = "https://www.reddit.com/r/" + subreddit + "/search.json?restrict_sr=1&" + q.Encode()
	} else {
		pageURL = "https://www.reddit.com/search/?" + q.Encode()
		reqURL = "https://www.reddit.com/search.json?" + q.Encode()
	}
	if after != "" {
		reqURL += "&after=" + after
	}
	return pageURL, reqURL
}

func TestSearchURLFor(t *testing.T) {
	// Site-wide search.
	page, req := searchURLFor("golang generics", "", "", "")
	if !strings.HasPrefix(req, "https://www.reddit.com/search.json?") {
		t.Errorf("site search reqURL: got %q", req)
	}
	if !strings.Contains(req, "q=golang+generics") && !strings.Contains(req, "q=golang") {
		t.Errorf("site search reqURL missing query: %q", req)
	}
	if !strings.HasPrefix(page, "https://www.reddit.com/search/?") {
		t.Errorf("site search pageURL: got %q", page)
	}

	// Subreddit-restricted search.
	page, req = searchURLFor("generics", "golang", "new", "")
	if !strings.Contains(req, "/r/golang/search.json") {
		t.Errorf("sub search reqURL: got %q", req)
	}
	if !strings.Contains(req, "restrict_sr=1") {
		t.Errorf("sub search missing restrict_sr: %q", req)
	}
	if !strings.Contains(req, "sort=new") {
		t.Errorf("sub search missing sort: %q", req)
	}
	if !strings.HasPrefix(page, "https://www.reddit.com/r/golang/search/") {
		t.Errorf("sub search pageURL: got %q", page)
	}

	// After cursor appended to reqURL only.
	_, req = searchURLFor("x", "", "", "t3_abc")
	if !strings.HasSuffix(req, "&after=t3_abc") {
		t.Errorf("after cursor: got %q", req)
	}
}

func TestFetchSearchEmptyQuery(t *testing.T) {
	if _, err := FetchSearch(context.Background(), nil, nil, "  ", "", "", "", Options{}); err == nil {
		t.Error("expected error on empty query")
	}
}
