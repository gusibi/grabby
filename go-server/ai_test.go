package main

import (
	"context"
	"testing"
)

func TestAIEngine_DisabledMode(t *testing.T) {
	// Initialize in-memory database
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	logger := GetLogger()

	// Initialize disabled AI settings
	settings := AISettings{
		Enabled:          false,
		Provider:         "gemini",
		APIKey:           "",
		Model:            "googleai/gemini-2.0-flash",
		QualityThreshold: 7,
	}

	engine, err := NewAIEngine(settings, db, logger)
	if err != nil {
		t.Fatalf("Failed to create AIEngine: %v", err)
	}

	// In disabled mode, Start should log and return immediately
	engine.Start()

	// Enqueue should return immediately without blocking
	engine.Enqueue(42)

	// Stop should shut down cleanly
	engine.Stop()
}

func TestAIDailyManager_DisabledMode(t *testing.T) {
	// Initialize in-memory database
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	logger := GetLogger()

	// Initialize disabled AI settings
	settings := AISettings{
		Enabled:          false,
		Provider:         "gemini",
		APIKey:           "",
		Model:            "googleai/gemini-2.0-flash",
		QualityThreshold: 7,
	}

	engine, err := NewAIEngine(settings, db, logger)
	if err != nil {
		t.Fatalf("Failed to create AIEngine: %v", err)
	}

	dailyManager := NewAIDailyManager(db, engine, logger)

	// GenerateDailyReport should return an error stating AI engine is disabled
	ctx := context.Background()
	_, err = dailyManager.GenerateDailyReport(ctx, "2026-06-07")
	if err == nil {
		t.Errorf("Expected error when generating report with disabled AI engine, but got nil")
	}
}

func TestAIEngine_CustomModelNormalization(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// 1. Load settings with custom provider and a model having leading/trailing spaces
	envSettings := AISettings{
		Enabled:  true,
		Provider: " custom ",
		Model:    " google/gemma-4-12b ",
		BaseURL:  " http://localhost:1234/v1 ",
		APIKey:   " test_key ",
	}

	loaded, err := db.LoadAISettings(envSettings)
	if err != nil {
		t.Fatalf("Failed to load AI settings: %v", err)
	}

	// 2. Assert they are trimmed and the model is prefixed with custom/
	if loaded.Provider != "custom" {
		t.Errorf("Expected Provider to be 'custom', got '%s'", loaded.Provider)
	}
	if loaded.Model != "custom/google/gemma-4-12b" {
		t.Errorf("Expected Model to be 'custom/google/gemma-4-12b', got '%s'", loaded.Model)
	}
	if loaded.BaseURL != "http://localhost:1234/v1" {
		t.Errorf("Expected BaseURL to be 'http://localhost:1234/v1', got '%s'", loaded.BaseURL)
	}
	if loaded.APIKey != "test_key" {
		t.Errorf("Expected APIKey to be 'test_key', got '%s'", loaded.APIKey)
	}

	// 3. Test that initGenkit runs and registers it without panicking
	engine, err := NewAIEngine(loaded, db, GetLogger())
	if err != nil {
		t.Fatalf("Failed to create AIEngine: %v", err)
	}
	defer engine.Stop()

	// Verify that the model is defined in the genkit registry
	isDefined := engine.settings.Model == "custom/google/gemma-4-12b"
	if !isDefined {
		t.Errorf("Model was not normalized to custom/google/gemma-4-12b")
	}
}

