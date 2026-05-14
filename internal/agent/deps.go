package agent

import (
	"context"
	"sync"
	"time"
)

// AgentDeps aggregates all package-level mutable state used by the agent
// handlers. Defining it explicitly makes state testable and prepares for
// future dependency-injection patterns.
type AgentDeps struct {
	// Tool execution: tool_call_id → result channel.
	PendingMu sync.Mutex
	Pending   map[string]*pendingResult

	// exec_id → aliasEntry (correlating Cursor's exec responses).
	ExecIDAliasMu sync.Mutex
	ExecIDAlias  map[string]aliasEntry

	// seq → aliasEntry (shellAccum collision avoidance).
	SeqAliasMu sync.Mutex
	SeqAlias   map[uint32]aliasEntry

	// seq → shell accumulator.
	ShellAccumMu sync.RWMutex
	ShellAccum   map[uint32]*shellAccumState

	// seq → interactionEntry (SwitchMode and other interactions).
	PendingInteractionMu sync.RWMutex
	PendingInteraction  map[uint32]interactionEntry

	// Conversation history root.
	HistoryDirMu sync.RWMutex
	HistoryDir  string

	// Compaction state per conversation.
	CompactionStatesMu sync.RWMutex
	CompactionStates   map[string]*CompactionState

	// Sweep lifecycle: cancel this to stop background goroutines.
	sweepCtx    context.Context
	sweepCancel context.CancelFunc
}

// DefaultDeps is the global deps instance used by all agent handlers unless
// explicitly overridden (e.g. in tests).
var DefaultDeps = newAgentDeps()

func newAgentDeps() *AgentDeps {
	ctx, cancel := context.WithCancel(context.Background())
	return &AgentDeps{
		Pending:             make(map[string]*pendingResult),
		ExecIDAlias:         make(map[string]aliasEntry),
		SeqAlias:            make(map[uint32]aliasEntry),
		ShellAccum:         make(map[uint32]*shellAccumState),
		PendingInteraction: make(map[uint32]interactionEntry),
		CompactionStates:   make(map[string]*CompactionState),
		sweepCtx:           ctx,
		sweepCancel:        cancel,
	}
}

// Shutdown stops all background sweep goroutines. After calling Shutdown,
// no new sweep ticks will fire; existing in-flight sweeps finish their
// current iteration.
func (d *AgentDeps) Shutdown() {
	d.sweepCancel()
}

// sweepTicker returns a channel that emits ticks every interval until
// the deps sweep context is cancelled.
func (d *AgentDeps) sweepTicker(interval time.Duration) <-chan time.Time {
	t := time.NewTicker(interval)
	go func() {
		<-d.sweepCtx.Done()
		t.Stop()
	}()
	return t.C
}