// Package agent manages Cursor agent sessions, LLM provider interactions,
// conversation history, tool execution, and usage statistics.
//
// The primary entry points are HandleRunSSE and HandleBidiAppend, which
// process streaming chat requests from the MITM proxy layer.
package agent

import (
	"context"
	"fmt"
	"cursorbridge/internal/logutil"
	"strings"
	"sync"
	"time"

	agentv1 "cursorbridge/internal/protocodec/gen/agent/v1"
	aiserverv1 "cursorbridge/internal/protocodec/gen/aiserver/v1"
)

// Session captures the conversation state Cursor sent through BidiAppend.
// We key sessions by RequestId (the BidiRequestId.request_id field) so the
// follow-up RunSSE call can look them back up without re-parsing the body.
//
// The Cursor agent loop normally streams BidiAppend (input) and RunSSE
// (output) on separate HTTP connections that share the same RequestId; the
// store is the only thing that bridges them.
type Session struct {
	RequestID      string
	ConversationID string
	UserText       string
	BugBotRequest  *aiserverv1.StreamBugBotRequest
	Mode           agentv1.AgentMode
	ModelDetails   *agentv1.ModelDetails
	Action         *agentv1.ConversationAction
	State          *agentv1.ConversationStateStructure
	Run            *agentv1.AgentRunRequest
	// CursorSystemPrompt holds the system prompt Cursor originally sent via
	// AgentRunRequest.custom_system_prompt or extracted from
	// ConversationStateStructure.root_prompt_messages_json. When non-empty,
	// buildMessageHistory uses it instead of defaultSystemPrompt so the BYOK
	// model receives Cursor's original persona/instructions rather than our
	// synthetic replacement.
	CursorSystemPrompt string
	// McpMap maps the OpenAI-visible function name (e.g. "mcp_0__search") to
	// the real (server_identifier, mcp_tool_name) pair. We expose every MCP
	// tool as a standalone OpenAI function so the model stops hallucinating
	// composite names — then translate back when it calls one.
	McpMap map[string]*McpRef

	// Plan state — populated by the local CreatePlan/AddTodo/UpdateTodo
	// tools. Projected into the system prompt on every turn so the model
	// can track progress without us having to serialize it through
	// ConversationState.
	PlanName     string
	PlanOverview string
	Todos        []*TodoEntry
}

type McpRef struct {
	// ServerName is the user-facing short name (McpDescriptor.server_name,
	// e.g. "context7"). Cursor writes this into McpArgs.name and renders it
	// in the UI pill.
	ServerName string
	// ServerID is the full routing identifier (McpDescriptor.server_identifier,
	// e.g. "plugin-context7-context7"). Goes into McpArgs.provider_identifier;
	// Cursor's MCP router uses this to pick the actual MCP server process.
	ServerID string
	ToolName string
}

// TodoEntry is a single item on a session's active plan. We keep these in
// the Session (not proto) because CreatePlan/AddTodo/UpdateTodo resolve
// locally — there's no ExecServer round-trip. The list is projected back
// into the system prompt on every turn so the model can track progress.
type TodoEntry struct {
	ID      string
	Content string
	Status  string
}

// ConvTurn is one user/assistant pair we've seen on a conversation. Cursor
// doesn't ship its own chat history in BidiAppend (it keeps it behind a
// server-side blob store the real cursor.sh backend implements), so we
// maintain our own per-conversation_id log and replay it whenever the user
// sends a new message with the same conversation_id.
//
// Messages (when non-nil) carries the exact OpenAI-format messages the turn
// added to the prompt — assistant-with-tool_calls + tool-role results. This
// is what lets the agent loop replay a tool-heavy turn without losing the
// intermediate tool state; plain string User/Assistant alone can't represent
// it. Old turns saved before rich history was introduced have nil Messages
// and fall back to the User/Assistant replay path.
type ConvTurn struct {
	User      string
	Assistant string
	Messages  []StoredMessage
}

// StoredMessage is a disk-friendly mirror of openAIMessage. We keep it here
// (not in openai.go) because history_disk.go marshals it into conversation.json
// and openai.go's openAIMessage uses json.RawMessage for Content which doesn't
// round-trip cleanly. Content here is always the string form.
type StoredMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
	ToolCalls  []StoredToolCall `json:"tool_calls,omitempty"`
}

type StoredToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type sessionStore struct {
	mu         sync.RWMutex
	byID       map[string]*Session
	byConv     map[string]*Session
	lastPut    *Session  // fallback when RunSSE ids don't match any BidiAppend id
	lastConv   *Session  // most recent session with UserText, for continuation rounds
	lastConvAt time.Time // when lastConv was last updated — bounds the continuation-fallback window
	// droppedIDConv remembers request_id → conversation_id for sessions
	// we've already finished with. Cursor sometimes retries RunSSE with a
	// recycled request_id after we've DropSession'd it; without this map
	// the retry would miss byID and the lastConv fallback could splice
	// the request into whichever chat happened to be active most recently.
	// Looking up the conversation_id here lets the retry rejoin its OWN
	// chat instead of cross-wiring to a sibling session.
	droppedIDConv map[string]droppedEntry
	history       map[string][]*ConvTurn // conversation_id -> turns in arrival order
	planEmitted   map[string]bool        // conversation_id -> did we already open Cursor's Plan panel?
	planByConv    map[string]*PlanState  // conversation_id -> authoritative plan state
}

type droppedEntry struct {
	convID    string
	droppedAt time.Time
}

// lastConvSafeFallback returns lastConv only when we can be reasonably sure
// the incoming RunSSE really belongs to it. In particular we refuse the
// fallback when more than one conversation has been active recently — in
// that case a request we can't identify could belong to either chat, and
// splicing it into the "most recent" one is a silent data-corruption
// hazard (prompt history from chat A persisted under chat B's id).
//
// Caller must hold s.mu (read or write lock).
func (s *sessionStore) lastConvSafeFallback() *Session {
	if s.lastConv == nil || s.lastConv.UserText == "" {
		return nil
	}
	if !s.lastConvAt.IsZero() && time.Since(s.lastConvAt) > lastConvMaxAge {
		return nil
	}
	// Count distinct conversations that still carry a live user prompt.
	// byConv holds at most one entry per conversation_id (re-puts overwrite),
	// so iterating gives a natural distinct count. More than one ⇒ ambiguous.
	seen := 0
	for convID, sess := range s.byConv {
		if sess == nil || sess.UserText == "" || convID == "" {
			continue
		}
		seen++
		if seen > 1 {
			return nil
		}
	}
	return s.lastConv
}

// lastConvMaxAge bounds how long after the last BidiAppend we still treat
// lastConv as a valid continuation target. Long enough to cover normal
// reconnect storms, short enough that a user switching chats doesn't get
// their new chat glued onto the previous one.
const lastConvMaxAge = 2 * time.Minute

// PlanState is the conversation-scoped plan snapshot. Cursor does not ship
// the plan back inside BidiAppend (it lives purely in our state), so we have
// to keep it keyed off conversation_id so that the next turn's session — a
// brand-new object built from a fresh BidiAppend — can still see the plan
// the previous turn created. Before this was session-scoped, and every new
// turn arrived with an empty Todos slice: UpdateTodo responded
// "no active plan — call CreatePlan first" and forced the model to rebuild
// the plan from scratch on every message.
type PlanState struct {
	Name     string
	Overview string
	Todos    []*TodoEntry
}

func init() { go store.sweep() }

var store = &sessionStore{
	byID:        map[string]*Session{},
	byConv:      map[string]*Session{},
	history:     map[string][]*ConvTurn{},
	planEmitted: map[string]bool{},
	planByConv:  map[string]*PlanState{},
}

// PlanStateFor returns a copy of the plan for a conversation (nil when none).
// Callers must not mutate the returned slice — use Save*/Append*/Update* helpers.
func PlanStateFor(conversationID string) *PlanState {
	if conversationID == "" {
		return nil
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	p := store.planByConv[conversationID]
	if p == nil {
		return nil
	}
	// Deep copy the todos so the caller can iterate without locking.
	todos := make([]*TodoEntry, len(p.Todos))
	for i, t := range p.Todos {
		cp := *t
		todos[i] = &cp
	}
	return &PlanState{Name: p.Name, Overview: p.Overview, Todos: todos}
}

// SavePlanState replaces (or creates) the plan for a conversation. Called by
// CreatePlan — drops any previous plan and installs the new one verbatim.
func SavePlanState(conversationID, name, overview string, todos []*TodoEntry) {
	if conversationID == "" {
		return
	}
	copied := make([]*TodoEntry, len(todos))
	for i, t := range todos {
		cp := *t
		copied[i] = &cp
	}
	store.mu.Lock()
	store.planByConv[conversationID] = &PlanState{Name: name, Overview: overview, Todos: copied}
	store.mu.Unlock()
}

// AppendTodo atomically adds a todo to the conversation's plan and returns
// the full updated list. Returns nil + false when there's no active plan
// (caller should tell the model to call CreatePlan first).
func AppendTodo(conversationID, content string) ([]*TodoEntry, bool) {
	if conversationID == "" {
		return nil, false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	p := store.planByConv[conversationID]
	if p == nil {
		return nil, false
	}
	id := fmt.Sprintf("t%d", len(p.Todos)+1)
	p.Todos = append(p.Todos, &TodoEntry{ID: id, Content: content, Status: "pending"})
	out := make([]*TodoEntry, len(p.Todos))
	for i, t := range p.Todos {
		cp := *t
		out[i] = &cp
	}
	return out, true
}

// UpdateTodoStatus finds a todo by id (or content prefix when id is empty),
// flips its status, and returns the full updated list. Second return value is
// true on success, false when either the plan doesn't exist or no todo matches.
func UpdateTodoStatus(conversationID, id, contentPrefix, status string) ([]*TodoEntry, bool) {
	if conversationID == "" {
		return nil, false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	p := store.planByConv[conversationID]
	if p == nil || len(p.Todos) == 0 {
		return nil, false
	}
	var target *TodoEntry
	if id != "" {
		for _, t := range p.Todos {
			if t.ID == id {
				target = t
				break
			}
		}
	}
	if target == nil && contentPrefix != "" {
		low := strings.ToLower(contentPrefix)
		for _, t := range p.Todos {
			if strings.HasPrefix(strings.ToLower(t.Content), low) {
				target = t
				break
			}
		}
	}
	if target == nil {
		return nil, false
	}
	target.Status = status
	out := make([]*TodoEntry, len(p.Todos))
	for i, t := range p.Todos {
		cp := *t
		out[i] = &cp
	}
	return out, true
}

// PlanEmittedFor reports whether Cursor's Plan panel has already been
// opened for this conversation. Conversation-scoped (not session-scoped)
// so continuation rounds or reconnect-clones don't accidentally reuse or
// reset the gate; a new chat always gets a fresh panel.
func PlanEmittedFor(conversationID string) bool {
	if conversationID == "" {
		return false
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.planEmitted[conversationID]
}

// MarkPlanEmitted records that we fired a CreatePlanRequestQuery for this
// conversation so subsequent CreatePlan calls in the same chat silently
// update state instead of spawning a second .plan.md file.
func MarkPlanEmitted(conversationID string) {
	if conversationID == "" {
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.planEmitted[conversationID] = true
}

func (s *sessionStore) Put(sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess.RequestID != "" {
		s.byID[sess.RequestID] = sess
	}
	if conv := sess.ConversationID; conv != "" {
		if sess.UserText != "" || s.byConv[conv] == nil {
			s.byConv[conv] = sess
		}
	}
	s.lastPut = sess
	if sess.UserText != "" {
		s.lastConv = sess
		s.lastConvAt = time.Now()
	}
}

func (s *sessionStore) Get(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sess := s.byID[id]; sess != nil {
		return sess
	}
	if sess := s.byConv[id]; sess != nil {
		return sess
	}
	// Retried RunSSE with a request_id we've already dropped: re-attach to
	// its original conversation rather than falling off to lastConv (which
	// could now point at a different chat entirely).
	if de, ok := s.droppedIDConv[id]; ok {
		if sess := s.byConv[de.convID]; sess != nil {
			return sess
		}
	}
	return nil
}

func (s *sessionStore) textfulSessionForConversation(conversationID string) *Session {
	if conversationID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess := s.byConv[conversationID]
	if sess == nil || sess.UserText == "" {
		return nil
	}
	return sess
}

func (s *sessionStore) Drop(id string) {
	s.mu.Lock()
	if sess := s.byID[id]; sess != nil {
		delete(s.byID, id)
		// Remember the conversation this request_id belonged to so any
		// later retry/reconnect finds its way back to the correct chat
		// instead of the most-recent lastConv (which might be a SIBLING
		// conversation the user has since switched to).
		if conv := sess.ConversationID; conv != "" {
			if s.droppedIDConv == nil {
				s.droppedIDConv = map[string]droppedEntry{}
			}
			s.droppedIDConv[id] = droppedEntry{convID: conv, droppedAt: time.Now()}
		}
		// Keep byConv mapping alive so follow-up RunSSE requests for the
		// same conversation (different request_id) can still find context.
		if s.lastPut == sess {
			s.lastPut = nil
		}
	}
	s.mu.Unlock()
}

func PutSession(s *Session)         { store.Put(s) }
func GetSession(id string) *Session { return store.Get(id) }
func DropSession(id string)         { store.Drop(id) }

// HistoryFor returns a copy of the stored turns for a conversation_id so
// the caller can iterate without holding the store lock. Callers are free
// to mutate the returned slice; the internal slice is not shared.
func HistoryFor(conversationID string) []*ConvTurn {
	store.mu.RLock()
	defer store.mu.RUnlock()
	src := store.history[conversationID]
	if len(src) == 0 {
		logutil.Debug("history: no turns found", "conversationID", conversationID, "storeKeys", len(store.history))
		return nil
	}
	logutil.Debug("history: turns found", "conversationID", conversationID, "turns", len(src))
	out := make([]*ConvTurn, len(src))
	copy(out, src)
	return out
}

// defaultMaxTurnsPerConversation caps in-memory turns per conversation.
// Beyond this the oldest pair is dropped from the live window, but the
// disk log keeps every turn intact.
const defaultMaxTurnsPerConversation = 50

// maxTurnsPerConversation can be overridden at init time (e.g. from config).
var maxTurnsPerConversation = defaultMaxTurnsPerConversation

// SetMaxTurnsPerConversation overrides the in-memory turn cap.
func SetMaxTurnsPerConversation(n int) {
	if n > 0 {
		maxTurnsPerConversation = n
	}
}

// RecordTurn appends a completed (user, assistant) pair to the conversation
// history (in-memory + on-disk). Called after RunSSE finishes streaming so
// we have the full assistant reply. In-memory is capped at
// maxTurnsPerConversation turns per conversation to avoid unbounded memory
// growth — beyond that we drop the oldest pair from the live window, but the
// disk log keeps every turn intact so a future restart with a bigger cap can
// replay everything.
//
// When art != nil the caller supplies the exact request/response bytes so
// they get written into turns/NNNNNN/ alongside conversation.json — this
// matches the working app's artifact layout.
//
// messages (when non-nil) is the OpenAI-format message list the turn added
// to the prompt. Persisting it lets buildMessageHistory replay intermediate
// tool_calls + tool results on the next turn instead of losing them to the
// User/Assistant-string fallback — which is what used to trigger the
// "reconnecting → re-run every tool from scratch" loop.
func RecordTurn(conversationID, requestID, user, assistant, mode string, art *turnArtifacts, messages []StoredMessage) {
	if conversationID == "" || user == "" {
		return
	}
	logutil.Debug("history: record turn", "conversationID", conversationID, "requestID", requestID, "userLen", len(user), "assistantLen", len(assistant), "msgsLen", len(messages))
	turn := &ConvTurn{User: user, Assistant: assistant, Messages: messages}
	store.mu.Lock()
	turns := store.history[conversationID]
	turns = append(turns, turn)
	if len(turns) > maxTurnsPerConversation {
		turns = turns[len(turns)-maxTurnsPerConversation:]
	}
	store.history[conversationID] = turns
	store.mu.Unlock()
	persistTurn(conversationID, requestID, user, assistant, mode, art, messages)
}

// modeString maps the proto AgentMode to the short label we record in
// conversation.json / stats panel. Kept here so every call site agrees.
func modeString(m agentv1.AgentMode) string {
	switch m {
	case agentv1.AgentMode_AGENT_MODE_AGENT:
		return "agent"
	case agentv1.AgentMode_AGENT_MODE_ASK:
		return "ask"
	case agentv1.AgentMode_AGENT_MODE_PLAN:
		return "plan"
	case agentv1.AgentMode_AGENT_MODE_DEBUG:
		return "debug"
	default:
		return "agent"
	}
}

// WaitForSession blocks until a session with usable UserText exists for id
// (or until the context expires). Cursor occasionally fires RunSSE before
// the matching BidiAppend lands, AND continuation rounds ship a fresh
// request_id with no user-facing text of their own — in that case we clone
// the last session that had UserText so the model keeps working on the
// same conversation instead of crashing with "no session/user text".
//
// Priority on exit:
//  1. A session with matching id AND non-empty UserText (happy path).
//  2. A lastConv clone (continuation rounds or reconnect storms — without
//     this we'd reject the retry and Cursor shows "Internal Error").
//  3. Any session at all (heartbeat skeleton) — last resort; caller still
//     decides whether UserText is usable.
func WaitForSession(ctx context.Context, id string) *Session {
	if id == "" {
		logutil.Debug("session: WaitForSession id empty, returning nil")
		return nil
	}
	deadline := time.Now().Add(5 * time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	for {
		if sess := GetSession(id); sess != nil {
			if sess.UserText != "" {
				logutil.Debug("session: WaitForSession found session with UserText", "id", id, "conversationID", sess.ConversationID)
				return sess
			}
			if fallback := store.textfulSessionForConversation(sess.ConversationID); fallback != nil {
				clone := *fallback
				clone.RequestID = id
				clone.ConversationID = sess.ConversationID
				clone.McpMap = nil
				PutSession(&clone)
				logutil.Debug("session: WaitForSession cloned textful fallback", "id", id, "conversationID", sess.ConversationID)
				return &clone
			}
		}
		if time.Now().After(deadline) {
			sess := GetSession(id)
			if sess != nil && sess.UserText != "" {
				return sess
			}
			// Timeout - try lastConv fallback before giving up. Cursor
			// sometimes fires RunSSE for a continuation round or prewarm
			// that carries no UserText of its own; lastConv gives us the
			// most recent session that did have text so the model can
			// keep working on the same conversation.
			if fallback := store.lastConvSafeFallback(); fallback != nil {
				clone := *fallback
				clone.RequestID = id
				if sess != nil {
					clone.ConversationID = sess.ConversationID
				}
				clone.McpMap = nil
				PutSession(&clone)
				logutil.Debug("session: WaitForSession timed out, cloned lastConv fallback", "id", id, "conversationID", clone.ConversationID)
				return &clone
			}
			logutil.Warn("session: WaitForSession timed out", "id", id, "hasSession", sess != nil)
			return sess
		}
		select {
		case <-ctx.Done():
			sess := GetSession(id)
			if sess != nil && sess.UserText != "" {
				return sess
			}
			if sess != nil {
				if fallback := store.textfulSessionForConversation(sess.ConversationID); fallback != nil {
					clone := *fallback
					clone.RequestID = id
					clone.ConversationID = sess.ConversationID
					clone.McpMap = nil
					PutSession(&clone)
					logutil.Debug("session: WaitForSession ctx-cancelled, cloned textful fallback", "id", id, "conversationID", sess.ConversationID)
					return &clone
				}
			}
			logutil.Warn("session: WaitForSession ctx-cancelled, returning nil", "id", id)
			return nil
		case <-time.After(25 * time.Millisecond):
		}
	}
}


// sessionTTL controls how long droppedIDConv entries survive before sweep
// reclaims them. 2 hours is long enough that Cursor's reconnect/retry storms
// (which can last several minutes) find their original conversation, while
// still bounding memory for idle sessions.
const sessionTTL = 2 * time.Hour

func (s *sessionStore) sweep() {
	for range DefaultDeps.sweepTicker(5 * time.Minute) {
		cutoff := time.Now().Add(-sessionTTL)
		s.mu.Lock()
		for id, de := range s.droppedIDConv {
			if de.droppedAt.Before(cutoff) {
				delete(s.droppedIDConv, id)
			}
		}
		for convID, turns := range s.history {
			if len(turns) == 0 {
				delete(s.history, convID)
			}
		}
		// Clean up byID entries that have no corresponding byConv (orphaned).
		for id, sess := range s.byID {
			if sess.ConversationID == "" {
				continue
			}
			if s.byConv[sess.ConversationID] != sess {
				delete(s.byID, id)
			}
		}
		s.mu.Unlock()
	}
}
