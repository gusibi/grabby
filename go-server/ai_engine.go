package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	oai "github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"go.uber.org/zap"
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
	profile  AIProviderProfile
	genkit   *genkit.Genkit   // non-nil for gemini/openai/custom
	lmstudio *LMStudioClient  // non-nil for lmstudio
}

// AIEngine handles queueing and executing AI analysis requests.
type AIEngine struct {
	mu            sync.RWMutex
	settings      AISettings
	db            *Database
	logger        *zap.Logger
	selector      *ProfileSelector
	clients       map[string]*profileClient // keyed by profile ID
	queue         chan int64
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	activeWorkers bool
}

// NewAIEngine initializes and returns a new AI processing engine.
func NewAIEngine(settings AISettings, db *Database, logger *zap.Logger) (*AIEngine, error) {
	ctx, cancel := context.WithCancel(context.Background())

	engine := &AIEngine{
		settings: settings,
		db:       db,
		logger:   logger,
		clients:  make(map[string]*profileClient),
		queue:    make(chan int64, 500),
		ctx:      ctx,
		cancel:   cancel,
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
func (e *AIEngine) initClients(settings AISettings) error {
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
func (e *AIEngine) buildClient(p AIProviderProfile) (*profileClient, error) {
	pc := &profileClient{profile: p}
	if strings.ToLower(p.Provider) == "lmstudio" {
		pc.lmstudio = NewLMStudioClient(p.BaseURL, p.Model, e.logger)
		e.logger.Info("Initialized LM Studio client", zap.String("profile", p.Name), zap.String("base_url", p.BaseURL))
	} else {
		g, err := e.initGenkit(AISettings{
			Enabled:  true,
			Provider: p.Provider,
			APIKey:   p.APIKey,
			Model:    p.Model,
			BaseURL:  p.BaseURL,
		})
		if err != nil {
			return nil, err
		}
		pc.genkit = g
	}
	return pc, nil
}

func (e *AIEngine) initGenkit(settings AISettings) (*genkit.Genkit, error) {
	var gOpts []genkit.GenkitOption
	var customPlugin *oai.OpenAICompatible

	if settings.Enabled {
		providerName := strings.ToLower(settings.Provider)
		e.logger.Info("Initializing Genkit with provider", zap.String("provider", providerName))

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

	// Dynamically register the custom model if using custom provider
	if settings.Enabled && strings.ToLower(settings.Provider) == "custom" && customPlugin != nil {
		modelName := settings.Model
		if modelName != "" {
			modelID := modelName
			if strings.HasPrefix(strings.ToLower(modelID), "custom/") {
				modelID = modelID[7:]
			}
			e.logger.Info("Registering custom model with compat_oai plugin", zap.String("modelID", modelID))
			customPlugin.DefineModel("custom", modelID, ai.ModelOptions{
				Supports: &ai.ModelSupports{
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
func (e *AIEngine) ReloadSettings(settings AISettings) error {
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
		for i := 0; i < 2; i++ {
			e.wg.Add(1)
			go e.workerLoop(i)
		}
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
	// Start 2 concurrent workers
	for i := 0; i < 2; i++ {
		e.wg.Add(1)
		go e.workerLoop(i)
	}

	// Start backfill check
	go e.backfillLoop()
}

// Stop gracefully shuts down the workers.
func (e *AIEngine) Stop() {
	e.cancel()
	e.wg.Wait()
	e.logger.Info("AI Engine stopped successfully")
}

// Enqueue adds an item ID to the AI analysis queue.
func (e *AIEngine) Enqueue(itemID int64) {
	e.mu.RLock()
	enabled := e.settings.Enabled
	e.mu.RUnlock()

	if !enabled || itemID <= 0 {
		return
	}
	select {
	case e.queue <- itemID:
	default:
		e.logger.Warn("AI processing queue is full, dropping item", zap.Int64("id", itemID))
	}
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

	item, err := e.db.GetScrapedItem(itemID)
	if err != nil {
		return fmt.Errorf("failed to get scraped item: %w", err)
	}
	if item == nil {
		return fmt.Errorf("scraped item not found: %d", itemID)
	}

	prompt := e.buildPrompt(settings, item)

	// Try up to N profiles (N = number of enabled profiles)
	maxAttempts := selector.EnabledCount()
	for attempt := 0; attempt < maxAttempts; attempt++ {
		profile := selector.Next()
		if profile == nil {
			return fmt.Errorf("no AI profile available")
		}

		pc, ok := clients[profile.ID]
		if !ok {
			e.logger.Warn("Client not found for profile, skipping",
				zap.String("profile", profile.Name), zap.String("id", profile.ID))
			selector.MarkUnhealthy(profile.ID)
			continue
		}

		ctx, cancel := context.WithTimeout(e.ctx, 120*time.Second)
		rawJSON, genErr := e.callProfile(ctx, pc, prompt)
		cancel()

		if genErr != nil {
			e.logger.Warn("Profile failed, trying next",
				zap.String("profile", profile.Name), zap.Error(genErr))
			selector.MarkUnhealthy(profile.ID)
			continue
		}

		// Success — mark healthy and parse
		selector.MarkHealthy(profile.ID)

		var result AIAnalysisResult
		if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
			return fmt.Errorf("failed to parse AI response as JSON (profile: %s): %w\nRaw: %s", profile.Name, err, rawJSON)
		}

		a := AIAnalysis{
			ItemID:        item.ID,
			AICategory:    result.Category,
			AISubcategory: result.Subcategory,
			QualityScore:  result.QualityScore,
			AISummary:     result.Summary,
			AIComment:     result.Comment,
			AITags:        strings.Join(result.Tags, ","),
			ModelUsed:     profile.Model,
		}

		if err := e.db.InsertAIAnalysis(a); err != nil {
			return fmt.Errorf("failed to save AI analysis: %w", err)
		}

		e.logger.Info("Successfully analyzed item",
			zap.Int64("item_id", item.ID),
			zap.String("profile", profile.Name),
			zap.String("category", result.Category),
			zap.Int("score", result.QualityScore),
		)
		return nil
	}

	return fmt.Errorf("all enabled AI profiles failed for item %d", itemID)
}

// callProfile invokes the appropriate client for a profile.
func (e *AIEngine) callProfile(ctx context.Context, pc *profileClient, prompt string) (string, error) {
	if pc.lmstudio != nil {
		schema := json.RawMessage(analysisResponseSchema)
		return pc.lmstudio.GenerateWithSchema(ctx, prompt, &schema)
	}
	resp, err := genkit.Generate(ctx, pc.genkit,
		ai.WithModelName(pc.profile.Model),
		ai.WithPrompt(prompt),
	)
	if err != nil {
		return "", err
	}
	return resp.Text(), nil
}

// buildPrompt constructs the analysis prompt from settings and item.
func (e *AIEngine) buildPrompt(settings AISettings, item *ScrapedItem) string {
	title := item.Title
	summary := item.Summary
	content := item.Content
	if len(content) > 3000 {
		content = content[:3000] + "...(content truncated)..."
	}

	sysPrompt := settings.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = DefaultSystemPrompt
	}

	prompt := sysPrompt
	prompt = strings.ReplaceAll(prompt, "{{.Title}}", title)
	prompt = strings.ReplaceAll(prompt, "{{.OriginSource}}", item.OriginSource)
	prompt = strings.ReplaceAll(prompt, "{{.Summary}}", summary)
	prompt = strings.ReplaceAll(prompt, "{{.Content}}", content)
	return prompt
}
