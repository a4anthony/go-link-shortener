package handler

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/a4anthony/go-link-shortener/internal/analytics"
	"github.com/a4anthony/go-link-shortener/internal/domain"
)

type fakeEnqueuer struct {
	mu     sync.Mutex
	events []analytics.ClickEvent
	accept bool
}

func (f *fakeEnqueuer) Enqueue(e analytics.ClickEvent) bool {
	f.mu.Lock()
	f.events = append(f.events, e)
	f.mu.Unlock()
	return f.accept
}

func TestPipelineClickRecorder_Record(t *testing.T) {
	enq := &fakeEnqueuer{accept: true}
	rec := NewPipelineClickRecorder(enq)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/abc1234", nil)
	req.Header.Set("Referer", "https://ref.example.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0")
	req.RemoteAddr = "203.0.113.5:1234"
	c.Request = req

	link := &domain.Link{ID: uuid.New(), TenantID: uuid.New(), Code: "abc1234", TargetURL: "https://example.com"}
	rec.Record(c, link)

	require.Len(t, enq.events, 1)
	e := enq.events[0]
	assert.Equal(t, link.ID, e.LinkID)
	assert.Equal(t, link.TenantID, e.TenantID)
	assert.Equal(t, "abc1234", e.Code)
	assert.Equal(t, "https://ref.example.com", e.Referrer)
	assert.Contains(t, e.UserAgent, "Chrome")
	assert.NotEmpty(t, e.IP)
	assert.False(t, e.OccurredAt.IsZero())
}
