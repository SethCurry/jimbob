# jimbob

A small terminal UI for the [Zygomys](https://github.com/glycerine/zygomys) Lisp
interpreter, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

The top of the screen is a scrollable transcript and output panel. The bottom is
a zygo REPL input line. Both Lisp forms and zygo's infix syntax work.

```
╭──────────────────────────────────────────────────────────╮
│ jimbob · zygo REPL                                       │
│ evaluate Lisp or infix expressions below · .help          │
│ zygo> x := 10                                            │
│ 10                                                       │
│ zygo> (defn square [x] (* x x))                          │
│ zygo> (square (* x 2))                                  │
│ 400                                                      │
╰──────────────────────────────────────────────────────────╯
╭──────────────────────────────────────────────────────────╮
│ zygo>                                                    │
╰──────────────────────────────────────────────────────────╯
       enter eval · ↑/↓ history · esc cancel · ctrl+c quit
```

## Run

Requires Go 1.27.2 or newer.

```sh
go run .
```

Or build a binary:

```sh
go build -o jimbob .
./jimbob
```

## Keys

| Key | Action |
| --- | --- |
| `enter` | Evaluate, or continue if the expression is incomplete |
| `↑` / `↓` | Browse input history |
| `pgup` / `pgdn` | Scroll the output panel |
| `esc` | Cancel a pending multi-line expression, or clear the input |
| `ctrl+l` | Clear the output panel |
| `ctrl+d` | Quit when the input is empty |
| `ctrl+c` | Quit |

## Commands

| Command | Action |
| --- | --- |
| `.help` | Show help in the output panel |
| `.clear` | Clear the output panel |
| `.quit` | Exit jimbob |

## Examples

Lisp forms:

```lisp
(+ 1 2)
(defn square [x] (* x x))
(square 12)
```

Infix syntax:

```lisp
x := 10
x * 3
if x > 5 { (println "big") } else { (println "small") }
for i := 0; i < 3; i++ { (println i) }
```

Multi-line forms are collected automatically. For example, entering
`for i := 0; i < 3; i++ {` and pressing `enter` switches the prompt to
continuation mode until the closing `}` is entered.

## Notes

- `print`, `println`, and `printf` are reimplemented so their output is captured
  in the transcript instead of being written directly to the terminal.
- Evaluation errors are shown in red with the zygo stack trace. The interpreter
  remains usable after an error.
- The zygo environment is persistent for the lifetime of the program.

## Development

```sh
go test ./...
go vet ./...
```
