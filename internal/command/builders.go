// Package command provides helpers to build and execute jj commands.
package command

// GitWorktreeAddOptions represents options for jj workspace add command.
type GitWorktreeAddOptions struct {
	Force  bool
	Detach bool
	Branch string
	Track  string
}

// GitWorktreeAdd builds a jj workspace add command.
func GitWorktreeAdd(path, commitish string, opts GitWorktreeAddOptions) Command {
	workspaceName := opts.Branch
	if workspaceName == "" {
		workspaceName = extractBranchName(commitish)
	}
	if workspaceName == "" {
		workspaceName = pathBase(path)
	}

	args := []string{"workspace", "add", "--name", workspaceName}
	if commitish != "" {
		args = append(args, "--revision", commitish)
	}
	args = append(args, path)

	return Command{
		Name: "jj",
		Args: args,
	}
}

// GitBranchDelete builds a jj bookmark delete command.
func GitBranchDelete(branchName string, _ bool) Command {
	args := []string{"bookmark", "delete", branchName}
	return Command{
		Name: "jj",
		Args: args,
	}
}

// GitWorktreeRemove builds a jj workspace forget command.
func GitWorktreeRemove(path string, _ bool) Command {
	return Command{
		Name: "jj",
		Args: []string{"workspace", "forget", path},
	}
}

// GitWorktreeList builds a jj workspace list command.
func GitWorktreeList() Command {
	return Command{
		Name: "jj",
		Args: []string{
			"workspace",
			"list",
			"--template",
			`name ++ "\t" ++ target.commit_id().short() ++ "\t" ++ target.bookmarks() ++ "\n"`,
		},
	}
}

// extractBranchName extracts branch name from a remote reference
// e.g., "origin/feature" -> "feature"
func extractBranchName(ref string) string {
	// Simple implementation - in real code this might be more sophisticated
	if ref == "" {
		return ref
	}

	// If it contains a slash, take the last part
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '/' {
			return ref[i+1:]
		}
	}

	return ref
}

func pathBase(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
