package ai

import (
	"sync"
	"testing"

	domainai "go-server/internal/domain/ai"
	"go.uber.org/zap"
)

func newQueueTestEngine(t *testing.T) *AIEngine {
	t.Helper()

	engine, err := NewAIEngine(domainai.AISettings{Enabled: true}, nil, zap.NewNop())
	if err != nil {
		t.Fatalf("NewAIEngine returned error: %v", err)
	}
	t.Cleanup(engine.cancel)
	return engine
}

func TestEnqueueDeduplicatesItemID(t *testing.T) {
	engine := newQueueTestEngine(t)

	engine.Enqueue(42)
	engine.Enqueue(42)

	if got := engine.QueueLength(); got != 1 {
		t.Fatalf("queue length = %d, want 1 for duplicate item IDs", got)
	}
}

func TestEnqueueDeduplicatesConcurrentItemID(t *testing.T) {
	engine := newQueueTestEngine(t)

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			engine.Enqueue(42)
		}()
	}
	wg.Wait()

	if got := engine.QueueLength(); got != 1 {
		t.Fatalf("queue length = %d, want 1 for concurrently duplicated item IDs", got)
	}
}

func TestFinishedItemCanBeEnqueuedAgain(t *testing.T) {
	engine := newQueueTestEngine(t)

	engine.Enqueue(42)
	<-engine.queue
	engine.releaseQueuedItem(42)
	engine.Enqueue(42)

	if got := engine.QueueLength(); got != 1 {
		t.Fatalf("queue length = %d, want 1 after re-enqueueing a finished item", got)
	}
}

func TestDroppedItemCanBeEnqueuedWhenCapacityReturns(t *testing.T) {
	engine := newQueueTestEngine(t)

	for id := int64(1); id <= 500; id++ {
		engine.Enqueue(id)
	}
	engine.Enqueue(501)

	processedID := <-engine.queue
	engine.releaseQueuedItem(processedID)
	engine.Enqueue(501)

	if got := engine.QueueLength(); got != 500 {
		t.Fatalf("queue length = %d, want 500 after retrying previously dropped item", got)
	}
}
