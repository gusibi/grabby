package xiaohongshu

import (
	"encoding/json"
	"testing"
)

// noteFromDetailMapStr parses a noteDetailMap JSON string then extracts the note.
func noteFromDetailMapStr(body, noteID, noteURL string) (*Note, bool) {
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return nil, false
	}
	return noteFromDetailMap(m, noteID, noteURL)
}

// sampleDetailMap mirrors window.__INITIAL_STATE__.note.noteDetailMap (from a
// real note page), trimmed to the fields the parser reads.
const sampleDetailMap = `{
  "691345f70000000003010c2b": {
    "comments": [],
    "currentTime": 1762870775000,
    "note": {
      "noteId": "691345f70000000003010c2b",
      "title": "Java→Go无痛入门",
      "desc": "Java工程师转Go实战系列-01/07",
      "type": "normal",
      "xsecToken": "ABADwXUo_Kq0PtaJCdZ-m__SnWnwJYPATbaoVqN6UomrQ=",
      "user": {
        "nick_name": "执一｜AI与云计算",
        "user_id": "5f938394000000000100b307"
      },
      "interactInfo": {
        "likedCount": "23",
        "collectedCount": "13",
        "commentCount": "0",
        "shareCount": "1"
      },
      "imageList": [
        { "urlDefault": "http://xhscdn.com/img1.jpg" },
        { "urlDefault": "http://xhscdn.com/img2.jpg" }
      ]
    }
  }
}`

func TestNoteFromDetailMap(t *testing.T) {
	note, ok := noteFromDetailMapStr(sampleDetailMap, "691345f70000000003010c2b", "https://www.xiaohongshu.com/explore/691345f70000000003010c2b")
	if !ok {
		t.Fatal("expected note, got none")
	}
	if note.ID != "691345f70000000003010c2b" {
		t.Errorf("id: got %q", note.ID)
	}
	if note.Title != "Java→Go无痛入门" {
		t.Errorf("title: got %q", note.Title)
	}
	if note.Desc != "Java工程师转Go实战系列-01/07" {
		t.Errorf("desc: got %q", note.Desc)
	}
	if note.Author != "执一｜AI与云计算" || note.AuthorID != "5f938394000000000100b307" {
		t.Errorf("author: got %q/%q", note.Author, note.AuthorID)
	}
	if note.LikedCount != "23" || note.CollectedCount != "13" || note.CommentCount != "0" || note.ShareCount != "1" {
		t.Errorf("interact: got %+v", note)
	}
	if len(note.Images) != 2 || note.Images[0] != "http://xhscdn.com/img1.jpg" {
		t.Errorf("images: got %+v", note.Images)
	}
}

func TestDetailMapFromResultHandlesEncodedString(t *testing.T) {
	encoded, err := json.Marshal(sampleDetailMap)
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}
	detailMap, err := detailMapFromResult(nil, string(encoded))
	if err != nil {
		t.Fatalf("detailMapFromResult: %v", err)
	}
	note, ok := noteFromDetailMap(detailMap, "691345f70000000003010c2b", "https://www.xiaohongshu.com/explore/691345f70000000003010c2b")
	if !ok {
		t.Fatal("expected note, got none")
	}
	if note.Title != "Java→Go无痛入门" {
		t.Errorf("title: got %q", note.Title)
	}
}

func TestNoteFromDetailMapHandlesEncodedEntry(t *testing.T) {
	var detailMap map[string]any
	if err := json.Unmarshal([]byte(sampleDetailMap), &detailMap); err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}
	entry, err := json.Marshal(detailMap["691345f70000000003010c2b"])
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	detailMap["691345f70000000003010c2b"] = string(entry)

	note, ok := noteFromDetailMap(detailMap, "691345f70000000003010c2b", "https://www.xiaohongshu.com/explore/691345f70000000003010c2b")
	if !ok {
		t.Fatal("expected note, got none")
	}
	if note.ID != "691345f70000000003010c2b" {
		t.Errorf("id: got %q", note.ID)
	}
}

func TestNoteFromDetailMapMissingNote(t *testing.T) {
	if _, ok := noteFromDetailMapStr(sampleDetailMap, "nonexistent", ""); ok {
		t.Error("expected no note for unknown id, but single-entry fallback should not match wrong id")
	}
}

// sampleSearchBody mirrors /api/sns/web/v1/search/notes: data.items[].note_card.
const sampleSearchBody = `{
  "code": 0,
  "success": true,
  "msg": "成功",
  "data": {
    "has_more": true,
    "items": [
      {
        "id": "note1",
        "xsec_token": "tok1=",
        "model_type": "note",
        "note_card": {
          "type": "normal",
          "display_title": "First note",
          "user": { "nick_name": "alice", "user_id": "u1" },
          "interact_info": { "liked_count": "23", "collected_count": "13", "comment_count": "0", "shared_count": "1" }
        }
      },
      {
        "id": "note2",
        "xsec_token": "tok2=",
        "model_type": "note",
        "note_card": {
          "type": "video",
          "display_title": "Second note",
          "user": { "nickname": "bob", "user_id": "u2" },
          "interact_info": { "liked_count": "5" }
        }
      }
    ]
  }
}`

func TestExtractNotesFromSearch(t *testing.T) {
	notes := extractNotesFromListing(sampleSearchBody, "search/notes")
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	if notes[0].ID != "note1" || notes[0].Title != "First note" {
		t.Errorf("first: %+v", notes[0])
	}
	if notes[0].Author != "alice" || notes[0].AuthorID != "u1" {
		t.Errorf("first author: %+v", notes[0])
	}
	if notes[0].LikedCount != "23" {
		t.Errorf("first liked: %q", notes[0].LikedCount)
	}
	if !contains(notes[0].URL, "xsec_token=tok1") {
		t.Errorf("first url missing token: %q", notes[0].URL)
	}
	// second uses nickname variant
	if notes[1].Author != "bob" {
		t.Errorf("second author: %q", notes[1].Author)
	}
}

// sampleUserPostedBody mirrors /api/sns/web/v1/user_posted: data.notes[].
const sampleUserPostedBody = `{
  "data": {
    "cursor": "nextcursor",
    "has_more": true,
    "notes": [
      {
        "note_id": "un1",
        "xsec_token": "utok1=",
        "type": "normal",
        "display_title": "User note one",
        "user": { "nick_name": "alice", "user_id": "u1" },
        "interact_info": { "liked_count": "10", "comment_count": "2" }
      }
    ]
  }
}`

func TestExtractNotesFromUserPosted(t *testing.T) {
	notes := extractNotesFromListing(sampleUserPostedBody, "user_posted")
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].ID != "un1" || notes[0].Title != "User note one" {
		t.Errorf("note: %+v", notes[0])
	}
	if notes[0].LikedCount != "10" {
		t.Errorf("liked: %q", notes[0].LikedCount)
	}
}

func TestExtractUserPostedFallsBackToItems(t *testing.T) {
	notes := extractNotesFromListing(sampleSearchBody, "user_posted")
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	if notes[0].ID != "note1" || notes[0].Title != "First note" {
		t.Errorf("first: %+v", notes[0])
	}
}

const sampleUserInitialState = `{
  "notes": [[
    {
      "id": "6a365891000000001503f632",
      "noteCard": {
        "user": {
          "avatar": "https://sns-avatar-qc.xhscdn.com/avatar/1040g2jo320m4j38ils6g5pd3n1agshh79t0fluo",
          "userId": "65a3b8550000000003024627",
          "nickname": "赛博猫2077",
          "nickName": "赛博猫2077"
        },
        "interactInfo": {
          "liked": false,
          "likedCount": "20",
          "sticky": false
        },
        "cover": {
          "urlDefault": "http://sns-webpic-qc.xhscdn.com/cover1"
        },
        "noteId": "6a365891000000001503f632",
        "xsecToken": "ABYsBvJXq617Z3n-Fo7LdUfG0gdjVI4kBm8SmMHp66A5A=",
        "type": "video",
        "displayTitle": "Codex Skill 零基础教程，看这一条就够了！"
      },
      "index": 0,
      "ssrRendered": true,
      "xsecToken": "ABYsBvJXq617Z3n-Fo7LdUfG0gdjVI4kBm8SmMHp66A5A="
    }
  ]]
}`

func TestUserNotesFromInitialState(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal([]byte(sampleUserInitialState), &root); err != nil {
		t.Fatalf("unmarshal initial state: %v", err)
	}
	notes := userNotesFromInitialState(root)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	note := notes[0]
	if note.ID != "6a365891000000001503f632" {
		t.Errorf("id: got %q", note.ID)
	}
	if note.Title != "Codex Skill 零基础教程，看这一条就够了！" {
		t.Errorf("title: got %q", note.Title)
	}
	if note.Author != "赛博猫2077" || note.AuthorID != "65a3b8550000000003024627" {
		t.Errorf("author: got %q/%q", note.Author, note.AuthorID)
	}
	if note.LikedCount != "20" {
		t.Errorf("liked: got %q", note.LikedCount)
	}
	if note.XsecToken != "ABYsBvJXq617Z3n-Fo7LdUfG0gdjVI4kBm8SmMHp66A5A=" || !contains(note.URL, "xsec_token=ABYsBvJXq617Z3n-Fo7LdUfG0gdjVI4kBm8SmMHp66A5A") {
		t.Errorf("token/url: got %q %q", note.XsecToken, note.URL)
	}
	if len(note.Images) != 1 || note.Images[0] != "http://sns-webpic-qc.xhscdn.com/cover1" {
		t.Errorf("images: got %+v", note.Images)
	}
}

const sampleCommentPageBody = `{
  "data": {
    "has_more": false,
    "comments": [
      {
        "status": 0,
        "content": "MacBook 能用吗[偷笑R]",
        "sub_comments": [
          {
            "id": "6a2cd10100000000290362fa",
            "content": "可以的，mac  都可以",
            "liked": false,
            "user_info": {
              "image": "https://sns-avatar-qc.xhscdn.com/avatar/a.jpg",
              "user_id": "65fd1cd5000000000600c8c1",
              "nickname": "魔犁 AI"
            },
            "ip_location": "广东",
            "pictures": [],
            "note_id": "6a1a6a480000000007011dd2",
            "like_count": "0",
            "show_tags": ["is_author"],
            "create_time": 1781321986000
          }
        ],
        "note_id": "6a1a6a480000000007011dd2",
        "like_count": "0",
        "show_tags": [],
        "create_time": 1781193907000,
        "sub_comment_count": "5",
        "id": "6a2adcb3000000002b029634",
        "user_info": {
          "user_id": "6970ffd900000000190352f8",
          "nickname": "闲散产品人",
          "image": "https://sns-avatar-qc.xhscdn.com/avatar/b.jpg"
        },
        "ip_location": "四川",
        "pictures": []
      },
      {
        "id": "6a2920ae00000000270286b3",
        "note_id": "6a1a6a480000000007011dd2",
        "like_count": "1",
        "user_info": {
          "user_id": "55c2e0bb41a2b30962132f2e",
          "nickname": "文山湖赛艇队长",
          "image": "https://sns-avatar-qc.xhscdn.com/avatar/c.jpg"
        },
        "pictures": [
          {
            "url_pre": "http://sns-webpic-qc.xhscdn.com/comment/pre",
            "url_default": "http://sns-webpic-qc.xhscdn.com/comment/default"
          }
        ],
        "content": "真的麻了，这破遥控不显示名称的",
        "create_time": 1781080239000,
        "liked": true
      }
    ]
  }
}`

func TestExtractCommentsFromBody(t *testing.T) {
	comments := extractCommentsFromBody(sampleCommentPageBody)
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	first := comments[0]
	if first.ID != "6a2adcb3000000002b029634" || first.Content != "MacBook 能用吗[偷笑R]" {
		t.Errorf("first comment: %+v", first)
	}
	if first.Author != "闲散产品人" || first.AuthorID != "6970ffd900000000190352f8" {
		t.Errorf("first author: %+v", first)
	}
	if first.SubCommentCount != "5" || len(first.SubComments) != 1 {
		t.Fatalf("sub comments: %+v", first)
	}
	if first.SubComments[0].Author != "魔犁 AI" || first.SubComments[0].ShowTags[0] != "is_author" {
		t.Errorf("sub comment: %+v", first.SubComments[0])
	}
	second := comments[1]
	if !second.Liked || second.LikeCount != "1" || second.CreateTime != 1781080239000 {
		t.Errorf("second meta: %+v", second)
	}
	if len(second.Pictures) != 1 || second.Pictures[0] != "http://sns-webpic-qc.xhscdn.com/comment/default" {
		t.Errorf("pictures: %+v", second.Pictures)
	}
}

func TestNoteIDFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.xiaohongshu.com/explore/691345f70000000003010c2b?xsec_token=x": "691345f70000000003010c2b",
		"https://www.xiaohongshu.com/discovery/item/abc123":                         "abc123",
		"https://xhslink.com/xyz":                                                   "xyz",
	}
	for in, want := range cases {
		if got := noteIDFromURL(in); got != want {
			t.Errorf("noteIDFromURL(%q): got %q want %q", in, got, want)
		}
	}
}

func TestMatchesAny(t *testing.T) {
	if !matchesAny("https://edith.xiaohongshu.com/api/sns/web/v1/search/notes", []string{"/api/sns/web/v1/search/notes"}) {
		t.Error("should match search/notes")
	}
	if !matchesAny("https://edith.xiaohongshu.com/api/sns/web/v2/user/notes", []string{"/api/sns/web/"}) {
		t.Error("should match xiaohongshu web api")
	}
	if matchesAny("https://edith.xiaohongshu.com/api/sns/web/v1/system/config", []string{"/api/sns/web/v1/search/notes"}) {
		t.Error("should not match system/config")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
