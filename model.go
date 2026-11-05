package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	primaryPrompt      = "zygo> "
	continuationPrompt = " ...> "
)

type entryKind int

const (
	entryBanner entryKind = iota
	entryPrompt
	entryOutput
	entryResult
	entryError
	entryNote
)

type entry struct {
	kind   entryKind
	prompt string
	text   string
}

type styles struct {
	outputBox  lipgloss.Style
	inputBox   lipgloss.Style
	inputBoxOn lipgloss.Style

	prompt     lipgloss.Style
	contPrompt lipgloss.Style
	result     lipgloss.Style
	error      lipgloss.Style
	note       lipgloss.Style
	banner     lipgloss.Style
	help       lipgloss.Style
}

func newStyles() styles {
	accent := lipgloss.AdaptiveColor{Light: "26", Dark: "39"}
	pink := lipgloss.AdaptiveColor{Light: "162", Dark: "213"}
	border := lipgloss.AdaptiveColor{Light: "250", Dark: "238"}
	muted := lipgloss.AdaptiveColor{Light: "243", Dark: "244"}
	danger := lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	text := lipgloss.AdaptiveColor{Light: "235", Dark: "252"}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1)

	return styles{
		outputBox:  box,
		inputBox:   box,
		inputBoxOn: box.BorderForeground(accent),
		prompt:     lipgloss.NewStyle().Foreground(pink).Bold(true),
		contPrompt: lipgloss.NewStyle().Foreground(muted),
		result:     lipgloss.NewStyle().Foreground(text),
		error:      lipgloss.NewStyle().Foreground(danger),
		note:       lipgloss.NewStyle().Foreground(muted).Italic(true),
		banner:     lipgloss.NewStyle().Foreground(accent).Bold(true),
		help:       lipgloss.NewStyle().Foreground(muted),
	}
}

type model struct {
	width  int
	height int

	output viewport.Model
	input  textinput.Model
	repl   *repl

	entries []entry
	pending []string

	history   []string
	histIdx   int
	histDraft string

	contentWidth int
	follow       bool
	ready        bool
	styles       styles
}

func newModel() *model {
	st := newStyles()

	in := textinput.New()
	in.Prompt = primaryPrompt
	in.PromptStyle = st.prompt
	in.Placeholder = "type a zygo expression…"
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "240"})
	in.CharLimit = 0
	in.Focus()

	out := viewport.New(0, 0)
	out.Style = st.outputBox

	m := &model{
		output: out,
		input:  in,
		repl:   newRepl(),
		follow: true,
		styles: st,
	}

	m.entries = append(m.entries, entry{
		kind: entryBanner,
		text: "jimbob · zygo REPL\nevaluate Lisp or infix expressions below · .help for commands",
	})
	m.history = nil
	m.histIdx = 0

	return m
}

func (m *model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			m.cancel()
			return m, nil

		case "enter":
			return m, m.submit()

		case "up":
			m.historyPrev()
			return m, nil

		case "down":
			m.historyNext()
			return m, nil

		case "pgup":
			m.output.PageUp()
			m.follow = m.output.AtBottom()
			return m, nil

		case "pgdown":
			m.output.PageDown()
			m.follow = m.output.AtBottom()
			return m, nil

		case "ctrl+l":
			m.clearOutput()
			return m, nil

		case "ctrl+d":
			if m.input.Value() == "" && len(m.pending) == 0 {
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	if !m.ready {
		return "starting jimbob…"
	}

	inputWidth := m.width - 2
	if inputWidth < 1 {
		inputWidth = 1
	}

	box := m.styles.inputBox
	if len(m.pending) > 0 {
		box = m.styles.inputBoxOn
	}

	input := box.Width(inputWidth).Render(m.input.View())
	help := m.helpView()

	return lipgloss.JoinVertical(lipgloss.Left, m.output.View(), input, help)
}

func (m *model) setSize(width, height int) {
	m.width = width
	m.height = height

	const inputHeight = 3
	const helpHeight = 1

	outputHeight := height - inputHeight - helpHeight
	if outputHeight < 1 {
		outputHeight = 1
	}

	m.output.Width = width
	m.output.Height = outputHeight

	m.contentWidth = width - 4
	if m.contentWidth < 1 {
		m.contentWidth = 1
	}

	m.resizeInput()
	m.ready = true
	m.refreshOutput()
}

func (m *model) resizeInput() {
	if m.width <= 0 {
		return
	}
	width := m.width - 4 - lipgloss.Width(m.input.Prompt)
	if width < 1 {
		width = 1
	}
	m.input.Width = width
}

func (m *model) submit() tea.Cmd {
	line := m.input.Value()
	if strings.TrimSpace(line) == "" {
		return nil
	}

	m.follow = true

	if len(m.pending) == 0 {
		m.entries = append(m.entries, entry{kind: entryPrompt, prompt: primaryPrompt, text: line})
		if cmd, handled := m.runCommand(strings.TrimSpace(line)); handled {
			m.addHistory(line)
			m.input.Reset()
			m.input.CursorEnd()
			m.refreshOutput()
			return cmd
		}
	} else {
		m.entries = append(m.entries, entry{kind: entryPrompt, prompt: continuationPrompt, text: line})
	}

	m.pending = append(m.pending, line)
	m.input.Reset()
	m.input.CursorEnd()

	result, err := m.repl.Eval(strings.Join(m.pending, "\n"))
	if errors.Is(err, errIncomplete) {
		m.updatePrompt()
		m.refreshOutput()
		return nil
	}

	// History recall is single-line, so collapse a completed multi-line
	// expression into one entry rather than recording each line separately.
	m.addHistory(strings.Join(m.pending, " "))
	m.pending = nil
	m.updatePrompt()

	if err != nil {
		m.entries = append(m.entries, entry{kind: entryError, text: fmt.Sprintf("internal error: %v", err)})
		m.refreshOutput()
		return nil
	}

	if result.Output != "" {
		m.entries = append(m.entries, entry{kind: entryOutput, text: result.Output})
	}
	if result.Value != "" {
		m.entries = append(m.entries, entry{kind: entryResult, text: result.Value})
	}
	if result.Err != nil {
		text := result.ErrText
		if text == "" {
			text = result.Err.Error()
		}
		m.entries = append(m.entries, entry{kind: entryError, text: text})
	}

	m.refreshOutput()
	return nil
}

// runCommand handles app-level dot commands. It returns handled=true when the
// line should not be sent to the zygo interpreter.
func (m *model) runCommand(line string) (tea.Cmd, bool) {
	switch line {
	case ".help":
		m.entries = append(m.entries, entry{kind: entryNote, text: helpText})
		return nil, true

	case ".clear":
		m.clearOutput()
		return nil, true

	case ".quit":
		return tea.Quit, true
	}
	return nil, false
}

func (m *model) clearOutput() {
	m.entries = []entry{{
		kind: entryNote,
		text: "output cleared · .help for commands",
	}}
	m.follow = true
	m.refreshOutput()
}

func (m *model) cancel() {
	if len(m.pending) > 0 {
		m.pending = nil
		m.updatePrompt()
		m.entries = append(m.entries, entry{kind: entryNote, text: "cancelled"})
		m.follow = true
		m.refreshOutput()
	}

	m.input.Reset()
	m.input.CursorEnd()
}

func (m *model) updatePrompt() {
	if len(m.pending) > 0 {
		m.input.Prompt = continuationPrompt
		m.input.PromptStyle = m.styles.contPrompt
	} else {
		m.input.Prompt = primaryPrompt
		m.input.PromptStyle = m.styles.prompt
	}
	m.resizeInput()
}

func (m *model) addHistory(line string) {
	if line == "" {
		return
	}
	if n := len(m.history); n == 0 || m.history[n-1] != line {
		m.history = append(m.history, line)
	}
	m.histIdx = len(m.history)
	m.histDraft = ""
}

func (m *model) historyPrev() {
	if len(m.pending) > 0 || len(m.history) == 0 {
		return
	}
	if m.histIdx == len(m.history) {
		m.histDraft = m.input.Value()
	}
	if m.histIdx > 0 {
		m.histIdx--
	}
	m.input.SetValue(m.history[m.histIdx])
	m.input.CursorEnd()
}

func (m *model) historyNext() {
	if len(m.pending) > 0 || len(m.history) == 0 {
		return
	}
	if m.histIdx < len(m.history)-1 {
		m.histIdx++
		m.input.SetValue(m.history[m.histIdx])
	} else if m.histIdx == len(m.history)-1 {
		m.histIdx = len(m.history)
		m.input.SetValue(m.histDraft)
	}
	m.input.CursorEnd()
}

func (m *model) refreshOutput() {
	if !m.ready {
		return
	}

	var b strings.Builder
	for i, e := range m.entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.renderEntry(e))
	}

	m.output.SetContent(b.String())
	if m.follow {
		m.output.GotoBottom()
	}
}

func (m *model) renderEntry(e entry) string {
	if e.kind == entryPrompt {
		return m.renderPromptEntry(e)
	}

	var rendered string
	switch e.kind {
	case entryResult:
		rendered = m.styles.result.Render(e.text)
	case entryError:
		rendered = m.styles.error.Render(e.text)
	case entryNote:
		rendered = m.styles.note.Render(e.text)
	case entryBanner:
		rendered = m.styles.banner.Render(e.text)
	default:
		rendered = e.text
	}
	return ansi.Wrap(rendered, m.contentWidth, "")
}

// renderPromptEntry wraps prompt lines with a hanging indent so continuation
// lines line up with the text after the prompt.
func (m *model) renderPromptEntry(e entry) string {
	prefix := m.styles.prompt.Render(e.prompt)
	indent := lipgloss.Width(e.prompt)
	width := m.contentWidth - indent
	if width < 1 {
		width = 1
	}

	lines := strings.Split(ansi.Wrap(e.text, width, ""), "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = strings.Repeat(" ", indent) + lines[i]
	}
	return prefix + strings.Join(lines, "\n")
}

func (m *model) helpView() string {
	text := "enter eval · ↑/↓ history · pgup/pgdn scroll · esc cancel · ctrl+l clear · ctrl+c quit"
	switch {
	case m.width < 42:
		text = "enter eval · ↑↓ history · ctrl+c quit"
	case m.width < 78:
		text = "enter eval · ↑↓ history · esc cancel · ctrl+c quit"
	}

	h := ansi.Truncate(m.styles.help.Render(text), m.width, "…")
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, h)
}

const helpText = `commands
  .help   show this help
  .clear  clear the output panel
  .quit   exit jimbob

keys
  enter       evaluate, or continue if the expression is incomplete
  ↑ / ↓       browse input history
  pgup/pgdn   scroll the output panel
  esc         cancel a pending multi-line expression
  ctrl+l      clear the output panel
  ctrl+c      quit

examples
  (+ 1 2)
  x := 10
  (defn square [x] (* x x))
  for i := 0; i < 3; i++ { (println i) }`
