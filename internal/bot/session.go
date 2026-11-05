package bot

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"autojob/internal/freehire"
)

const PageSize = 4

type SearchSession struct {
	ID         string
	TelegramID int64
	Mode       string // "all" or "unseen"
	City       string
	Categories []string
	Days       int
	Jobs       []freehire.Job
	SeenMap    map[string]bool
	CreatedAt  time.Time
}

func (s *SearchSession) TotalPages() int {
	if len(s.Jobs) == 0 {
		return 0
	}
	return (len(s.Jobs) + PageSize - 1) / PageSize
}

func (s *SearchSession) PageJobs(page int) []freehire.Job {
	if page < 0 {
		return nil
	}
	start := page * PageSize
	if start >= len(s.Jobs) {
		return nil
	}
	end := start + PageSize
	if end > len(s.Jobs) {
		end = len(s.Jobs)
	}
	return s.Jobs[start:end]
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SearchSession
	bySlug   map[string]freehire.Job
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*SearchSession),
		bySlug:   make(map[string]freehire.Job),
	}
}

func (st *SessionStore) Create(telegramID int64, mode, city string, categories []string, days int, jobs []freehire.Job, seenMap map[string]bool) *SearchSession {
	st.mu.Lock()
	defer st.mu.Unlock()

	// Evict sessions older than 24 hours
	cutoff := time.Now().Add(-24 * time.Hour)
	for id, sess := range st.sessions {
		if sess.CreatedAt.Before(cutoff) {
			delete(st.sessions, id)
		}
	}

	for _, j := range jobs {
		st.bySlug[j.PublicSlug] = j
	}

	if seenMap == nil {
		seenMap = make(map[string]bool)
	}

	id := shortID()
	sess := &SearchSession{
		ID:         id,
		TelegramID: telegramID,
		Mode:       mode,
		City:       city,
		Categories: categories,
		Days:       days,
		Jobs:       jobs,
		SeenMap:    seenMap,
		CreatedAt:  time.Now(),
	}
	st.sessions[id] = sess
	return sess
}

func (st *SessionStore) Get(id string) (*SearchSession, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	sess, ok := st.sessions[id]
	return sess, ok
}

func (st *SessionStore) GetJobBySlug(slug string) (freehire.Job, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	j, ok := st.bySlug[slug]
	return j, ok
}

func (st *SessionStore) CacheJob(j freehire.Job) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.bySlug[j.PublicSlug] = j
}

func shortID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
