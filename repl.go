package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/glycerine/zygomys/v9/zygo"
)

// errIncomplete means the submitted source is syntactically incomplete and
// the REPL should keep collecting input.
var errIncomplete = errors.New("incomplete expression")

// replResult is the complete outcome of one successful eval.
type replResult struct {
	// Output is everything the zygo program wrote with print/println/printf.
	Output string
	// Value is the formatted result value. It is empty for SexpNull.
	Value string
	// Err is an evaluation error, if any.
	Err error
	// ErrText is the human-readable stack trace for Err.
	ErrText string
}

// repl owns the zygo environment and adapts it for use inside the TUI.
//
// zygo's own REPL reads directly from stdin and writes to stdout, so instead we
// use EvalString and provide our own print functions that write into a buffer.
type repl struct {
	env   *zygo.Zlisp
	probe *zygo.Parser

	mu  sync.Mutex
	out strings.Builder
}

func newRepl() *repl {
	env := zygo.NewZlisp()
	env.StandardSetup()

	r := &repl{
		env:   env,
		probe: env.NewParser(),
	}
	r.installPrinters()
	return r
}

func (r *repl) Close() error {
	_ = r.probe.Stop()
	return r.env.Close()
}

// isIncomplete parses the source without evaluating it. This mirrors what
// zygo's own REPL does so constructs like "for ... {" can span several lines.
func (r *repl) isIncomplete(src string) (incomplete bool) {
	defer func() {
		// A malformed expression should never take down the whole TUI. If the
		// parser panics, let evaluation attempt it and report the error.
		if recover() != nil {
			incomplete = false
		}
	}()

	r.probe.ResetAddNewInput(bytes.NewBufferString(src + "\n"))
	_, err := r.probe.ParseTokens()
	return errors.Is(err, zygo.ErrMoreInputNeeded) ||
		errors.Is(err, zygo.UnexpectedEnd) ||
		errors.Is(err, zygo.ResetRequested)
}

// Eval evaluates source, which may contain multiple lines.
//
// It returns errIncomplete when more input is required. Evaluation errors are
// reported in replResult.Err so any output produced before the error can still
// be displayed.
func (r *repl) Eval(src string) (res replResult, err error) {
	if strings.TrimSpace(src) == "" {
		return replResult{}, nil
	}
	if r.isIncomplete(src) {
		return replResult{}, errIncomplete
	}

	defer func() {
		if p := recover(); p != nil {
			r.env.Clear()
			res.Output = r.takeOutput()
			res.Err = fmt.Errorf("zygo panic: %v", p)
			res.ErrText = res.Err.Error()
		}
	}()

	r.mu.Lock()
	r.out.Reset()
	r.mu.Unlock()

	// Wrapping in braces is what zygo's REPL does to enable infix syntax
	// such as "x := 1" and "if cond { ... }". Lisp forms still work inside it.
	value, evalErr := r.env.EvalString("{" + src + "\n}\n")

	res.Output = r.takeOutput()
	if evalErr != nil {
		res.Err = evalErr
		res.ErrText = cleanErrText(r.env.GetStackTrace(evalErr))
		if res.ErrText == "" {
			res.ErrText = evalErr.Error()
		}
		r.env.Clear()
		return res, nil
	}

	if value != nil && value != zygo.SexpNull {
		res.Value = value.SexpString(nil)
	}
	return res, nil
}

func (r *repl) write(s string) {
	r.mu.Lock()
	r.out.WriteString(s)
	r.mu.Unlock()
}

// cleanErrText removes internal parser details that zygo adds to syntax errors.
func cleanErrText(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		if j := strings.Index(line, " (LoadStream err="); j >= 0 {
			line = line[:j]
		}
		for {
			j := strings.Index(line, " (TokenType: ")
			if j < 0 {
				break
			}
			k := strings.Index(line[j:], ")")
			if k < 0 {
				break
			}
			line = line[:j] + line[j+k+1:]
		}
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (r *repl) takeOutput() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := r.out.String()
	r.out.Reset()
	return strings.TrimRight(s, "\n")
}

// installPrinters replaces zygo's print functions, which write straight to
// stdout, with versions that capture their output for the output panel.
func (r *repl) installPrinters() {
	r.env.AddGlobal("print", zygo.MakeUserFunction("print", func(_ *zygo.Zlisp, _ string, args []zygo.Sexp) (zygo.Sexp, error) {
		var b strings.Builder
		for _, a := range args {
			b.WriteString(plainString(a))
		}
		r.write(b.String())
		return zygo.SexpNull, nil
	}))

	r.env.AddGlobal("println", zygo.MakeUserFunction("println", func(_ *zygo.Zlisp, _ string, args []zygo.Sexp) (zygo.Sexp, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = plainString(a)
		}
		r.write(strings.Join(parts, " ") + "\n")
		return zygo.SexpNull, nil
	}))

	r.env.AddGlobal("printf", zygo.MakeUserFunction("printf", func(_ *zygo.Zlisp, _ string, args []zygo.Sexp) (zygo.Sexp, error) {
		if len(args) == 0 {
			return zygo.SexpNull, zygo.WrongNargs
		}
		format := plainString(args[0])
		if len(args) == 1 {
			r.write(format)
			return zygo.SexpNull, nil
		}
		values := make([]any, 0, len(args)-1)
		for _, a := range args[1:] {
			values = append(values, goValue(a))
		}
		r.write(fmt.Sprintf(format, values...))
		return zygo.SexpNull, nil
	}))
}

// plainString formats a value the way a human expects print/println to.
func plainString(s zygo.Sexp) string {
	if str, ok := s.(*zygo.SexpStr); ok {
		return str.S
	}
	return s.SexpString(nil)
}

func goValue(s zygo.Sexp) any {
	switch v := s.(type) {
	case *zygo.SexpInt:
		return v.Val
	case *zygo.SexpUint64:
		return v.Val
	case *zygo.SexpFloat:
		return v.Val
	case *zygo.SexpBool:
		return v.Val
	case *zygo.SexpChar:
		return v.Val
	case *zygo.SexpStr:
		return v.S
	case *zygo.SexpTime:
		return v.Tm
	default:
		return plainString(s)
	}
}
