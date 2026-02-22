package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	gitignoreFileMode       = 0o600
	gitignoreAffirmativeY   = "y"
	gitignoreAffirmativeYes = "yes"
)

func offerAddKanbanToGitignore(kanbanDir string) error {
	gitignorePath, entry, err := gitignorePromptData(kanbanDir)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Add %q to .gitignore? [Y/n] ", entry)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("reading input: %w", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "" && answer != gitignoreAffirmativeY && answer != gitignoreAffirmativeYes {
		return nil
	}

	return ensureGitignoreEntry(gitignorePath, entry)
}

func gitignorePromptData(kanbanDir string) (string, string, error) {
	absDir, err := filepath.Abs(kanbanDir)
	if err != nil {
		return "", "", fmt.Errorf("resolving path: %w", err)
	}

	parentDir := filepath.Dir(absDir)
	entry := filepath.Base(absDir)
	if entry == "" || entry == "." {
		return "", "", fmt.Errorf("invalid kanban directory %q", kanbanDir)
	}

	return filepath.Join(parentDir, ".gitignore"), entry + "/", nil
}

func ensureGitignoreEntry(gitignorePath, entry string) error {
	entry = sanitizeGitignoreEntry(entry)
	contents, err := os.ReadFile(gitignorePath) //nolint:gosec // path is derived from validated kanban directory provided by the caller
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	if err == nil && hasGitignoreEntry(contents, entry) {
		return nil
	}

	if os.IsNotExist(err) {
		return os.WriteFile(gitignorePath, []byte(entry+"\n"), gitignoreFileMode)
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, gitignoreFileMode) //nolint:gosec // path is derived from validated kanban directory provided by the caller
	if err != nil {
		return fmt.Errorf("opening .gitignore: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	if len(contents) > 0 && contents[len(contents)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("updating .gitignore: %w", err)
		}
	}

	if _, err := f.WriteString(entry + "\n"); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	}
	return nil
}

func hasGitignoreEntry(contents []byte, entry string) bool {
	for _, line := range strings.Split(string(contents), "\n") {
		if strings.TrimSpace(line) == entry {
			return true
		}
	}
	return false
}

func sanitizeGitignoreEntry(entry string) string {
	clean := strings.TrimSpace(filepath.ToSlash(entry))
	clean = strings.TrimSuffix(clean, "/")
	return clean + "/"
}
