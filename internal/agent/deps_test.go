package agent

import (
	"testing"
	"time"
)

func TestNewAgentDeps(t *testing.T) {
	d := NewAgentDeps()
	if d == nil {
		t.Fatal("NewAgentDeps returned nil")
	}
	if d.Pending == nil {
		t.Error("Pending map not initialized")
	}
	if d.ExecIDAlias == nil {
		t.Error("ExecIDAlias map not initialized")
	}
	if d.SeqAlias == nil {
		t.Error("SeqAlias map not initialized")
	}
	if d.ShellAccum == nil {
		t.Error("ShellAccum map not initialized")
	}
	if d.PendingInteraction == nil {
		t.Error("PendingInteraction map not initialized")
	}
	if d.CompactionStates == nil {
		t.Error("CompactionStates map not initialized")
	}
	if d.sweepCtx == nil {
		t.Error("sweepCtx not initialized")
	}
	if d.sweepCancel == nil {
		t.Error("sweepCancel not initialized")
	}
}

func TestAgentDepsShutdown(t *testing.T) {
	d := NewAgentDeps()
	d.Shutdown()
	select {
	case <-d.sweepCtx.Done():
	default:
		t.Error("sweepCtx should be done after Shutdown")
	}
}

func TestAgentDepsSweepTicker(t *testing.T) {
	d := NewAgentDeps()
	defer d.Shutdown()
	ch := d.sweepTicker(10 * time.Millisecond)
	select {
	case <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Error("sweepTicker should emit ticks")
	}
	d.Shutdown()
	select {
	case <-ch:
		t.Error("sweepTicker should stop after Shutdown")
	default:
	}
}

func TestAgentDepsIsolation(t *testing.T) {
	d1 := NewAgentDeps()
	d2 := NewAgentDeps()
	d1.ExecIDAlias["test"] = aliasEntry{}
	if _, ok := d2.ExecIDAlias["test"]; ok {
		t.Error("different AgentDeps instances should have independent maps")
	}
	d1.Shutdown()
	select {
	case <-d2.sweepCtx.Done():
		t.Error("d2 should not be affected by d1.Shutdown()")
	default:
	}
	d2.Shutdown()
}

func TestGetCompactionStateWithDeps(t *testing.T) {
	d := NewAgentDeps()
	cs1 := getCompactionState(d, "conv-1")
	if cs1 == nil {
		t.Fatal("getCompactionState returned nil")
	}
	cs2 := getCompactionState(d, "conv-1")
	if cs1 != cs2 {
		t.Error("same conversation ID should return same CompactionState")
	}
	cs3 := getCompactionState(d, "conv-2")
	if cs1 == cs3 {
		t.Error("different conversation IDs should return different CompactionState")
	}
}

func TestGetCompactionStateIsolation(t *testing.T) {
	d1 := NewAgentDeps()
	d2 := NewAgentDeps()
	cs1 := getCompactionState(d1, "conv-1")
	cs2 := getCompactionState(d2, "conv-1")
	if cs1 == cs2 {
		t.Error("different deps instances should have independent CompactionStates")
	}
}
