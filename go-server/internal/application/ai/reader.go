package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	genai "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"go.uber.org/zap"

	domainai "go-server/internal/domain/ai"
	"go-server/internal/infrastructure/sqlite"
)

// TranslationResponseSchema constrains the translation output for providers
// that support JSON-schema response formats (LM Studio, OpenAI-compatible).
const TranslationResponseSchema = `{
	"type": "object",
	"properties": {
		"translations": {
			"type": "array",
			"items": {"type": "string"}
		}
	},
	"required": ["translations"]
}`

// InterpretationResponseSchema is the structured reader brief.
const InterpretationResponseSchema = `{
	"type": "object",
	"properties": {
		"summary": {"type": "string"},
		"key_points": {"type": "array", "items": {"type": "string"}},
		"insights": {"type": "array", "items": {"type": "string"}},
		"background": {"type": "string"},
		"reading_tips": {"type": "string"}
	},
	"required": ["summary", "key_points", "insights"]
}`

// TranslationResult is the aligned translation of a reader document. The
// caller sends the title plus one entry per body block and gets the same shape
// back, so a paragraph always maps to its translation.
type TranslationResult struct {
	Title        string   `json:"title"`
	Translations []string `json:"translations"`
	ModelUsed    string   `json:"model_used"`
}

// ReaderInterpretation is the AI brief shown above an article.
type ReaderInterpretation struct {
	Summary     string   `json:"summary"`
	KeyPoints   []string `json:"key_points"`
	Insights    []string `json:"insights"`
	Background  string   `json:"background"`
	ReadingTips string   `json:"reading_tips"`
	ModelUsed   string   `json:"model_used"`
}

// Translation limits. A chunk stays small enough for one request while a long
// article still translates every paragraph, sequentially, within the request
// timeout the desktop client allows.
const (
	translationMaxBlocks = 400
	translationChunkSize = 4800 // runes per request
	translationChunkRows = 8    // blocks per request
	interpretContentMax  = 8000 // runes of body sent to the interpreter
)

// TranslateText translates a reader document into target. Empty values follow
// the configured translation model, then the active analysis profile.
func (e *AIEngine) TranslateText(ctx context.Context, title string, blocks []string, target string) (*TranslationResult, error) {
	settings := e.Settings()
	target = strings.TrimSpace(target)
	if target == "" {
		target = settings.TranslationTargetLanguage
	}
	if target == "" {
		target = "简体中文"
	}
	if len(blocks) > translationMaxBlocks {
		blocks = blocks[:translationMaxBlocks]
	}

	// The title rides at index 0 so one translation pass can cover it too.
	texts := make([]string, 0, len(blocks)+1)
	if strings.TrimSpace(title) != "" {
		texts = append(texts, strings.TrimSpace(title))
	}
	texts = append(texts, blocks...)
	if len(texts) == 0 {
		return nil, fmt.Errorf("没有可翻译的正文")
	}

	translations := make([]string, len(texts))
	modelUsed := ""
	for _, chunk := range translationChunks(texts) {
		prompt := buildTranslationPrompt(target, chunk.texts)
		var (
			raw    string
			model  string
			parsed []string
			err    error
		)
		for attempt := 0; attempt < 2; attempt++ {
			if attempt > 0 {
				// One retry smooths over transient rate limits and truncated
				// responses without turning a whole long article into a failure.
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(translationRetryDelay):
				}
			}
			raw, model, err = e.generateReaderTask(
				ctx,
				settings.TranslationProfileID,
				settings.TranslationModel,
				prompt,
				"translate",
				TranslationResponseSchema,
			)
			if err != nil {
				continue
			}
			parsed, err = parseTranslationArray(raw, len(chunk.texts))
			if err == nil && !allStringsEmpty(parsed) {
				break
			}
			if err == nil {
				err = fmt.Errorf("模型返回空译文")
			}
		}
		if err != nil {
			if raw == "" {
				return nil, fmt.Errorf("翻译失败: %w", err)
			}
			return nil, fmt.Errorf("翻译结果格式不正确: %w", err)
		}
		modelUsed = model
		for i, idx := range chunk.indices {
			translations[idx] = parsed[i]
		}
	}

	result := &TranslationResult{ModelUsed: modelUsed}
	if strings.TrimSpace(title) != "" {
		result.Title = translations[0]
		result.Translations = translations[1:]
	} else {
		result.Translations = translations
	}
	if allStringsEmpty(result.Translations) && strings.TrimSpace(result.Title) == "" {
		return nil, fmt.Errorf("模型没有返回译文，请检查翻译模型设置")
	}
	return result, nil
}

func allStringsEmpty(list []string) bool {
	for _, s := range list {
		if strings.TrimSpace(s) != "" {
			return false
		}
	}
	return true
}

// InterpretItem asks the configured interpretation model for a structured
// brief on one archived item.
func (e *AIEngine) InterpretItem(ctx context.Context, itemID int64) (*ReaderInterpretation, error) {
	scraped, err := e.db.GetScrapedItem(itemID)
	if err != nil {
		return nil, fmt.Errorf("读取内容失败: %w", err)
	}
	if scraped == nil {
		return nil, fmt.Errorf("内容不存在: %d", itemID)
	}

	content := strings.TrimSpace(scraped.Content)
	if content == "" {
		content = strings.TrimSpace(scraped.Summary)
	}
	content = truncateRunes(content, interpretContentMax)
	if content == "" {
		return nil, fmt.Errorf("这篇内容还没有可解读的正文")
	}

	settings := e.Settings()
	prompt := buildInterpretationPrompt(scraped.Title, scraped.OriginSource, scraped.Summary, content)
	raw, model, err := e.generateReaderTask(
		ctx,
		settings.InterpretationProfileID,
		settings.InterpretationModel,
		prompt,
		"interpret",
		InterpretationResponseSchema,
	)
	if err != nil {
		return nil, fmt.Errorf("AI 解读失败: %w", err)
	}

	var result ReaderInterpretation
	if err := json.Unmarshal([]byte(trimToJSON(raw)), &result); err != nil {
		return nil, fmt.Errorf("解读结果格式不正确: %w（原始响应: %.200s）", err, raw)
	}
	result.ModelUsed = model
	if strings.TrimSpace(result.Summary) == "" && len(result.KeyPoints) == 0 {
		return nil, fmt.Errorf("模型没有返回有效的解读内容")
	}
	return &result, nil
}

// translationChunk is one request's worth of aligned source entries.
type translationChunk struct {
	indices []int
	texts   []string
}

// translationChunks groups entries by rune count and row count. Empty entries
// stay aligned but are not sent to the model.
func translationChunks(texts []string) []translationChunk {
	var chunks []translationChunk
	var cur translationChunk
	curRunes := 0
	flush := func() {
		if len(cur.indices) > 0 {
			chunks = append(chunks, cur)
			cur = translationChunk{}
			curRunes = 0
		}
	}
	for i, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		n := len([]rune(text))
		if len(cur.indices) > 0 && (curRunes+n > translationChunkSize || len(cur.indices) >= translationChunkRows) {
			flush()
		}
		cur.indices = append(cur.indices, i)
		cur.texts = append(cur.texts, text)
		curRunes += n
	}
	flush()
	return chunks
}

func buildTranslationPrompt(target string, texts []string) string {
	payload, _ := json.Marshal(texts)
	return fmt.Sprintf(`你是专业的翻译引擎。请把下面的 JSON 数组里的每一段文本翻译成%s。

要求：
1. 保持原意、语气、术语和数字准确，不增删内容，不做解释或总结。
2. 数组中的每一项是一段独立文本，必须逐项翻译；标题也在其中，按标题的语气翻译。
3. 只返回 JSON 对象，不要 Markdown 代码块，不要任何额外文字。
4. 输入共 %d 项，返回格式必须严格为：{"translations":["第一段译文","第二段译文", ...]}，数组长度（%d 项）和顺序必须与输入完全一致，严禁合并或拆分段落。

输入：
%s`, target, len(texts), len(texts), string(payload))
}

func buildInterpretationPrompt(title, source, summary, content string) string {
	return `你是资深阅读编辑。请阅读下面这篇文章，为中文读者写一份结构化解读。

【标题】：` + title + `
【来源】：` + source + `
【摘要】：` + summary + `
【正文】：
` + content + `

请只返回 JSON 对象，不要 Markdown 代码块，不要额外文字。字段要求：
1. summary：一句话说清文章的核心结论（不超过 60 字）。
2. key_points：3-5 条关键信息，每条一句话，具体、可核查。
3. insights：2-4 条值得注意的判断、影响或反常识之处，说明"为什么重要"。
4. background：必要的背景补充（1-2 句），没有则留空字符串。
5. reading_tips：给读者的阅读建议或值得追问的问题（1-2 句），没有则留空字符串。`
}

// generateReaderTask calls the configured (profile, model) for a reader task
// and falls back to the regular analysis route when it is unavailable.
func (e *AIEngine) generateReaderTask(ctx context.Context, profileID, model, prompt, task, schemaJSON string) (string, string, error) {
	schema := json.RawMessage(schemaJSON)
	tried := map[string]bool{}
	var lastErr error

	if pc, m, ok := e.taskClient(profileID, model); ok {
		tried[pc.profile.ID+"|"+m] = true
		raw, err := e.callProfileWithUsage(ctx, pc, m, prompt, task, &schema)
		if err == nil {
			return raw, m, nil
		}
		lastErr = err
		e.logger.Warn("Configured reader model failed, trying analysis route",
			zap.String("task", task), zap.String("model", m), zap.Error(err))
	}

	e.mu.RLock()
	selector := e.selector
	e.mu.RUnlock()
	if selector != nil {
		for i := 0; i < selector.EnabledCount(); i++ {
			sel := selector.Next()
			if sel == nil {
				break
			}
			key := sel.Profile.ID + "|" + sel.Model
			if tried[key] {
				continue
			}
			pc, ok := e.clientForProfile(sel.Profile)
			if !ok {
				continue
			}
			raw, err := e.callProfileWithUsage(ctx, pc, sel.Model, prompt, task, &schema)
			if err == nil {
				selector.MarkHealthy(sel.Key())
				return raw, sel.Model, nil
			}
			lastErr = err
			e.logger.Warn("Reader fallback model failed",
				zap.String("task", task), zap.String("model", sel.Model), zap.Error(err))
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的 AI 模型，请先在设置中配置模型")
	}
	return "", "", lastErr
}

// taskClient resolves the explicitly configured model, else the active
// profile's primary model. It can build a client on demand so translation works
// even when automatic analysis is switched off.
func (e *AIEngine) taskClient(profileID, model string) (*profileClient, string, bool) {
	e.mu.RLock()
	settings := e.settings
	e.mu.RUnlock()

	profileID = strings.TrimSpace(profileID)
	model = strings.TrimSpace(model)

	var chosen *domainai.AIProviderProfile
	for i := range settings.Profiles {
		p := &settings.Profiles[i]
		if p.Disabled {
			continue
		}
		if profileID != "" && p.ID == profileID {
			chosen = p
			break
		}
	}
	if chosen == nil && profileID != "" {
		// If the dedicated profile was deleted or disabled, fall back to the
		// active profile rather than failing outright.
		model = ""
	}
	if chosen == nil {
		for i := range settings.Profiles {
			p := &settings.Profiles[i]
			if p.Disabled {
				continue
			}
			if p.ID == settings.ActiveProfileID {
				chosen = p
				break
			}
		}
	}
	if chosen == nil {
		for i := range settings.Profiles {
			p := &settings.Profiles[i]
			if !p.Disabled {
				chosen = p
				break
			}
		}
	}
	if chosen == nil {
		return nil, "", false
	}

	if model == "" {
		model = chosen.Model
	}
	if model == "" {
		if models := chosen.EffectiveModels(); len(models) > 0 {
			model = models[0]
		}
	}
	if model == "" {
		return nil, "", false
	}

	pc, ok := e.clientForProfile(*chosen)
	if !ok {
		return nil, "", false
	}
	return pc, model, true
}

func (e *AIEngine) clientForProfile(chosen domainai.AIProviderProfile) (*profileClient, bool) {
	e.mu.RLock()
	pc := e.clients[chosen.ID]
	e.mu.RUnlock()
	if pc != nil {
		return pc, true
	}
	built, err := e.buildClient(chosen)
	if err != nil {
		e.logger.Warn("Failed to build reader client",
			zap.String("profile", chosen.Name), zap.Error(err))
		return nil, false
	}
	// Cache it so a translation with several chunks does not rebuild a
	// Genkit client per request when automatic analysis is switched off.
	e.mu.Lock()
	if existing := e.clients[chosen.ID]; existing != nil {
		pc = existing
	} else {
		e.clients[chosen.ID] = built
		pc = built
	}
	e.mu.Unlock()
	return pc, true
}

// parseTranslationArray accepts the shapes models actually return: an object
// with "translations", a bare array, or an index-keyed object.
func parseTranslationArray(raw string, want int) ([]string, error) {
	body := trimToJSON(raw)
	if body == "" {
		return nil, fmt.Errorf("模型返回为空")
	}

	var obj struct {
		Translations []string `json:"translations"`
		Items        []string `json:"items"`
		Result       []string `json:"result"`
	}
	if err := json.Unmarshal([]byte(body), &obj); err == nil {
		for _, list := range [][]string{obj.Translations, obj.Items, obj.Result} {
			if len(list) > 0 {
				return fitTranslationLength(list, want), nil
			}
		}
	}

	var wrapped struct {
		Translations []struct {
			Translation string `json:"translation"`
			Text        string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal([]byte(body), &wrapped); err == nil && len(wrapped.Translations) > 0 {
		out := make([]string, 0, len(wrapped.Translations))
		for _, o := range wrapped.Translations {
			if o.Translation != "" {
				out = append(out, o.Translation)
			} else {
				out = append(out, o.Text)
			}
		}
		return fitTranslationLength(out, want), nil
	}

	var arr []string
	if err := json.Unmarshal([]byte(body), &arr); err == nil && len(arr) > 0 {
		return fitTranslationLength(arr, want), nil
	}

	// Some models wrap each entry: [{"translation":"..."}] or {"text":"..."}.
	var objects []struct {
		Translation string `json:"translation"`
		Text        string `json:"text"`
	}
	if err := json.Unmarshal([]byte(body), &objects); err == nil && len(objects) > 0 {
		out := make([]string, 0, len(objects))
		for _, o := range objects {
			if o.Translation != "" {
				out = append(out, o.Translation)
			} else {
				out = append(out, o.Text)
			}
		}
		return fitTranslationLength(out, want), nil
	}

	var indexed map[string]string
	if err := json.Unmarshal([]byte(body), &indexed); err == nil && len(indexed) > 0 {
		out := make([]string, want)
		matched := 0
		for key, value := range indexed {
			if idx, err := strconv.Atoi(strings.TrimSpace(key)); err == nil && idx >= 0 && idx < want {
				out[idx] = value
				matched++
			}
		}
		if matched > 0 {
			return out, nil
		}
	}

	return nil, fmt.Errorf("没有找到 translations 数组")
}

func fitTranslationLength(list []string, want int) []string {
	out := make([]string, want)
	copy(out, list)
	return out
}

// trimToJSON strips fences and prose around a JSON object or array.
func trimToJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if idx := strings.Index(s, "```"); idx >= 0 {
		if end := strings.Index(s[idx+3:], "```"); end >= 0 {
			s = s[idx+3 : idx+3+end]
		} else {
			s = s[idx+3:]
		}
		s = strings.TrimPrefix(s, "json")
		s = strings.TrimSpace(s)
	}
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return s
	}
	if s[start] == '{' {
		if end := strings.LastIndex(s, "}"); end > start {
			return strings.TrimSpace(s[start : end+1])
		}
		return strings.TrimSpace(s[start:])
	}
	if end := strings.LastIndex(s, "]"); end > start {
		return strings.TrimSpace(s[start : end+1])
	}
	return strings.TrimSpace(s[start:])
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// Wait between translation retries is short: a transient 429 or timeout often
// clears while a long article still finishes inside the client timeout.
var translationRetryDelay = time.Second

// ChatMessage is one turn of the reader conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatStream answers a question about one archived article, streaming the
// assistant text as it arrives. The first message is usually the reader's
// "解读这篇文章" prompt; later messages are follow-up questions and the
// transcript keeps the conversation coherent.
func (e *AIEngine) ChatStream(ctx context.Context, itemID int64, messages []ChatMessage, onDelta func(string) error) (string, error) {
	scraped, err := e.db.GetScrapedItem(itemID)
	if err != nil {
		return "", fmt.Errorf("读取内容失败: %w", err)
	}
	if scraped == nil {
		return "", fmt.Errorf("内容不存在: %d", itemID)
	}
	if strings.TrimSpace(scraped.Content) == "" && strings.TrimSpace(scraped.Summary) == "" {
		return "", fmt.Errorf("这篇内容还没有可解读的正文")
	}

	settings := e.Settings()
	pc, model, ok := e.taskClient(settings.InterpretationProfileID, settings.InterpretationModel)
	if !ok {
		e.mu.RLock()
		selector := e.selector
		e.mu.RUnlock()
		if selector == nil {
			return "", fmt.Errorf("没有可用的 AI 模型，请先在设置中配置模型")
		}
		sel := selector.Next()
		if sel == nil {
			return "", fmt.Errorf("没有可用的 AI 模型，请先在设置中配置模型")
		}
		pc, _ = e.clientForProfile(sel.Profile)
		model = sel.Model
		if pc == nil {
			return "", fmt.Errorf("AI 模型客户端未就绪")
		}
	}

	prompt := buildReaderChatPrompt(scraped.Title, scraped.OriginSource, scraped.Summary, scraped.Content, messages)
	start := time.Now()
	pc.rateLimiter.Wait()

	modelRef := domainai.DriverModelRef(pc.profile.Provider, model)
	var answer strings.Builder

	if pc.lmstudio != nil {
		// LM Studio is called through the direct client; it has no streaming
		// path here, so the text arrives in one piece.
		out, genErr := pc.lmstudio.GenerateWithSchemaModel(ctx, modelRef, prompt, nil)
		if genErr == nil && strings.TrimSpace(out) != "" {
			answer.WriteString(out)
			if err := onDelta(out); err != nil {
				return "", err
			}
		} else if genErr == nil {
			genErr = fmt.Errorf("模型没有返回内容")
		}
		e.recordReaderUsage(pc, model, "interpret", prompt, answer.String(), start, genErr)
		if genErr != nil {
			return "", genErr
		}
		return model, nil
	}

	var genErr error
	for result, err := range genkit.GenerateStream(ctx, pc.genkit,
		genai.WithModelName(modelRef),
		genai.WithPrompt(prompt),
	) {
		if err != nil {
			genErr = err
			break
		}
		if result.Done {
			if answer.Len() == 0 && result.Response != nil {
				final := result.Response.Text()
				if final != "" {
					answer.WriteString(final)
					if cbErr := onDelta(final); cbErr != nil {
						return "", cbErr
					}
				}
			}
			break
		}
		if result.Chunk == nil {
			continue
		}
		delta := result.Chunk.Text()
		if delta == "" {
			continue
		}
		answer.WriteString(delta)
		if cbErr := onDelta(delta); cbErr != nil {
			return "", cbErr
		}
	}
	if genErr != nil && answer.Len() == 0 && ctx.Err() == nil {
		// Some OpenAI-compatible endpoints reject streaming. Fall back to one
		// non-streamed answer so the user still gets the analysis.
		resp, retryErr := genkit.Generate(ctx, pc.genkit,
			genai.WithModelName(modelRef),
			genai.WithPrompt(prompt),
		)
		if retryErr == nil && resp != nil {
			final := resp.Text()
			if final != "" {
				answer.WriteString(final)
				if cbErr := onDelta(final); cbErr != nil {
					return "", cbErr
				}
				genErr = nil
			}
		} else if retryErr != nil {
			genErr = retryErr
		}
	}
	e.recordReaderUsage(pc, model, "interpret", prompt, answer.String(), start, genErr)
	if genErr != nil {
		return "", genErr
	}
	if strings.TrimSpace(answer.String()) == "" {
		return "", fmt.Errorf("模型没有返回内容")
	}
	return model, nil
}

func buildReaderChatPrompt(title, source, summary, content string, messages []ChatMessage) string {
	summary = strings.TrimSpace(summary)
	content = strings.TrimSpace(content)
	if content == "" {
		content = summary
	}
	content = truncateRunes(content, 12000)
	summary = truncateRunes(summary, 1000)

	var b strings.Builder
	b.WriteString("你是一位资深阅读编辑，正在和用户讨论下面这篇文章。\n\n")
	b.WriteString("【标题】：" + title + "\n")
	b.WriteString("【来源】：" + source + "\n")
	b.WriteString("【摘要】：" + summary + "\n")
	b.WriteString("【正文】：\n" + content + "\n\n")
	b.WriteString(`回答要求：
1. 用中文回答，使用 Markdown 排版；小标题用 ###，列表用短句。
2. 用户第一次让你解读时，按「核心结论」「关键要点」「值得注意」「背景与延伸」组织，不要写成 JSON。
3. 只基于上面的文章内容回答，不编造事实；文章没有提到的，明确说"文章未提及"。
4. 后续追问要直接回答问题，必要时引用文章里的具体数字、人物或结论。
5. 保持简洁，避免空话和重复原文。

对话记录：
`)
	// Keep the last turns and a bounded transcript so a long conversation does
	// not push the article out of the model's context.
	start := 0
	if len(messages) > 12 {
		start = len(messages) - 12
	}
	turns := 0
	for _, m := range messages[start:] {
		role := "用户"
		if strings.EqualFold(m.Role, "assistant") {
			role = "助手"
		}
		text := truncateRunes(strings.TrimSpace(m.Content), 2000)
		if text == "" {
			continue
		}
		b.WriteString(role + "：" + text + "\n")
		turns++
	}
	if turns == 0 {
		b.WriteString("用户：请解读这篇文章\n")
	}
	b.WriteString("\n助手：")
	return b.String()
}

func (e *AIEngine) recordReaderUsage(pc *profileClient, model, task, prompt, output string, start time.Time, genErr error) {
	status, errMessage := "success", ""
	if genErr != nil {
		status = "failed"
		errMessage = truncateString(genErr.Error(), 500)
	}
	usage := sqlite.AIUsageRecord{
		ConnectionID:     pc.profile.ID,
		ModelID:          stripModelNamespace(model),
		Task:             task,
		PromptTokens:     estimateTokens(prompt),
		CompletionTokens: estimateTokens(output),
		TotalTokens:      estimateTokens(prompt) + estimateTokens(output),
		LatencyMS:        time.Since(start).Milliseconds(),
		Status:           status,
		Error:            errMessage,
	}
	if err := e.db.RecordAIUsage(usage); err != nil {
		e.logger.Warn("Failed to record AI usage", zap.Error(err))
	}
}
