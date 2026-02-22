//go:build !windows

package e2e_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"
)

const (
	tuiRows           = 40
	tuiCols           = 120
	tuiStartupTimeout = 3 * time.Second
	tuiExitTimeout    = 3 * time.Second
	tuiKeyDelay       = 12 * time.Millisecond
	tuiTaskTimeout    = 2 * time.Second
)

var (
	ansiCSIRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	ansiOSCRe = regexp.MustCompile(`\x1b\][^\x07]*\x07`)
)

type tuiOutputBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *tuiOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *tuiOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type tuiSession struct {
	t       *testing.T
	cmd     *exec.Cmd
	ptmx    *os.File
	out     tuiOutputBuffer
	done    chan struct{}
	doneErr chan error
	cleanup sync.Once
}

func startTUIProcess(t *testing.T, dir string) *tuiSession {
	t.Helper()

	cmd := exec.Command(binPath, "--dir", dir, "tui") //nolint:gosec,noctx // command uses test-built binary path
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: tuiCols, Rows: tuiRows})
	if err != nil {
		t.Fatalf("starting TUI process: %v", err)
	}

	session := &tuiSession{
		t:       t,
		cmd:     cmd,
		ptmx:    ptmx,
		done:    make(chan struct{}),
		doneErr: make(chan error, 1),
	}

	go func() {
		_, _ = io.Copy(&session.out, ptmx)
		session.doneErr <- cmd.Wait()
		close(session.done)
	}()

	t.Cleanup(session.close)
	return session
}

func (s *tuiSession) close() {
	s.cleanup.Do(func() {
		_ = s.pressKey("q")
		timer := time.NewTimer(tuiExitTimeout)
		select {
		case <-s.done:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			_ = s.pressKey("ctrl+c")
			select {
			case <-s.done:
			case <-time.After(150 * time.Millisecond):
				if s.cmd.Process != nil {
					_ = s.cmd.Process.Kill()
				}
			}
		}

		select {
		case err := <-s.doneErr:
			_ = err
		case <-time.After(time.Second):
			if s.cmd.Process != nil {
				_ = s.cmd.Process.Kill()
			}
			select {
			case err := <-s.doneErr:
				_ = err
			default:
			}
		}
		_ = s.ptmx.Close()
	})
}

func (s *tuiSession) output() string {
	return sanitizeTTYOutput(s.out.String())
}

func (s *tuiSession) pressKey(name string) error {
	s.t.Helper()
	_, err := s.ptmx.Write([]byte(encodeKey(name)))
	if err != nil {
		return err
	}
	if s.t != nil {
		time.Sleep(tuiKeyDelay)
	}
	return nil
}

func (s *tuiSession) pressKeys(names ...string) {
	s.t.Helper()
	for _, name := range names {
		if err := s.pressKey(name); err != nil {
			s.t.Fatalf("pressing key %q: %v", name, err)
		}
	}
}

func (s *tuiSession) typeText(text string) {
	s.t.Helper()
	for _, r := range text {
		if err := s.pressKey(string(r)); err != nil {
			s.t.Fatalf("typing text %q: %v", text, err)
		}
	}
}

func (s *tuiSession) pressBackspace(count int) {
	s.t.Helper()
	for range count {
		if err := s.pressKey("backspace"); err != nil {
			s.t.Fatalf("backspacing %d times: %v", count, err)
		}
	}
}

func (s *tuiSession) pressBackspaceRunes(value string) {
	s.pressBackspace(utf8.RuneCountInString(value))
}

func (s *tuiSession) waitForOutput(needle string) {
	s.t.Helper()
	deadline := time.Now().Add(tuiStartupTimeout)
	for time.Now().Before(deadline) {
		if strings.Contains(s.output(), needle) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("timed out waiting for output containing %q; got %q", needle, s.output())
}

func (s *tuiSession) waitForExit() {
	s.t.Helper()
	select {
	case <-s.done:
		_ = s.waitErr()
	case <-time.After(tuiExitTimeout):
		s.t.Fatalf("timed out waiting for TUI process to exit")
	}
}

func (s *tuiSession) waitErr() error {
	select {
	case err := <-s.doneErr:
		return err
	default:
		return nil
	}
}

func encodeKey(name string) string {
	switch name {
	case "enter", "return":
		return "\r"
	case "tab":
		return "\t"
	case "esc":
		return "\x1b"
	case "shift+tab":
		return "\x1b[Z"
	case "up":
		return "\x1b[A"
	case "down":
		return "\x1b[B"
	case "left":
		return "\x1b[D"
	case "right":
		return "\x1b[C"
	case "backspace", "delete":
		return "\x7f"
	case "ctrl+c":
		return "\x03"
	default:
		return name
	}
}

func sanitizeTTYOutput(raw string) string {
	raw = strings.ReplaceAll(raw, "\r", "")
	raw = ansiCSIRe.ReplaceAllString(raw, "")
	raw = ansiOSCRe.ReplaceAllString(raw, "")
	return raw
}

func initBoardWithSeededTasks(t *testing.T) string {
	t.Helper()

	dir := initBoard(t)
	mustCreateTask(t, dir, "Task A", "--priority", "high")
	mustCreateTask(t, dir, "Task B", "--priority", "medium")
	mustCreateTask(t, dir, "Task C", "--status", "in-progress", "--priority", "high")
	mustCreateTask(t, dir, "Task D", "--status", "done", "--priority", "low")

	return dir
}

func waitForTask(t *testing.T, kanbanDir string, id int, check func(taskJSON) bool) {
	t.Helper()

	deadline := time.Now().Add(tuiTaskTimeout)
	for {
		var tk taskJSON
		r := runKanbanJSON(t, kanbanDir, &tk, "show", strconv.Itoa(id))
		if r.exitCode == 0 && check(tk) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for task update (id=%d), last seen: %#v", id, tk)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func containsTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}
