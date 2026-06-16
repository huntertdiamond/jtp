// Package git provides helpers for interacting with git repositories and worktrees.
package git

import (
	stdErrors "errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/satococoa/wtp/v2/internal/errors"
)

const (
	jjWorkspaceRequiredFields = 2
	jjWorkspaceBookmarkField  = 2
	workspaceParentDirPerm    = 0o750
	remoteBookmarkFields      = 2
	defaultWorkspaceName      = "default"
)

// Repository represents a git repository and offers helper methods for worktree operations.
type Repository struct {
	path string
}

// NewRepository constructs a Repository for the given path after validating it is a jj repository.
func NewRepository(path string) (*Repository, error) {
	if !isJjRepository(path) {
		return nil, errors.NotInGitRepository()
	}
	return &Repository{path: path}, nil
}

// Path returns the root path for the repository.
func (r *Repository) Path() string {
	return r.path
}

// GetRepositoryName returns the name of the repository
func (r *Repository) GetRepositoryName() string {
	return filepath.Base(r.path)
}

// GetMainWorktreePath returns the path to the main worktree (original repository)
// This is useful when running commands from within a worktree
func (r *Repository) GetMainWorktreePath() (string, error) {
	workspaces, err := r.listJjWorkspaces()
	if err != nil {
		return "", err
	}
	if len(workspaces) == 0 {
		return "", fmt.Errorf("no jj workspaces found")
	}

	return r.workspaceRoot(mainWorkspaceName(workspaces))
}

// GetWorktrees lists the worktrees associated with the repository.
func (r *Repository) GetWorktrees() ([]Worktree, error) {
	workspaces, err := r.listJjWorkspaces()
	if err != nil {
		return nil, err
	}

	worktrees := make([]Worktree, 0, len(workspaces))
	mainName := mainWorkspaceName(workspaces)
	for i := range workspaces {
		workspace := workspaces[i]
		root, err := r.workspaceRoot(workspace.Name)
		if err != nil {
			return nil, err
		}

		worktrees = append(worktrees, Worktree{
			Path:   root,
			Branch: workspace.DisplayName(),
			HEAD:   workspace.Head,
			IsMain: workspace.Name == mainName,
		})
	}

	return worktrees, nil
}

func mainWorkspaceName(workspaces []jjWorkspace) string {
	for _, workspace := range workspaces {
		if workspace.Name == defaultWorkspaceName {
			return workspace.Name
		}
	}
	if len(workspaces) == 0 {
		return ""
	}
	return workspaces[0].Name
}

type jjWorkspace struct {
	Name     string
	Head     string
	Bookmark string
}

func (w jjWorkspace) DisplayName() string {
	if w.Bookmark != "" {
		return w.Bookmark
	}
	return w.Name
}

func (r *Repository) listJjWorkspaces() ([]jjWorkspace, error) {
	template := `name ++ "\t" ++ target.commit_id().short() ++ "\t" ++ target.bookmarks() ++ "\n"`
	cmd := exec.Command("jj", "workspace", "list", "--template", template)
	cmd.Dir = r.path
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list jj workspaces: %w", err)
	}

	return parseJjWorkspaceList(string(output)), nil
}

func parseJjWorkspaceList(output string) []jjWorkspace {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	workspaces := make([]jjWorkspace, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < jjWorkspaceRequiredFields {
			continue
		}

		workspace := jjWorkspace{
			Name: strings.TrimSpace(parts[0]),
			Head: strings.TrimSpace(parts[1]),
		}
		if len(parts) > jjWorkspaceBookmarkField {
			workspace.Bookmark = firstBookmark(parts[jjWorkspaceBookmarkField])
		}
		workspaces = append(workspaces, workspace)
	}
	return workspaces
}

func firstBookmark(bookmarks string) string {
	for _, field := range strings.Fields(bookmarks) {
		return strings.TrimSuffix(field, "*")
	}
	return ""
}

func (r *Repository) workspaceRoot(name string) (string, error) {
	cmd := exec.Command("jj", "workspace", "root", "--name", name)
	cmd.Dir = r.path
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get jj workspace root for %q: %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// CreateWorktree creates a new worktree at the given path and optionally checks out the branch.
func (r *Repository) CreateWorktree(path, branch string) error {
	if err := os.MkdirAll(filepath.Dir(path), workspaceParentDirPerm); err != nil {
		return fmt.Errorf("failed to create workspace parent directory: %w", err)
	}

	workspaceName := filepath.Base(path)
	if branch != "" {
		workspaceName = branch
	}

	args := []string{"workspace", "add", "--name", workspaceName}
	if branch != "" {
		args = append(args, "--revision", branch)
	}
	args = append(args, path)

	// #nosec G204 - args are passed as argv elements; callers validate user-facing refs.
	cmd := exec.Command("jj", args...)
	cmd.Dir = r.path
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create jj workspace: %w", err)
	}
	return nil
}

// RemoveWorktree removes the worktree at the provided path, optionally forcing removal.
func (r *Repository) RemoveWorktree(path string, _ bool) error {
	workspaces, err := r.listJjWorkspaces()
	if err != nil {
		return err
	}

	workspaceName := ""
	targetPath := normalizePathForCompare(path)
	for _, workspace := range workspaces {
		root, err := r.workspaceRoot(workspace.Name)
		if err != nil {
			return err
		}
		if normalizePathForCompare(root) == targetPath {
			workspaceName = workspace.Name
			break
		}
	}
	if workspaceName == "" {
		return fmt.Errorf("workspace for path %q not found", path)
	}

	cmd := exec.Command("jj", "workspace", "forget", workspaceName)
	cmd.Dir = r.path
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to forget jj workspace: %w", err)
	}
	return os.RemoveAll(path)
}

func normalizePathForCompare(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	evaluatedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return filepath.Clean(absPath)
	}
	return filepath.Clean(evaluatedPath)
}

// ExecuteGitCommand executes a jj command in the repository directory
func (r *Repository) ExecuteGitCommand(args ...string) error {
	cmd := exec.Command("jj", args...)
	cmd.Dir = r.path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.GitCommandFailed(fmt.Sprintf("jj %s", strings.Join(args, " ")), string(output))
	}
	return nil
}

// BranchExists checks if a bookmark exists locally.
func (r *Repository) BranchExists(branch string) (bool, error) {
	if strings.Contains(branch, "..") || strings.ContainsAny(branch, "\n\r") {
		return false, errors.InvalidBranchName(branch)
	}

	// #nosec G204 - branch is validated above and passed as an argv element.
	cmd := exec.Command("jj", "bookmark", "list", branch, "--template", `name ++ "\n"`)
	cmd.Dir = r.path
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if stdErrors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check branch existence: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == branch {
			return true, nil
		}
	}
	return false, nil
}

// GetRemoteBranches returns a map of tracked remote bookmarks by remote name.
func (r *Repository) GetRemoteBranches(branch string) (map[string]string, error) {
	if strings.Contains(branch, "..") || strings.ContainsAny(branch, "\n\r") {
		return nil, errors.InvalidBranchName(branch)
	}

	remotes := make(map[string]string)

	// #nosec G204 - branch is validated above and passed as an argv element.
	cmd := exec.Command("jj", "bookmark", "list", "--all-remotes", branch, "--template", `name ++ "\t" ++ remote ++ "\n"`)
	cmd.Dir = r.path
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get remote bookmarks: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < remoteBookmarkFields {
			continue
		}
		name := strings.TrimSpace(parts[0])
		remote := strings.TrimSpace(parts[1])
		if name == branch && remote != "" && remote != "git" {
			remotes[remote] = fmt.Sprintf("%s@%s", name, remote)
		}
	}

	return remotes, nil
}

// NormalizeRevision converts Git-style remote revisions such as origin/feature/x
// to jj remote bookmark revisions such as feature/x@origin when the remote exists.
func (r *Repository) NormalizeRevision(revision string) (string, error) {
	remote, branch, ok := splitGitRemoteRevision(revision)
	if !ok {
		return revision, nil
	}

	exists, err := r.remoteExists(remote)
	if err != nil {
		return "", err
	}
	if !exists {
		return revision, nil
	}

	return fmt.Sprintf("%s@%s", branch, remote), nil
}

func splitGitRemoteRevision(revision string) (remote, branch string, ok bool) {
	remote, branch, ok = strings.Cut(revision, "/")
	if !ok || remote == "" || branch == "" {
		return "", "", false
	}
	return remote, branch, true
}

func (r *Repository) remoteExists(remote string) (bool, error) {
	cmd := exec.Command("jj", "git", "remote", "list")
	cmd.Dir = r.path
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to list remotes: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == remote {
			return true, nil
		}
	}
	return false, nil
}

// ResolveBranch resolves a branch name following git's behavior:
// 1. Check if branch exists locally
// 2. If not, check remote branches
// 3. If multiple remotes have the branch, return an error
func (r *Repository) ResolveBranch(branch string) (resolvedBranch string, isRemote bool, err error) {
	// First check if branch exists locally
	exists, err := r.BranchExists(branch)
	if err != nil {
		return "", false, err
	}
	if exists {
		return branch, false, nil
	}

	// Check remote branches
	remoteBranches, err := r.GetRemoteBranches(branch)
	if err != nil {
		return "", false, err
	}

	if len(remoteBranches) == 0 {
		return "", false, errors.BranchNotFound(branch)
	}

	if len(remoteBranches) > 1 {
		// Multiple remotes have this branch
		remoteNames := make([]string, 0, len(remoteBranches))
		for remote := range remoteBranches {
			remoteNames = append(remoteNames, remote)
		}
		return "", false, errors.MultipleBranchesFound(branch, remoteNames)
	}

	// Single remote has this branch
	for _, remoteBranch := range remoteBranches {
		return remoteBranch, true, nil
	}

	return "", false, nil
}

func isJjRepository(path string) bool {
	cmd := exec.Command("jj", "root")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func isGitRepository(path string) bool {
	return isJjRepository(path)
}

func parseWorktreeList(output string) []Worktree {
	var worktrees []Worktree
	lines := strings.Split(output, "\n")

	var current *Worktree
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}

		if after, found := strings.CutPrefix(line, "worktree "); found {
			current = &Worktree{
				Path: after,
			}
		} else if current != nil {
			if after, found := strings.CutPrefix(line, "HEAD "); found {
				current.HEAD = after
			} else if after, found := strings.CutPrefix(line, "branch refs/heads/"); found {
				current.Branch = after
			}
		}
	}

	if current != nil {
		worktrees = append(worktrees, *current)
	}

	return worktrees
}
