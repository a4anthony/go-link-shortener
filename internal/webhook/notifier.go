package webhook

import (
	"strings"

	"github.com/google/uuid"

	"github.com/a4anthony/go-link-shortener/internal/domain"
)

// Notifier adapts domain changes into webhook events and hands them to the
// Dispatcher. It implements service.LinkEventSink (link.created) and
// analytics.BatchHook (batched link.clicked).
type Notifier struct {
	dispatcher *Dispatcher
	baseURL    string
}

// NewNotifier builds a Notifier. baseURL renders absolute short URLs in payloads.
func NewNotifier(dispatcher *Dispatcher, baseURL string) *Notifier {
	return &Notifier{dispatcher: dispatcher, baseURL: strings.TrimRight(baseURL, "/")}
}

// LinkCreated dispatches a link.created event for a newly-created link.
func (n *Notifier) LinkCreated(l *domain.Link) {
	n.dispatcher.Dispatch(domain.Event{
		Type:     domain.EventLinkCreated,
		TenantID: l.TenantID,
		Data: map[string]any{
			"id":         l.ID.String(),
			"code":       l.Code,
			"short_url":  n.baseURL + "/" + l.Code,
			"target_url": l.TargetURL,
			"created_at": l.CreatedAt,
		},
	})
}

// ClicksFlushed dispatches batched link.clicked events: one per link in the
// flushed batch, carrying the number of clicks recorded for it.
func (n *Notifier) ClicksFlushed(clicks []domain.Click) {
	if len(clicks) == 0 {
		return
	}

	type agg struct {
		tenantID uuid.UUID
		count    int
	}
	byLink := make(map[uuid.UUID]*agg)
	for _, c := range clicks {
		a := byLink[c.LinkID]
		if a == nil {
			a = &agg{tenantID: c.TenantID}
			byLink[c.LinkID] = a
		}
		a.count++
	}

	for linkID, a := range byLink {
		n.dispatcher.Dispatch(domain.Event{
			Type:     domain.EventLinkClicked,
			TenantID: a.tenantID,
			Data: map[string]any{
				"link_id": linkID.String(),
				"count":   a.count,
			},
		})
	}
}
