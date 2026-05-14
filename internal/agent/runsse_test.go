package agent

import (
	"testing"
)

func TestResolveAdapterForModel_Empty(t *testing.T) {
	_, ok := ResolveAdapterForModel(nil, "")
	if ok {
		t.Fatal("expected false for nil adapters")
	}
	_, ok = ResolveAdapterForModel([]AdapterTarget{}, "")
	if ok {
		t.Fatal("expected false for empty adapters")
	}
}

func TestResolveAdapterForModel_FirstFallback(t *testing.T) {
	adapters := []AdapterTarget{
		{Model: "gpt-4o", StableID: "stable-1"},
		{Model: "claude-3", StableID: "stable-2"},
	}
	target, ok := ResolveAdapterForModel(adapters, "")
	if !ok {
		t.Fatal("expected ok")
	}
	if target.Model != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %s", target.Model)
	}
}

func TestResolveAdapterForModel_ByModel(t *testing.T) {
	adapters := []AdapterTarget{
		{Model: "gpt-4o", StableID: "stable-1"},
		{Model: "claude-3", StableID: "stable-2"},
	}
	target, ok := ResolveAdapterForModel(adapters, "claude-3")
	if !ok {
		t.Fatal("expected ok")
	}
	if target.Model != "claude-3" {
		t.Fatalf("expected claude-3, got %s", target.Model)
	}
}

func TestResolveAdapterForModel_ByStableID(t *testing.T) {
	adapters := []AdapterTarget{
		{Model: "gpt-4o", StableID: "stable-1"},
		{Model: "claude-3", StableID: "stable-2"},
	}
	target, ok := ResolveAdapterForModel(adapters, "stable-2")
	if !ok {
		t.Fatal("expected ok")
	}
	if target.Model != "claude-3" {
		t.Fatalf("expected claude-3, got %s", target.Model)
	}
}

func TestResolveAdapterForModel_ByDisplayName(t *testing.T) {
	adapters := []AdapterTarget{
		{Model: "gpt-4o", StableID: "stable-1", DisplayName: "GPT-4o"},
		{Model: "claude-3", StableID: "stable-2", DisplayName: "Claude"},
	}
	target, ok := ResolveAdapterForModel(adapters, "Claude")
	if !ok {
		t.Fatal("expected ok")
	}
	if target.Model != "claude-3" {
		t.Fatalf("expected claude-3, got %s", target.Model)
	}
}

func TestResolveAdapterForModel_CaseInsensitive(t *testing.T) {
	adapters := []AdapterTarget{
		{Model: "gpt-4o", StableID: "stable-1"},
	}
	target, ok := ResolveAdapterForModel(adapters, "GPT-4O")
	if !ok {
		t.Fatal("expected ok for case-insensitive match")
	}
	if target.Model != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %s", target.Model)
	}
}
