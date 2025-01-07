package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jobodd/rgst/internal/colours"
)

type GitStats struct {
	CurrentBranch  string
	HasRemote      bool
	NCommitsAhead  int
	NCommitsBehind int
	NFilesAdded    int
	NFilesRemoved  int
	NFilesModified int
	NFilesUnstaged int
}

type FormatStats struct {
	MaxFolderTreeWidth int
	MaxBranchWidth     int
	MaxGitStatsWidth   int
	MaxAheadWidth      int
	MaxBehindWidth     int
	MaxAddedWidth      int
	MaxRemovedWidth    int
	MaxModifiedWidth   int
	MaxUnstagedWidth   int
}

func (g *GitStats) StatsLen() int {
	return len(strconv.Itoa(g.NCommitsAhead)) +
		len(strconv.Itoa(g.NCommitsBehind)) +
		len(strconv.Itoa(g.NFilesAdded)) +
		len(strconv.Itoa(g.NFilesRemoved)) +
		len(strconv.Itoa(g.NFilesModified)) +
		len(strconv.Itoa(g.NFilesUnstaged))
}

func GitFileStatus(statusOutput string) (int, int, int, int) {
	added, removed, modified, unstaged := 0, 0, 0, 0

	for _, line := range strings.Split(statusOutput, "\n") {
		if len(line) < 2 {
			continue
		}

		switch line[:2] {
		case "A ": // staged
			added++
		case " D": // deleted
			removed++
		case " M":
			unstaged++
		case "M ":
			modified++
		case "MM":
			modified++
			unstaged++
		case "??": // untracked?
			unstaged++
		case "T ": // type changed
			modified++
		case " T": // type changed
			unstaged++
		case "TT": // type changed
			modified++
			unstaged++
		case "R ":
			modified++
		case " R":
			unstaged++
		case "RR":
			modified++
			unstaged++
		case "!!": //ignored
			fmt.Printf("File ignored: %s\n", line)
		case "UU": // conflicted
			fmt.Printf("File conflict: %s\n", line)
		default:
			fmt.Printf("Unhandled file status: %s\n", line)
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
		return gitStats, fmt.Errorf("Failed to get git status --porcelain", err)
	}
	gitStats.NFilesAdded,
		gitStats.NFilesRemoved,
		gitStats.NFilesModified,
		gitStats.NFilesUnstaged = GitFileStatus(string(statusPorcelainOut))

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

func PrettyGitStats(g GitStats, f FormatStats) string {
	var pad string

	pad = strings.Repeat(" ", f.MaxAheadWidth-(len(strconv.Itoa(g.NCommitsAhead))+1))
	ahead := fmt.Sprintf("\u2191%d%s", g.NCommitsAhead, pad)
	if g.NCommitsAhead > 0 {
		ahead = colours.ColouredString(ahead, colours.Green)
	}

	pad = strings.Repeat(" ", f.MaxBehindWidth-(len(strconv.Itoa(g.NCommitsBehind))+1))
	behind := fmt.Sprintf("\u2193%d%s", g.NCommitsBehind, pad)
	if g.NCommitsBehind > 0 {
		behind = colours.ColouredString(behind, colours.Red)
	}

	pad = strings.Repeat(" ", f.MaxAddedWidth-(len(strconv.Itoa(g.NFilesAdded))+1))
	added := fmt.Sprintf("+%d%s", g.NFilesAdded, pad)
	if g.NFilesAdded > 0 {
		added = colours.ColouredString(added, colours.Green)
	}

	pad = strings.Repeat(" ", f.MaxRemovedWidth-(len(strconv.Itoa(g.NFilesRemoved))+1))
	removed := fmt.Sprintf("-%d%s", g.NFilesRemoved, pad)
	if g.NFilesRemoved > 0 {
		removed = colours.ColouredString(removed, colours.Red)
	}

	pad = strings.Repeat(" ", f.MaxModifiedWidth-(len(strconv.Itoa(g.NFilesModified))+1))
	modified := fmt.Sprintf("~%d%s", g.NFilesModified, pad)
	if g.NFilesModified > 0 {
		modified = colours.ColouredString(modified, colours.Yellow)
	}

	pad = strings.Repeat(" ", f.MaxUnstagedWidth-(len(strconv.Itoa(g.NFilesUnstaged))+1))
	unstaged := fmt.Sprintf("U%d%s", g.NFilesUnstaged, pad)
	if g.NFilesUnstaged > 0 {
		unstaged = colours.ColouredString(unstaged, colours.Red)
	}

	return fmt.Sprintf(
		" %s %s %s %s %s %s",
		ahead,
		behind,
		added,
		removed,
		modified,
		unstaged,
	)

}
