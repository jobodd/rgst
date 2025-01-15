package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jobodd/rgst/internal/colours"
)

type GitOptions struct {
	ShouldFetch    bool
	ShouldFetchAll bool
	ShouldPull     bool
	ShowFiles      bool
	Command        string
}

type GitStats struct {
	CurrentBranch        string
	HasRemote            bool
	NCommitsAheadRemote  int
	NCommitsBehindRemote int
	NCommitsAheadBranch  int
	NCommitsBehindBranch int
	NFilesAdded          int
	NFilesRemoved        int
	NFilesModified       int
	NFilesUnstaged       int
	ChangedFiles         []string
}

func UpdateDirectory(absPath string, opts GitOptions) {
	var cmd *exec.Cmd
	if opts.ShouldPull {
		cmd = exec.Command("git", "pull")
	} else if opts.ShouldFetchAll {
		cmd = exec.Command("git", "fetch", "--all", "--no-recurse-submodules")
	} else {
		cmd = exec.Command("git", "fetch", "--no-recurse-submodules")
	}

	cmd.Dir = absPath
	out, err := cmd.Output()

	if err != nil {
		// fmt.Println("failed to get current branch: %w")
		//TODO: add to msg
	} else {
		if len(out) == 0 {
			//TODO: add to msg
		}
	}
}

func GitFileStatus(porcelainLines []string) (int, int, int, int) {
	added, removed, modified, unstaged := 0, 0, 0, 0

	//TODO: this was rushed; sanity check these
	for _, line := range porcelainLines {
		switch line[:4] {
		case "[A ]": // staged
			added++
		case "[D ", " D]": // deleted
			removed++
		case "[ M]":
			unstaged++
		case "[M ]":
			modified++
		case "[MM]":
			modified++
			unstaged++
		case "[??]": // untracked?
			unstaged++
		case "[T ]": // type changed
			modified++
		case "[ T]": // type changed
			unstaged++
		case "[TT]": // type changed
			modified++
			unstaged++
		case "[R ]":
			modified++
		case "[ R]":
			unstaged++
		case "[RR]":
			modified++
			unstaged++
		case "[!!]": //ignored
			fmt.Printf("File ignored: %s\n", line)
		case "[UU]": // conflicted
			fmt.Printf("File conflict: %s\n", line)
		default:
			fmt.Printf("Unhandled file status: `%s`\n", line)
		}
	}

	return added, removed, modified, unstaged
}

func GetGitStats(absDir string) (GitStats, error) {
	var gitStats GitStats

	gitStats.CurrentBranch = strings.ReplaceAll(getGitBranch(absDir), "\n", "")
	if hasSingle, err := GitDirHasSingleRemote(absDir); err != nil {
		return gitStats, err
	} else {
		//TODO: handle multiple remotes
		gitStats.HasRemote = hasSingle
	}

	// Get the current branch
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = absDir
	branchOutput, err := cmd.Output()
	if err != nil {
		return gitStats, fmt.Errorf("failed to get current branch: %w", err)
	}
	gitStats.CurrentBranch = strings.TrimSpace(string(branchOutput))

	// Get ahead/behind count
	cmd = exec.Command("git",
		"rev-list",
		"--count",
		"--left-right",
		fmt.Sprintf("origin/%s...%s",
			gitStats.CurrentBranch,
			gitStats.CurrentBranch))
	cmd.Dir = absDir

	aheadBehindOutput, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(aheadBehindOutput), "unknown revision or path not in the working tree") {
			return gitStats, nil
		}
		return gitStats, fmt.Errorf("failed to get ahead/behind count.\nBranch was: %s\nError was: %w", gitStats.CurrentBranch, err)
	}
	parts := strings.Fields(string(aheadBehindOutput))
	if len(parts) == 2 {
		gitStats.NCommitsBehind, _ = strconv.Atoi(parts[0])
		gitStats.NCommitsAhead, _ = strconv.Atoi(parts[1])
	}

	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = absDir
	statusPorcelainOut, err := cmd.Output()
	if err != nil {
		return gitStats, fmt.Errorf("Failed to get git status --porcelain. %s", err)
	}
	porcelainStatus := strings.TrimRight(string(statusPorcelainOut), " \n")
	gitStats.ChangedFiles = strings.Split(porcelainStatus, "\n")
	if len(gitStats.ChangedFiles) == 1 && gitStats.ChangedFiles[0] == "" {
		gitStats.ChangedFiles = []string{}
	} else {
		for i := 0; i < len(gitStats.ChangedFiles); i++ {
			gitStats.ChangedFiles[i] = fmt.Sprintf(
				"[%s]%s",
				gitStats.ChangedFiles[i][0:2],
				gitStats.ChangedFiles[i][2:],
			)
		}
	}

	gitStats.NFilesAdded,
		gitStats.NFilesRemoved,
		gitStats.NFilesModified,
		gitStats.NFilesUnstaged = GitFileStatus(gitStats.ChangedFiles)

	return gitStats, nil
}

func GitDirHasSingleRemote(absDir string) (bool, error) {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = absDir
	remoteOutput, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to get current branch: %w", err)
	}
	wc := len(strings.Split(string(remoteOutput), "\n"))

	return wc == 3, nil
}

func getGitBranch(gitDirectory string) string {

	err := os.Chdir(gitDirectory)
	if err != nil {
		panic(err)
	}

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = gitDirectory

	branchOutput, err := cmd.Output()
	if err != nil {
		fmt.Println(string(branchOutput))
		panic(err)
	}

	return string(branchOutput)
}

func PrettyGitStats(g GitStats) string {
	ahead := fmt.Sprintf("\u2191%d", g.NCommitsAhead)
	if g.NCommitsAhead > 0 {
		ahead = colours.ColouredString(ahead, colours.Green)
	} else {
		ahead = colours.ColouredString(ahead, colours.White)
	}

	behind := fmt.Sprintf("\u2193%d", g.NCommitsBehind)
	if g.NCommitsBehind > 0 {
		behind = colours.ColouredString(behind, colours.Red)
	} else {
		behind = colours.ColouredString(behind, colours.White)
	}

	added := fmt.Sprintf("+%d", g.NFilesAdded)
	if g.NFilesAdded > 0 {
		added = colours.ColouredString(added, colours.Green)
	} else {
		added = colours.ColouredString(added, colours.White)
	}

	removed := fmt.Sprintf("-%d", g.NFilesRemoved)
	if g.NFilesRemoved > 0 {
		removed = colours.ColouredString(removed, colours.Red)
	} else {
		removed = colours.ColouredString(removed, colours.White)
	}

	modified := fmt.Sprintf("~%d", g.NFilesModified)
	if g.NFilesModified > 0 {
		modified = colours.ColouredString(modified, colours.Yellow)
	} else {
		modified = colours.ColouredString(modified, colours.White)
	}

	unstaged := fmt.Sprintf("U%d", g.NFilesUnstaged)
	if g.NFilesUnstaged > 0 {
		unstaged = colours.ColouredString(unstaged, colours.Red)
	} else {
		unstaged = colours.ColouredString(unstaged, colours.White)
	}

	return fmt.Sprintf(
		"%s\t%s\t%s\t%s\t%s\t%s\t",
		ahead,
		behind,
		added,
		removed,
		modified,
		unstaged,
	)

}
