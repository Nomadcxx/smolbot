package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unicode"

	term "github.com/charmbracelet/x/term"
)

type readlineSession interface {
	ReadLine() (string, error)
	AddToHistory(line string)
	Close() error
	GetTerminalSize() (int, int, error)
}

type bubbleteaReadline struct {
	in       *chatIO
	out      io.Writer
	width    int
	height   int
	closed   bool
	mu       sync.Mutex
	history  []string
	histIdx  int
	pasteBuf strings.Builder
}

func newBubbleteaReadline(in *chatIO, out io.Writer) (readlineSession, error) {
	width, height := 80, 24
	if fd := os.Stdin.Fd(); term.IsTerminal(fd) {
		if w, h, err := term.GetSize(fd); err == nil {
			width, height = w, h
		}
	}

	return &bubbleteaReadline{
		in:      in,
		out:     out,
		width:   width,
		height:  height,
		histIdx: -1,
	}, nil
}

func (r *bubbleteaReadline) ReadLine() (string, error) {
	if r.closed {
		return "", io.EOF
	}

	fmt.Fprint(r.out, "\n> ")

	var currentLine strings.Builder
	var lines []string
	pasteMode := false

	reader := bufio.NewReader(r.in.In)
	for {
		ch, err := reader.ReadByte()
		if err != nil {
			if currentLine.Len() > 0 || len(lines) > 0 {
				break
			}
			return "", err
		}

		if ch == '\x1b' {
			seq, err := reader.Peek(6)
			if err == nil && len(seq) >= 6 {
				if seq[0] == '[' {
					switch seq[1] {
					case 'A':
						reader.ReadByte()
						reader.ReadByte()
						entry := r.navigateHistory(-1)
						currentLine.Reset()
						currentLine.WriteString(entry)
						r.redrawPrompt(&currentLine)
						continue
					case 'B':
						reader.ReadByte()
						reader.ReadByte()
						entry := r.navigateHistory(1)
						currentLine.Reset()
						currentLine.WriteString(entry)
						r.redrawPrompt(&currentLine)
						continue
					}
				}
				if seq[0] == '[' && seq[1] == '2' && seq[2] == '0' && seq[3] == '0' && seq[4] == '~' {
					for i := 0; i < 5; i++ {
						reader.ReadByte()
					}
					pasteMode = true
					r.mu.Lock()
					r.pasteBuf.Reset()
					r.mu.Unlock()
					continue
				}
				if seq[0] == '[' && seq[1] == '2' && seq[2] == '0' && seq[3] == '1' && seq[4] == '~' {
					for i := 0; i < 5; i++ {
						reader.ReadByte()
					}
					if pasteMode {
						pasteMode = false
						currentLine.WriteString(r.pasteBuf.String())
						r.mu.Lock()
						r.pasteBuf.Reset()
						r.mu.Unlock()
					}
					reader.ReadByte() // skip the character after [201~
					continue
				}
			}
			continue
		}

		if pasteMode {
			if ch == '\r' || ch == '\n' {
				continue
			}
			if unicode.IsPrint(rune(ch)) {
				r.mu.Lock()
				r.pasteBuf.WriteByte(ch)
				r.mu.Unlock()
				r.redrawPrompt(&r.pasteBuf)
			}
			continue
		}

		switch ch {
		case '\n', '\r':
			line := currentLine.String()

			if line == "" && len(lines) == 0 {
				continue
			}

			if strings.HasSuffix(line, "\\") {
				lines = append(lines, strings.TrimSuffix(line, "\\"))
				currentLine.Reset()
				r.redrawPrompt(&currentLine)
				continue
			}

			if line == "" && len(lines) > 0 {
				break
			}

			lines = append(lines, line)
			break
		case '\b':
			if currentLine.Len() > 0 {
				currentLine.WriteByte(ch)
			}
		default:
			if unicode.IsPrint(rune(ch)) {
				currentLine.WriteByte(ch)
				r.redrawPrompt(&currentLine)
			}
		}
	}

	var result string
	if len(lines) > 0 {
		result = strings.Join(lines, "\n")
	} else {
		result = currentLine.String()
	}

	return result, nil
}

func (r *bubbleteaReadline) redrawPrompt(currentLine *strings.Builder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprint(r.out, "\r"+strings.Repeat(" ", r.width)+"\r> "+currentLine.String())
}

func (r *bubbleteaReadline) navigateHistory(delta int) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.history) == 0 {
		return ""
	}

	if r.histIdx == -1 {
		r.histIdx = len(r.history)
	}

	newIdx := r.histIdx + delta
	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= len(r.history) {
		r.histIdx = len(r.history)
		return ""
	}

	r.histIdx = newIdx
	return r.history[newIdx]
}

func (r *bubbleteaReadline) AddToHistory(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.history = append(r.history, line)
	r.histIdx = len(r.history)
}

func (r *bubbleteaReadline) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func (r *bubbleteaReadline) GetTerminalSize() (int, int, error) {
	return r.width, r.height, nil
}

var _ readlineSession = (*bubbleteaReadline)(nil)
