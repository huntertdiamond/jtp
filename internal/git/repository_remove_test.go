package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runJjCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("jj", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run jj %v: %v", args, err)
	}
}

func initializeTestRepo(t *testing.T, repoDir string) {
	t.Helper()
	runJjCommand(t, repoDir, "git", "init")

	readmeFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Test Repo"), 0644); err != nil {
		t.Fatalf("Failed to write README: %v", err)
	}
	runJjCommand(t, repoDir, "describe", "-m", "Initial commit")
	runJjCommand(t, repoDir, "bookmark", "create", "main", "-r", "@")
}

func createBookmark(t *testing.T, repoDir, bookmarkName string) {
	t.Helper()
	runJjCommand(t, repoDir, "bookmark", "create", bookmarkName, "-r", "@")
}

func setupTestRepoWithBranches(t *testing.T) (repoDir, mergedBranch, unmergedBranch string) {
	repoDir = t.TempDir()
	mergedBranch = "merged-branch"
	unmergedBranch = "unmerged-branch"

	initializeTestRepo(t, repoDir)
	createBookmark(t, repoDir, mergedBranch)
	createBookmark(t, repoDir, unmergedBranch)

	return repoDir, mergedBranch, unmergedBranch
}

func TestBranchDeletion(t *testing.T) {
	repoDir, mergedBranch, unmergedBranch := setupTestRepoWithBranches(t)

	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	err = repo.ExecuteGitCommand("bookmark", "delete", mergedBranch)
	if err != nil {
		t.Errorf("Failed to delete bookmark: %v", err)
	}

	err = repo.ExecuteGitCommand("bookmark", "delete", unmergedBranch)
	if err != nil {
		t.Errorf("Failed to delete second bookmark: %v", err)
	}

	cmd := exec.Command("jj", "bookmark", "list", "--template", `name ++ "\n"`)
	cmd.Dir = repoDir
	output, _ := cmd.Output()
	branches := string(output)

	if strings.Contains(branches, mergedBranch) {
		t.Error("First bookmark should have been deleted")
	}
	if strings.Contains(branches, unmergedBranch) {
		t.Error("Second bookmark should have been deleted")
	}
}

func TestWorktreeWithBranchRemoval(t *testing.T) {
	repoDir, _, unmergedBranch := setupTestRepoWithBranches(t)

	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Create worktree for unmerged branch
	worktreePath := filepath.Join(repoDir, "..", "worktrees", unmergedBranch)
	err = repo.CreateWorktree(worktreePath, unmergedBranch)
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	// Remove worktree
	err = repo.RemoveWorktree(worktreePath, false)
	if err != nil {
		t.Errorf("Failed to remove worktree: %v", err)
	}

	if _, statErr := os.Stat(worktreePath); !os.IsNotExist(statErr) {
		t.Errorf("Expected worktree directory to be removed, got stat error: %v", statErr)
	}

	err = repo.ExecuteGitCommand("bookmark", "delete", unmergedBranch)
	if err != nil {
		t.Errorf("Failed to delete bookmark: %v", err)
	}
}
