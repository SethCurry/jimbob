package main

import (
	"strings"
	"testing"
)

func newTestModel(t *testing.T) *model {
	t.Helper()

	m := newModel()
	t.Cleanup(func() { _ = m.repl.Close() })
	m.setSize(80, 24)
	return m
}

func submitLine(m *model, line string) {
	m.input.SetValue(line)
	m.submit()
}

func TestModelEvaluatesAndDisplaysResult(t *testing.T) {
	m := newTestModel(t)

	submitLine(m, "1 + 2")

	if len(m.pending) != 0 {
		t.Fatalf("pending = %v, want empty", m.pending)
	}
	if got := m.input.Prompt; got != primaryPrompt {
		t.Fatalf("prompt = %q, want %q", got, primaryPrompt)
	}

	var transcript strings.Builder
	for _, e := range m.entries {
		transcript.WriteString(e.text)
		transcript.WriteByte('\n')
	}
	if !strings.Contains(transcript.String(), "1 + 2") {
		t.Fatalf("transcript does not contain the submitted line:\n%s", transcript.String())
	}
	if !strings.Contains(transcript.String(), "3") {
		t.Fatalf("transcript does not contain the result:\n%s", transcript.String())
	}
}

func TestModelMultilineAndHistory(t *testing.T) {
	m := newTestModel(t)

	submitLine(m, "for i := 0; i < 2; i++ {")
	if len(m.pending) != 1 {
		t.Fatalf("pending = %v, want one line", m.pending)
	}
	if got := m.input.Prompt; got != continuationPrompt {
		t.Fatalf("prompt = %q, want %q", got, continuationPrompt)
	}

	submitLine(m, "  (println i)")
	submitLine(m, "}")

	if len(m.pending) != 0 {
		t.Fatalf("pending = %v, want empty", m.pending)
	}
	if got := m.input.Prompt; got != primaryPrompt {
		t.Fatalf("prompt = %q, want %q", got, primaryPrompt)
	}

	wantHistory := "for i := 0; i < 2; i++ {   (println i) }"
	if n := len(m.history); n != 1 || m.history[0] != wantHistory {
		t.Fatalf("history = %v, want [%q]", m.history, wantHistory)
	}

	m.historyPrev()
	if got := m.input.Value(); got != wantHistory {
		t.Fatalf("recalled input = %q, want %q", got, wantHistory)
	}
}

func TestModelCancelPending(t *testing.T) {
	m := newTestModel(t)

	submitLine(m, "(defn f [x]")
	if len(m.pending) != 1 {
		t.Fatalf("pending = %v, want one line", m.pending)
	}

	m.cancel()

	if len(m.pending) != 0 {
		t.Fatalf("pending = %v, want empty", m.pending)
	}
	if got := m.input.Prompt; got != primaryPrompt {
		t.Fatalf("prompt = %q, want %q", got, primaryPrompt)
	}
	if len(m.history) != 0 {
		t.Fatalf("history = %v, want empty", m.history)
	}
}

func TestModelCommands(t *testing.T) {
	m := newTestModel(t)

	submitLine(m, ".help")
	foundHelp := false
	for _, e := range m.entries {
		if strings.Contains(e.text, "commands") {
			foundHelp = true
			break
		}
	}
	if !foundHelp {
		t.Fatal(".help did not add help text to the transcript")
	}

	submitLine(m, ".clear")
	if len(m.entries) != 1 || !strings.Contains(m.entries[0].text, "output cleared") {
		t.Fatalf("entries after .clear = %v", m.entries)
	}
}
