package main

import (
	"errors"
	"strings"
	"testing"
)

func TestReplInfixAndLispForms(t *testing.T) {
	r := newRepl()
	defer r.Close()

	res, err := r.Eval("x := 10")
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Err != nil {
		t.Fatalf("eval error: %v", res.Err)
	}
	if res.Value != "10" {
		t.Fatalf("value = %q, want %q", res.Value, "10")
	}

	res, err = r.Eval("x * 2")
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Value != "20" {
		t.Fatalf("value = %q, want %q", res.Value, "20")
	}

	res, err = r.Eval(`(println "hello" 42)`)
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Err != nil {
		t.Fatalf("eval error: %v", res.Err)
	}
	if res.Output != "hello 42" {
		t.Fatalf("output = %q, want %q", res.Output, "hello 42")
	}
	if res.Value != "" {
		t.Fatalf("value = %q, want empty", res.Value)
	}
}

func TestReplIncompleteMultiline(t *testing.T) {
	r := newRepl()
	defer r.Close()

	if _, err := r.Eval("for i := 0; i < 3; i++ {"); !errors.Is(err, errIncomplete) {
		t.Fatalf("expected errIncomplete, got %v", err)
	}
	if _, err := r.Eval("(defn double [x]"); !errors.Is(err, errIncomplete) {
		t.Fatalf("expected errIncomplete, got %v", err)
	}

	src := "for i := 0; i < 3; i++ {\n  (println i)\n}"
	res, err := r.Eval(src)
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Err != nil {
		t.Fatalf("eval error: %v", res.Err)
	}
	if res.Output != "0\n1\n2" {
		t.Fatalf("output = %q, want %q", res.Output, "0\n1\n2")
	}

	res, err = r.Eval("(defn double [x]\n  (* x 2))")
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Err != nil {
		t.Fatalf("eval error: %v", res.Err)
	}

	res, err = r.Eval("(double 21)")
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Value != "42" {
		t.Fatalf("value = %q, want %q", res.Value, "42")
	}
}

func TestReplPrintf(t *testing.T) {
	r := newRepl()
	defer r.Close()

	res, err := r.Eval(`(printf "%d-%s" 42 "ok")`)
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Err != nil {
		t.Fatalf("eval error: %v", res.Err)
	}
	if res.Output != "42-ok" {
		t.Fatalf("output = %q, want %q", res.Output, "42-ok")
	}
}

func TestReplEvalError(t *testing.T) {
	r := newRepl()
	defer r.Close()

	res, err := r.Eval("y := 7")
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Err != nil {
		t.Fatalf("eval error: %v", res.Err)
	}

	res, err = r.Eval(`(+ 1 "not-a-number")`)
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Err == nil {
		t.Fatal("expected an eval error")
	}
	if !strings.Contains(res.ErrText, "error in") {
		t.Fatalf("error text = %q, want a zygo stack trace", res.ErrText)
	}

	// The environment should still be usable after an error.
	res, err = r.Eval("1 + 2")
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Err != nil {
		t.Fatalf("eval error after recovery: %v", res.Err)
	}
	if res.Value != "3" {
		t.Fatalf("value = %q, want %q", res.Value, "3")
	}

	// Clearing the error state must not discard user globals.
	res, err = r.Eval("y")
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Err != nil {
		t.Fatalf("eval error: %v", res.Err)
	}
	if res.Value != "7" {
		t.Fatalf("value = %q, want %q", res.Value, "7")
	}

	// Internal parser details should be stripped from syntax errors.
	res, err = r.Eval("(+ 1))")
	if err != nil {
		t.Fatalf("Eval returned %v", err)
	}
	if res.Err == nil {
		t.Fatal("expected a syntax error")
	}
	if strings.Contains(res.ErrText, "LoadStream") || strings.Contains(res.ErrText, "TokenType") {
		t.Fatalf("error text still contains parser internals: %q", res.ErrText)
	}
}
