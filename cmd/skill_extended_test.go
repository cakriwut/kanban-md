package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/antopolskiy/kanban-md/internal/skill"
)

const testVersion = "v1.0.0"

// --- resolveAgentList tests ---

func TestResolveAgentList_AllAgents(t *testing.T) {
	// Empty filter returns all agents.
	agents := resolveAgentList(nil)
	if len(agents) == 0 {
		t.Fatal("expected non-empty agent list")
	}
	if len(agents) != len(skill.Agents()) {
		t.Errorf("len = %d, want %d", len(agents), len(skill.Agents()))
	}
}

func TestResolveAgentList_SpecificAgent(t *testing.T) {
	agents := resolveAgentList([]string{"claude"})
	if len(agents) != 1 {
		t.Fatalf("len = %d, want 1", len(agents))
	}
	if agents[0].Name != "claude" {
		t.Errorf("agent name = %q, want %q", agents[0].Name, "claude")
	}
}

func TestResolveAgentList_MultipleAgents(t *testing.T) {
	agents := resolveAgentList([]string{"claude", "codex"})
	if len(agents) != 2 {
		t.Fatalf("len = %d, want 2", len(agents))
	}
}

func TestResolveAgentList_UnknownAgent(t *testing.T) {
	agents := resolveAgentList([]string{"unknown-agent"})
	if len(agents) != 0 {
		t.Errorf("expected empty list for unknown agent, got %d", len(agents))
	}
}

func TestResolveAgentList_MixedKnownUnknown(t *testing.T) {
	agents := resolveAgentList([]string{"claude", "unknown-agent"})
	if len(agents) != 1 {
		t.Fatalf("len = %d, want 1 (only known agent)", len(agents))
	}
	if agents[0].Name != "claude" {
		t.Errorf("agent name = %q, want %q", agents[0].Name, "claude")
	}
}

// --- resolveSkills tests (with explicit filter) ---

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringSlice("skill", nil, "")
	return cmd
}

func TestResolveSkills_AllSkills(t *testing.T) {
	// Empty filter in non-interactive mode returns all skills.
	skills, err := resolveSkills(newSkillCmd(), nil)
	if err != nil {
		t.Fatalf("resolveSkills error: %v", err)
	}
	if len(skills) != len(skill.AvailableSkills) {
		t.Errorf("len = %d, want %d", len(skills), len(skill.AvailableSkills))
	}
}

func TestResolveSkills_SpecificSkill(t *testing.T) {
	skills, err := resolveSkills(newSkillCmd(), []string{"kanban-md"})
	if err != nil {
		t.Fatalf("resolveSkills error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("len = %d, want 1", len(skills))
	}
	if skills[0].Name != "kanban-md" {
		t.Errorf("skill name = %q, want %q", skills[0].Name, "kanban-md")
	}
}

func TestResolveSkills_UnknownSkill(t *testing.T) {
	_, err := resolveSkills(newSkillCmd(), []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
}

// --- relativePath tests ---

func TestRelativePath_Simple(t *testing.T) {
	got := relativePath("/home/user/project", "/home/user/project/skills/SKILL.md")
	want := "skills/SKILL.md"
	if got != want {
		t.Errorf("relativePath = %q, want %q", got, want)
	}
}

func TestRelativePath_SamePath(t *testing.T) {
	got := relativePath("/home/user", "/home/user")
	if got != "." {
		t.Errorf("relativePath = %q, want %q", got, ".")
	}
}

// --- exitCodeError tests ---

func TestExitCodeError_Error(t *testing.T) {
	e := &exitCodeError{code: 1}
	got := e.Error()
	if got != "exit code 1" {
		t.Errorf("Error() = %q, want %q", got, "exit code 1")
	}
}

func TestExitCodeError_ZeroCode(t *testing.T) {
	e := &exitCodeError{code: 0}
	got := e.Error()
	if got != "exit code 0" {
		t.Errorf("Error() = %q, want %q", got, "exit code 0")
	}
}

// --- findProjectRoot tests ---

func TestFindProjectRoot_GitDir(t *testing.T) {
	// Create a temp directory with a .git directory.
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}

	// Create a subdirectory to search from.
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}

	t.Chdir(sub)

	got, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot error: %v", err)
	}
	if got != root {
		t.Errorf("findProjectRoot = %q, want %q", got, root)
	}
}

func TestFindProjectRoot_NoGitDir(t *testing.T) {
	// In a temp directory with no .git, should return cwd.
	dir := t.TempDir()
	t.Chdir(dir)

	got, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot error: %v", err)
	}
	// Should return cwd since there's no .git anywhere up the temp tree.
	// It walks to root and falls back to cwd.
	if got != dir {
		t.Errorf("findProjectRoot = %q, want %q (cwd fallback)", got, dir)
	}
}

// --- CheckSkillStaleness tests ---

func TestCheckSkillStaleness_DevVersion(t *testing.T) {
	// When version is "dev", should not print anything.
	savedVersion := version
	version = "dev"
	t.Cleanup(func() { version = savedVersion })

	r, w := captureStderr(t)

	CheckSkillStaleness(t.TempDir())

	got := drainPipe(t, r, w)
	if got != "" {
		t.Errorf("expected no output for dev version, got: %s", got)
	}
}

func TestCheckSkillStaleness_NoSkillsInstalled(t *testing.T) {
	savedVersion := version
	version = testVersion
	t.Cleanup(func() { version = savedVersion })

	r, w := captureStderr(t)

	// Empty project root with no installed skills.
	CheckSkillStaleness(t.TempDir())

	got := drainPipe(t, r, w)
	if got != "" {
		t.Errorf("expected no output when no skills installed, got: %s", got)
	}
}

func TestCheckSkillStaleness_OutdatedSkill(t *testing.T) {
	savedVersion := version
	version = "v2.0.0"
	t.Cleanup(func() { version = savedVersion })

	// Install a skill with an old version.
	projectRoot := t.TempDir()
	claude := skill.AgentByName("claude")
	if claude == nil {
		t.Skip("claude agent not found")
	}
	baseDir := claude.ProjectPath(projectRoot)
	if err := skill.Install("kanban-md", baseDir, testVersion); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	r, w := captureStderr(t)

	CheckSkillStaleness(projectRoot)

	got := drainPipe(t, r, w)
	if !containsSubstring(got, "outdated") {
		t.Errorf("expected 'outdated' warning, got: %s", got)
	}
}

func TestCheckSkillStaleness_UpToDate(t *testing.T) {
	savedVersion := version
	version = testVersion
	t.Cleanup(func() { version = savedVersion })

	// Install a skill at the current version.
	projectRoot := t.TempDir()
	claude := skill.AgentByName("claude")
	if claude == nil {
		t.Skip("claude agent not found")
	}
	baseDir := claude.ProjectPath(projectRoot)
	if err := skill.Install("kanban-md", baseDir, testVersion); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	r, w := captureStderr(t)

	CheckSkillStaleness(projectRoot)

	got := drainPipe(t, r, w)
	if got != "" {
		t.Errorf("expected no output when up to date, got: %s", got)
	}
}

// --- installToPath tests ---

func TestInstallToPath_CreatesFiles(t *testing.T) {
	dir := t.TempDir()

	err := installToPath(dir, skill.AvailableSkills, true)
	if err != nil {
		t.Fatalf("installToPath error: %v", err)
	}

	// Verify skill files were created.
	for _, s := range skill.AvailableSkills {
		path := filepath.Join(dir, s.Name, "SKILL.md")
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("expected skill file at %s, got error: %v", path, statErr)
		}
	}
}

func TestInstallToPath_SkipsCurrent(t *testing.T) {
	savedVersion := version
	version = testVersion
	t.Cleanup(func() { version = savedVersion })

	dir := t.TempDir()

	// Install first time.
	if err := installToPath(dir, skill.AvailableSkills, true); err != nil {
		t.Fatalf("first installToPath error: %v", err)
	}

	// Install again without force — should skip (already at current version).
	r, w := captureStdout(t)
	err := installToPath(dir, skill.AvailableSkills, false)
	got := drainPipe(t, r, w)

	if err != nil {
		t.Fatalf("second installToPath error: %v", err)
	}
	if !containsSubstring(got, "skipped") {
		t.Errorf("expected 'skipped' in output, got: %s", got)
	}
}
