package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	genai "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	oai "github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"go.uber.org/zap"

	domainai "go-server/internal/domain/ai"
	"go-server/internal/domain/item"
	"go-server/internal/infrastructure/llm"
	"go-server/internal/infrastructure/sqlite"
)

// AIAnalysisResult is the structured output format requested from LLM.
type AIAnalysisResult struct {
	Category     string   `json:"category"`
	Subcategory  string   `json:"subcategory"`
	QualityScore int      `json:"quality_score"`
	Summary      string   `json:"summary"`
	Comment      string   `json:"comment"`
	Tags         []string `json:"tags"`
}

// profileClient holds an initialized client for a single profile.
type profileClient struct {
	profile     domainai.AIProviderProfile
	genkit      *genkit.Genkit      // non-nil for gemini/openai/custom
	lmstudio    *llm.LMStudioClient // non-nil for lmstudio
	rateLimiter *llm.RateLimiter    // per-profile rate limiter for all providers
}

// AIEngine handles queueing and executing AI analysis requests.
type AIEngine struct {
	mu            sync.RWMutex
	settings      domainai.AISettings
	db            *sqlite.Database
	logger        *zap.Logger
	selector      *ProfileSelector
	clients       map[string]*profileClient // keyed by profile ID
	queue         chan int64
	queueMu       sync.Mutex
	queuedItems   map[int64]struct{}
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	activeWorkers bool
}

// NewAIEngine initializes and returns a new AI processing engine.
func NewAIEngine(settings domainai.AISettings, db *sqlite.Database, logger *zap.Logger) (*AIEngine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	engine := &AIEngine{
		settings:    settings,
		db:          db,
		logger:      logger,
		clients:     make(map[string]*profileClient),
		queue:       make(chan int64, 500),
		queuedItems: make(map[int64]struct{}),
		ctx:         ctx,
		cancel:      cancel,
	}

	selector := NewProfileSelector(settings)
	engine.selector = selector

	if settings.Enabled {
		if err := engine.initClients(settings); err != nil {
			cancel()
			return nil, err
		}
	}

	return engine, nil
}

// initClients builds profileClient entries for every enabled profile.
func (e *AIEngine) initClients(settings domainai.AISettings) error {
	for _, p := range settings.Profiles {
		if p.Disabled {
			continue
		}
		pc, err := e.buildClient(p)
		if err != nil {
			e.logger.Warn("Failed to init client for profile, skipping",
				zap.String("profile", p.Name), zap.Error(err))
			continue
		}
		e.clients[p.ID] = pc
	}
	if len(e.clients) == 0 && len(settings.Profiles) > 0 {
		return fmt.Errorf("no enabled profile could be initialized")
	}
	return nil
}

// buildClient creates a profileClient for a single profile.
func (e *AIEngine) buildClient(p domainai.AIProviderProfile) (*profileClient, error) {
	rpm := p.RequestsPerMinute
	if rpm <= 0 {
		rpm = 10
	}
	pc := &profileClient{
		profile:     p,
		rateLimiter: llm.NewRateLimiter(rpm),
	}
	if strings.ToLower(p.Provider) == "lmstudio" {
		// The model is passed per request (a profile can hold several models),
		// so the client itself stays model-agnostic.
		pc.lmstudio = llm.NewLMStudioClient(p.BaseURL, "", e.logger)
		e.logger.Info("Initialized LM Studio client",
			zap.String("profile", p.Name),
			zap.String("base_url", p.BaseURL),
			zap.Int("models", len(p.EffectiveModels())),
			zap.Int("requests_per_minute", rpm))
	} else {
		g, err := e.initGenkit(domainai.AISettings{
			Enabled:  true,
			Provider: p.Provider,
			APIKey:   p.APIKey,
			Model:    p.Model,
			Models:   p.EffectiveModels(),
			BaseURL:  p.BaseURL,
		})
		if err != nil {
			return nil, err
		}
		pc.genkit = g
	}
	return pc, nil
}

func (e *AIEngine) initGenkit(settings domainai.AISettings) (*genkit.Genkit, error) {
	var gOpts []genkit.GenkitOption
	var customPlugin *oai.OpenAICompatible

	if settings.Enabled {
		providerName := strings.ToLower(settings.Provider)
		e.logger.Info("Initializing Genkit with provider", zap.String("provider", providerName))

		// The official plugins panic during Init when no key is configured.
		// Return a normal error instead so a malformed profile only gets
		// skipped, never takes the whole server down.
		if (providerName == "openai" || providerName == "gemini") && strings.TrimSpace(settings.APIKey) == "" {
			return nil, fmt.Errorf("%s 官方接口缺少 API 密钥", providerName)
		}

		switch providerName {
		case "gemini":
			gOpts = append(gOpts, genkit.WithPlugins(
				&googlegenai.GoogleAI{
					APIKey: settings.APIKey,
				},
			))
		case "openai":
			gOpts = append(gOpts, genkit.WithPlugins(
				&openai.OpenAI{
					APIKey: settings.APIKey,
				},
			))
		case "custom":
			customPlugin = &oai.OpenAICompatible{
				Provider: "custom",
				APIKey:   settings.APIKey,
				BaseURL:  settings.BaseURL,
			}
			gOpts = append(gOpts, genkit.WithPlugins(customPlugin))
		default:
			e.logger.Warn("Unknown provider, falling back to gemini default", zap.String("provider", settings.Provider))
			gOpts = append(gOpts, genkit.WithPlugins(
				&googlegenai.GoogleAI{},
			))
		}
	}

	g := genkit.Init(e.ctx, gOpts...)

	// Dynamically register every custom model the profile may use. Genkit's
	// custom/oai plugin resolves models by name, so all of them must be defined
	// up front; otherwise only the first model would be callable.
	if settings.Enabled && strings.ToLower(settings.Provider) == "custom" && customPlugin != nil {
		models := settings.Models
		if len(models) == 0 && settings.Model != "" {
			models = []string{settings.Model}
		}
		for _, modelName := range models {
			modelID := strings.TrimSpace(modelName)
			if modelID == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(modelID), "custom/") {
				modelID = modelID[7:]
			}
			e.logger.Info("Registering custom model with compat_oai plugin", zap.String("modelID", modelID))
			customPlugin.DefineModel("custom", modelID, genai.ModelOptions{
				Supports: &genai.ModelSupports{
					Multiturn:   true,
					SystemRole:  true,
					Constrained: "all",
				},
			})
		}
	}

	return g, nil
}

// ReloadSettings updates settings and restarts/stops workers if state changes.
func (e *AIEngine) ReloadSettings(settings domainai.AISettings) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.logger.Info("Reloading AI Engine settings...",
		zap.Bool("enabled", settings.Enabled),
		zap.String("provider", settings.Provider),
		zap.String("strategy", settings.Strategy),
	)

	oldEnabled := e.settings.Enabled
	e.settings = settings
	e.selector = NewProfileSelector(settings)
	e.clients = make(map[string]*profileClient)

	if settings.Enabled {
		if err := e.initClients(settings); err != nil {
			e.logger.Warn("Some profiles failed to initialize", zap.Error(err))
		}
	}

	// If it was disabled and now enabled, start workers
	if !oldEnabled && settings.Enabled && !e.activeWorkers {
		e.logger.Info("AI Engine was enabled. Starting workers...")
		e.activeWorkers = true
		e.wg.Add(1)
		go e.workerLoop(0)
		go e.backfillLoop()
	}

	return nil
}

// Start starts the AI processing worker loops.
func (e *AIEngine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.settings.Enabled {
		e.logger.Info("AI Engine is disabled by configuration")
		return
	}

	e.logger.Info("Starting AI Engine workers...",
		zap.String("provider", e.settings.Provider),
		zap.String("model", e.settings.Model),
	)

	e.activeWorkers = true
	// Start a single worker to avoid concurrent rate limit issues
	e.wg.Add(1)
	go e.workerLoop(0)

	// Start backfill check
	go e.backfillLoop()
}

// Stop gracefully shuts down the workers.
func (e *AIEngine) Stop() {
	e.cancel()
	e.wg.Wait()
	e.logger.Info("AI Engine stopped successfully")
}

// Settings returns a copy of the current AI settings (thread-safe).
func (e *AIEngine) Settings() domainai.AISettings {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.settings
}

// IsEnabled returns whether the engine is currently enabled (thread-safe).
func (e *AIEngine) IsEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.settings.Enabled
}

// QueueLength returns the current number of pending items in the queue.
func (e *AIEngine) QueueLength() int {
	return len(e.queue)
}

// RunBackfill enqueues unanalyzed items (exported for handler use).
func (e *AIEngine) RunBackfill() {
	e.runBackfill()
}

// Enqueue adds an item ID to the AI analysis queue.
func (e *AIEngine) Enqueue(itemID int64) {
	e.mu.RLock()
	enabled := e.settings.Enabled
	e.mu.RUnlock()

	if !enabled || itemID <= 0 {
		return
	}

	e.queueMu.Lock()
	defer e.queueMu.Unlock()
	if _, exists := e.queuedItems[itemID]; exists {
		return
	}

	select {
	case e.queue <- itemID:
		e.queuedItems[itemID] = struct{}{}
	default:
		e.logger.Warn("AI processing queue is full, dropping item", zap.Int64("id", itemID))
	}
}

func (e *AIEngine) releaseQueuedItem(itemID int64) {
	e.queueMu.Lock()
	delete(e.queuedItems, itemID)
	e.queueMu.Unlock()
}

func (e *AIEngine) workerLoop(workerID int) {
	defer e.wg.Done()
	e.logger.Info("AI Worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Info("AI Worker stopping", zap.Int("worker_id", workerID))
			return
		case itemID := <-e.queue:
			err := e.AnalyzeItem(itemID)
			if err != nil {
				e.logger.Error("Failed to analyze item", zap.Int64("id", itemID), zap.Error(err))
				time.Sleep(2 * time.Second) // rate-limiting backoff
			}
			e.releaseQueuedItem(itemID)
		}
	}
}

func (e *AIEngine) backfillLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Run initial backfill on startup
	e.runBackfill()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.mu.RLock()
			enabled := e.settings.Enabled
			e.mu.RUnlock()
			if enabled {
				e.runBackfill()
			}
		}
	}
}

func (e *AIEngine) runBackfill() {
	items, err := e.db.GetUnanalyzedItems(50)
	if err != nil {
		e.logger.Error("Failed to query unanalyzed items in backfill", zap.Error(err))
		return
	}
	if len(items) > 0 {
		e.logger.Info("Backfilling unanalyzed items", zap.Int("count", len(items)))
		for _, item := range items {
			e.Enqueue(item.ID)
		}
	}
}

// AnalyzeItem executes the AI analysis for a specific item and saves the result.
// Uses the profile selector for multi-profile strategies with automatic failover.
func (e *AIEngine) AnalyzeItem(itemID int64) error {
	e.mu.RLock()
	settings := e.settings
	selector := e.selector
	clients := e.clients
	e.mu.RUnlock()

	if !settings.Enabled {
		return fmt.Errorf("AI engine is disabled")
	}
	if selector.EnabledCount() == 0 {
		return fmt.Errorf("no enabled AI profiles available")
	}

	scrapedItem, err := e.db.GetScrapedItem(itemID)
	if err != nil {
		return fmt.Errorf("failed to get scraped item: %w", err)
	}
	if scrapedItem == nil {
		return fmt.Errorf("scraped item not found: %d", itemID)
	}

	prompt := e.buildPrompt(settings, scrapedItem)

	// Try up to N slots (N = enabled profiles × their models). A time budget
	// keeps a provider with many dead/rate-limited free models from pinning the
	// worker: fast 429s still cycle through many models, while slow timeouts
	// stop after a couple of attempts.
	maxAttempts := selector.EnabledCount()
	deadline := time.Now().Add(5 * time.Minute)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if time.Now().After(deadline) {
			e.logger.Warn("AI attempt budget exhausted, giving up on item",
				zap.Int64("item_id", itemID), zap.Int("attempts", attempt))
			break
		}
		sel := selector.Next()
		if sel == nil {
			return fmt.Errorf("no AI profile available")
		}

		pc, ok := clients[sel.Profile.ID]
		if !ok {
			e.logger.Warn("Client not found for profile, skipping",
				zap.String("profile", sel.Profile.Name), zap.String("id", sel.Profile.ID))
			selector.MarkUnhealthy(sel.Key())
			continue
		}

		ctx, cancel := context.WithTimeout(e.ctx, 120*time.Second)
		rawJSON, genErr := e.callProfile(ctx, pc, sel.Model, prompt)
		cancel()

		if genErr != nil {
			e.logger.Warn("Model failed, trying next",
				zap.String("profile", sel.Profile.Name),
				zap.String("model", sel.Model),
				zap.Error(genErr))
			selector.MarkUnhealthy(sel.Key())
			continue
		}

		// Success — mark healthy and parse
		selector.MarkHealthy(sel.Key())

		var result AIAnalysisResult
		if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
			return fmt.Errorf("failed to parse AI response as JSON (profile: %s, model: %s): %w\nRaw: %s", sel.Profile.Name, sel.Model, err, rawJSON)
		}

		a := domainai.AIAnalysis{
			ItemID:        scrapedItem.ID,
			AICategory:    result.Category,
			AISubcategory: result.Subcategory,
			QualityScore:  result.QualityScore,
			AISummary:     result.Summary,
			AIComment:     result.Comment,
			AITags:        strings.Join(result.Tags, ","),
			ModelUsed:     sel.Model,
		}

		if err := e.db.InsertAIAnalysis(a); err != nil {
			return fmt.Errorf("failed to save AI analysis: %w", err)
		}

		e.logger.Info("Successfully analyzed item",
			zap.Int64("item_id", scrapedItem.ID),
			zap.String("profile", sel.Profile.Name),
			zap.String("model", sel.Model),
			zap.String("category", result.Category),
			zap.Int("score", result.QualityScore),
		)
		return nil
	}

	return fmt.Errorf("all enabled AI profiles failed for item %d", itemID)
}

// callProfile invokes the appropriate client for a profile + model.
// Applies per-profile rate limiting before the request.
func (e *AIEngine) callProfile(ctx context.Context, pc *profileClient, model, prompt string) (string, error) {
	// Per-profile rate limit: wait for a token before calling
	pc.rateLimiter.Wait()

	if pc.lmstudio != nil {
		schema := json.RawMessage(llm.AnalysisResponseSchema)
		return pc.lmstudio.GenerateWithSchemaModel(ctx, model, prompt, &schema)
	}
	resp, err := genkit.Generate(ctx, pc.genkit,
		genai.WithModelName(model),
		genai.WithPrompt(prompt),
		genai.WithOutputFormat("json"),
	)
	if err != nil {
		return "", err
	}
	return llm.StripMarkdownFences(resp.Text()), nil
}

// callDailyProfile invokes a profile for daily report generation.
// It uses the daily report schema instead of the per-item analysis schema.
func (e *AIEngine) callDailyProfile(ctx context.Context, pc *profileClient, model, prompt string) (string, error) {
	pc.rateLimiter.Wait()

	if pc.lmstudio != nil {
		schema := json.RawMessage(DailyReportResponseSchema)
		return pc.lmstudio.GenerateWithSchemaModel(ctx, model, prompt, &schema)
	}
	resp, err := genkit.Generate(ctx, pc.genkit,
		genai.WithModelName(model),
		genai.WithPrompt(prompt),
		genai.WithOutputFormat("json"),
	)
	if err != nil {
		return "", err
	}
	return llm.StripMarkdownFences(resp.Text()), nil
}

// TestProfile performs a single, side-effect-free generation against a draft
// provider profile. It is used by the settings screen to validate a connection
// before the configuration is saved. No analysis row is written to the DB.
func (e *AIEngine) TestProfile(ctx context.Context, p domainai.AIProviderProfile, systemPrompt string) (map[string]any, int64, error) {
	start := time.Now()
	p = normalizeTestProfile(p)
	// Keep the whole list namespaced too: the custom plugin must be able to
	// resolve every model the profile declares, not just the tested one.
	if len(p.Models) > 0 {
		p.Models = domainai.NormalizeModelList(p.Provider, p.Models)
	}
	if p.Model != "" && !containsModel(p.Models, p.Model) {
		p.Models = append([]string{p.Model}, p.Models...)
	}

	pc, err := e.buildClient(p)
	if err != nil {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("无法初始化模型客户端: %w", err)
	}

	sample := &item.ScrapedItem{
		Title:        "Grabby 连通性测试：一条用于验证模型输出的样本资讯",
		OriginSource: "Grabby System",
		Summary:      "这是一条用于验证 AI 接口连通性、结构化输出与中文评分能力的测试内容。",
		Content:      "人工智能阅读助手 Grabby 今天完成了模型接入设置的重构。新的设置界面支持从开放模型目录中选择服务商与模型，自动填充接口地址，并在保存前对当前草稿配置执行连通性测试。本次测试不写入数据库，仅用于确认模型能够按照约定的 JSON 结构返回分类、评分与摘要字段。",
		Category:     "article",
	}

	e.mu.RLock()
	settings := e.settings
	e.mu.RUnlock()
	settings.SystemPrompt = strings.TrimSpace(systemPrompt)

	raw, err := e.callProfile(ctx, pc, p.Model, e.buildPrompt(settings, sample))
	if err != nil {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("AI 接口连接失败: %w", err)
	}

	var result AIAnalysisResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, time.Since(start).Milliseconds(), fmt.Errorf("模型未返回有效的结构化 JSON: %w（原始响应: %.200s）", err, raw)
	}

	return map[string]any{
		"ai_category":    result.Category,
		"ai_subcategory": result.Subcategory,
		"quality_score":  result.QualityScore,
		"ai_summary":     result.Summary,
		"ai_comment":     result.Comment,
		"ai_tags":        strings.Join(result.Tags, ","),
		"model_used":     p.Model,
	}, time.Since(start).Milliseconds(), nil
}

// containsModel reports whether models already includes target
// (case-insensitive).
func containsModel(models []string, target string) bool {
	for _, m := range models {
		if strings.EqualFold(strings.TrimSpace(m), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

// normalizeTestProfile makes sure a draft model id carries the namespace that
// Genkit expects for the selected backend driver.
func normalizeTestProfile(p domainai.AIProviderProfile) domainai.AIProviderProfile {
	p.Provider = strings.ToLower(strings.TrimSpace(p.Provider))
	p.Model = strings.TrimSpace(p.Model)
	p.BaseURL = strings.TrimSpace(p.BaseURL)

	prefix := ""
	switch p.Provider {
	case "gemini":
		prefix = "googleai/"
	case "openai":
		prefix = "openai/"
	case "custom":
		prefix = "custom/"
	}
	if prefix != "" && p.Model != "" && !strings.HasPrefix(strings.ToLower(p.Model), prefix) {
		// Drop any stale namespace from a previously selected provider.
		if idx := strings.Index(p.Model, "/"); idx > 0 {
			head := strings.ToLower(p.Model[:idx])
			if head == "googleai" || head == "openai" || head == "custom" {
				p.Model = p.Model[idx+1:]
			}
		}
		p.Model = prefix + p.Model
	}
	return p
}

// buildPrompt constructs the analysis prompt from settings and item.
func (e *AIEngine) buildPrompt(settings domainai.AISettings, scrapedItem *item.ScrapedItem) string {
	title := scrapedItem.Title
	summary := scrapedItem.Summary
	content := scrapedItem.Content
	if len(content) > 3000 {
		content = content[:3000] + "...(content truncated)..."
	}

	sysPrompt := settings.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = DefaultSystemPrompt
	}

	prompt := sysPrompt
	prompt = strings.ReplaceAll(prompt, "{{.Title}}", title)
	prompt = strings.ReplaceAll(prompt, "{{.OriginSource}}", scrapedItem.OriginSource)
	prompt = strings.ReplaceAll(prompt, "{{.Summary}}", summary)
	prompt = strings.ReplaceAll(prompt, "{{.Content}}", content)
	return prompt
}
