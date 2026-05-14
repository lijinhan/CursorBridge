// Package knowledgebase implements local storage for Cursor Rules (knowledge base).
// When Cursor calls KnowledgeBaseAdd/List/Get/Remove, we persist rules locally
// instead of relying on Cursor's upstream server (which returns 401 for BYOK users).
package knowledgebase

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cursorbridge/internal/appdir"
)

// Item represents a stored knowledge base rule.
type Item struct {
	ID          string `json:"id"`
	Knowledge   string `json:"knowledge"`
	Title       string `json:"title"`
	CreatedAt   string `json:"created_at"`
	IsGenerated bool   `json:"is_generated"`
	GitOrigin   string `json:"git_origin,omitempty"`
}

var (
	mu    sync.Mutex
	items []Item
	loaded bool
)

func dataPath() string {
	dir, err := appdir.ConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "knowledgebase.json")
}

func ensureLoaded() {
	if loaded {
		return
	}
	loaded = true
	data, err := os.ReadFile(dataPath())
	if err != nil {
		items = nil
		return
	}
	_ = json.Unmarshal(data, &items)
}

func save() {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(dataPath())
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(dataPath(), data, 0o644)
}

func genID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Add stores a new rule and returns its ID.
func Add(knowledge, title, gitOrigin string) string {
	mu.Lock()
	defer mu.Unlock()
	ensureLoaded()

	item := Item{
		ID:        genID(),
		Knowledge: knowledge,
		Title:     title,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		GitOrigin: gitOrigin,
	}
	items = append(items, item)
	save()
	return item.ID
}

// List returns all stored rules, optionally filtered by git origin.
func List(gitOrigin string) []Item {
	mu.Lock()
	defer mu.Unlock()
	ensureLoaded()

	if gitOrigin == "" {
		result := make([]Item, len(items))
		copy(result, items)
		return result
	}
	var result []Item
	for _, it := range items {
		if it.GitOrigin == gitOrigin || it.GitOrigin == "" {
			result = append(result, it)
		}
	}
	return result
}

// Get returns a single rule by ID.
func Get(id string) (Item, bool) {
	mu.Lock()
	defer mu.Unlock()
	ensureLoaded()

	for _, it := range items {
		if it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

// Remove deletes a rule by ID.
func Remove(id string) bool {
	mu.Lock()
	defer mu.Unlock()
	ensureLoaded()

	for i, it := range items {
		if it.ID == id {
			items = append(items[:i], items[i+1:]...)
			save()
			return true
		}
	}
	return false
}
