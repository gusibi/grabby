package nativeui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

func (s *ChatSession) hasVisibleMessages() bool {
	if s == nil {
		return false
	}
	for _, m := range s.Messages {
		if m.Hidden {
			continue
		}
		if m.Role == "user" || strings.TrimSpace(m.Content) != "" {
			return true
		}
	}
	return false
}

// openReaderChat opens the AI dialog for the article in the reader. reset
// starts a fresh conversation with a full reading brief.
func (a *App) openReaderChat(reset bool) {
	it := a.reader.Item
	if it == nil || a.win == nil {
		return
	}
	if a.chat.ItemID != it.ID {
		a.chat.Draft = ""
	}
	a.chat.Open = true
	a.chat.ItemID = it.ID

	s := a.chats[it.ID]
	if s == nil || reset || (!s.Streaming && !s.hasVisibleMessages()) {
		if s != nil {
			s.seq++ // stop late deltas from the old conversation
			if s.cancel != nil {
				s.cancel()
			}
		}
		s = &ChatSession{}
		a.chats[it.ID] = s
		a.chat.Draft = ""
		a.sendReaderChat(it.ID, s, "请解读这篇文章", true)
	}
}

func (a *App) submitReaderChat() {
	it := a.reader.Item
	if it == nil {
		return
	}
	s := a.chats[it.ID]
	if s == nil || s.Streaming {
		return
	}
	text := strings.TrimSpace(a.chat.Draft)
	if text == "" {
		return
	}
	a.chat.Draft = ""
	a.sendReaderChat(it.ID, s, text, false)
}

func (a *App) sendReaderChat(itemID int64, s *ChatSession, text string, hidden bool) {
	if s == nil || s.Streaming || a.win == nil {
		return
	}
	if !hidden && len(s.Messages) == 1 && s.Messages[0].Hidden {
		s.Messages = nil
	}
	s.Messages = append(s.Messages, ChatMessage{Role: "user", Content: text, Hidden: hidden})
	s.Messages = append(s.Messages, ChatMessage{Role: "assistant"})
	a.startReaderChatStream(itemID, s)
}

func (a *App) retryReaderChat() {
	it := a.reader.Item
	if it == nil {
		return
	}
	s := a.chats[it.ID]
	if s == nil || s.Streaming {
		return
	}
	s.Err = ""
	if len(s.Messages) == 0 {
		a.sendReaderChat(it.ID, s, "请解读这篇文章", true)
		return
	}
	if last := len(s.Messages) - 1; s.Messages[last].Role == "assistant" {
		s.Messages[last] = ChatMessage{Role: "assistant"}
	} else {
		s.Messages = append(s.Messages, ChatMessage{Role: "assistant"})
	}
	a.startReaderChatStream(it.ID, s)
}

// startReaderChatStream sends the visible transcript and appends streamed
// deltas to the last assistant message. Updates are throttled so a fast model
// does not rebuild the dialog for every token.
func (a *App) startReaderChatStream(itemID int64, s *ChatSession) {
	s.Err = ""
	s.Streaming = true
	s.seq++
	seq := s.seq
	if s.cancel != nil {
		s.cancel()
	}

	history := make([]ChatMessage, 0, len(s.Messages))
	for _, m := range s.Messages {
		if m.Role == "assistant" && strings.TrimSpace(m.Content) == "" {
			continue
		}
		history = append(history, m)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	a.chat.ScrollNext = true
	go func() {
		defer cancel()
		var pending strings.Builder
		lastFlush := time.Now()

		flush := func(streamErr error, modelUsed string, final bool) {
			delta := pending.String()
			pending.Reset()
			a.win.Update(func() {
				if seq != s.seq {
					return
				}
				if delta != "" {
					for i := len(s.Messages) - 1; i >= 0; i-- {
						if s.Messages[i].Role == "assistant" {
							s.Messages[i].Content += delta
							break
						}
					}
				}
				if final {
					s.Streaming = false
					s.cancel = nil
					if modelUsed != "" {
						s.ModelUsed = modelUsed
					}
					if streamErr != nil && errors.Is(streamErr, context.Canceled) {
						return
					}
					if streamErr == nil {
						if n := len(s.Messages); n > 0 && s.Messages[n-1].Role == "assistant" && strings.TrimSpace(s.Messages[n-1].Content) == "" {
							streamErr = fmt.Errorf("模型没有返回内容")
						}
					}
					if streamErr != nil {
						s.Err = streamErr.Error()
						if n := len(s.Messages); n > 0 && s.Messages[n-1].Role == "assistant" && strings.TrimSpace(s.Messages[n-1].Content) == "" {
							s.Messages = s.Messages[:n-1]
						}
					}
				}
				if a.chat.Open && a.chat.ItemID == itemID {
					a.chat.ScrollNext = true
				}
			})
		}

		model, err := a.client.streamChat(ctx, itemID, history, func(delta string) {
			pending.WriteString(delta)
			if time.Since(lastFlush) >= 70*time.Millisecond {
				lastFlush = time.Now()
				flush(nil, "", false)
			}
		})
		flush(err, model, true)
	}()
}

func (a *App) readerChatDialog(c *ui.Context) {
	if !a.chat.Open || a.reader.Item == nil {
		return
	}
	it := *a.reader.Item
	s := a.chats[it.ID]
	if s == nil {
		s = &ChatSession{}
		a.chats[it.ID] = s
	}
	t := c.Theme()
	winW, winH := c.Size()
	w := float32(760)
	if winW-120 < w {
		w = winW - 120
	}
	if w < 380 {
		w = 380
	}
	h := float32(700)
	if winH-120 < h {
		h = winH - 120
	}
	if h < 420 {
		h = 420
	}

	ui.DialogBase(c, &a.chat.Open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.Hex("#000000").Alpha(0.48))
		panel.Width(w).Height(h).Radius(16).Clip().Background(t.Background).
			Border(1, t.Border).Shadow(0, 24, 70, -18, ui.Hex("#000000").Alpha(0.55))
		ui.Column(c).Fill().Children(func() {
			a.readerChatHeader(c, it, s)
			ui.Divider(c)
			a.readerChatTranscript(c, s)
			ui.Divider(c)
			a.readerChatComposer(c, s)
		})
	})
}

func (a *App) readerChatHeader(c *ui.Context, it Item, s *ChatSession) {
	t := c.Theme()
	ui.Row(c).Padding(16, 20, 14, 20).Gap(10).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Box(c).Size(26, 26).Radius(8).Background(t.Accent.Alpha(0.12)).Center().Children(func() {
			ui.Icon(c, iconSparkle).Size(14, 14).TextColor(t.Accent)
		})
		ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, "AI 解读").FontSize(15.5).FontWeight(650).LetterSpacing(-0.2)
			ui.Text(c, truncate(orDefault(it.Title, "未命名内容"), 52)).FontSize(11.5).TextColor(textSoft(c)).MaxLines(1)
		})
		if s.ModelUsed != "" {
			softChip(c, truncate(displayModel(s.ModelUsed), 26))
		}
		if iconButton(c, iconRefresh, "重新解读").Clicked() {
			a.openReaderChat(true)
		}
		if iconButton(c, iconClose, "关闭").Clicked() {
			a.chat.Open = false
		}
	})
}

func (a *App) readerChatTranscript(c *ui.Context, s *ChatSession) {
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Grow(1).Padding(20, 22, 18, 22).Gap(18).Children(func() {
			visible := 0
			for i, m := range s.Messages {
				if m.Hidden {
					continue
				}
				visible++
				last := i == len(s.Messages)-1
				readerChatMessage(c, m, s.Streaming && last)
			}
			if visible == 0 && !s.Streaming {
				hint(c, "点「重新解读」让 AI 从这篇文章开始分析。")
			}
			if s.Err != "" {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, s.Err).FontSize(12).TextColor(c.Theme().Danger).Grow(1).MaxLines(3)
					if quietButton(c, "重试").FontSize(11.5).Clicked() {
						a.retryReaderChat()
					}
				})
			}
			if a.chat.ScrollNext {
				a.chat.ScrollNext = false
				ui.Box(c).Size(1, 1).ScrollIntoView(ui.ScrollIntoViewOptions{Block: ui.ScrollEnd})
			}
		})
	})
}

func readerChatMessage(c *ui.Context, m ChatMessage, streaming bool) {
	t := c.Theme()
	if m.Role == "user" {
		ui.Row(c).Justify(ui.End).Children(func() {
			ui.Column(c).MaxWidth(540).Padding(9, 13).Radius(13).Background(t.Accent.Alpha(0.1)).Children(func() {
				ui.Text(c, m.Content).FontSize(13).LineHeight(1.55)
			})
		})
		return
	}

	ui.Column(c).Gap(7).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconSparkle).Size(12, 12).TextColor(t.Accent)
			ui.Text(c, "Grabby AI").FontSize(10.5).FontWeight(650).TextColor(textSoft(c)).LetterSpacing(0.2)
		})
		if strings.TrimSpace(m.Content) == "" && streaming {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在阅读这篇文章…").FontSize(12).TextColor(textSoft(c))
			})
			return
		}
		if strings.TrimSpace(m.Content) != "" {
			markdown.View(c, m.Content, nil)
		}
		if streaming {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在生成…").FontSize(11).TextColor(textFaint(c))
			})
		}
	})
}

func (a *App) readerChatComposer(c *ui.Context, s *ChatSession) {
	it := a.reader.Item
	if it == nil {
		return
	}
	ui.Column(c).Padding(14, 18, 16, 18).Gap(7).Shrink(0).Children(func() {
		send := false
		ui.Row(c).Gap(10).AlignItems(ui.End).Children(func() {
			ta := ui.TextArea(c, &a.chat.Draft).Height(62).Grow(1).Placeholder("继续追问这篇文章，例如：这个结论有哪些证据？")
			ta.OnTextKey(func(k ui.TextKey) bool {
				if k.Key != ui.KeyEnter || k.Mods != 0 {
					return false
				}
				text := strings.TrimSpace(k.Text())
				if text == "" || s.Streaming {
					return true
				}
				k.SetText("")
				a.chat.Draft = ""
				a.sendReaderChat(it.ID, s, text, false)
				return true
			})
			disabled := s.Streaming || strings.TrimSpace(a.chat.Draft) == ""
			if primaryButton(c, "发送", disabled).Clicked() && !disabled {
				send = true
			}
		})
		hint(c, "Enter 发送 · Shift+Enter 换行 · AI 只基于当前文章回答")
		if send {
			a.submitReaderChat()
		}
	})
}
