package git

import (
	"fmt"
	"log"
	// "os"
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
	CommitsAheadOfRemote int
	CommitsBehindRemote  int
	CommitsAheadOfBranch int
	CommitsBehindBranch  int
	FilesAddedCount      int
	FilesRemovedCount    int
	FilesModifiedCount   int
	FilesUnstagedCount   int
	ChangedFiles         []string
}

func (s GitStats) String() string {
	return fmt.Sprintf(
		"CurrentBranch: %s\nHasRemote: %b\nCommitsAheadOfRemote: %d",
		s.CurrentBranch,
		s.HasRemote,
		s.CommitsAheadOfRemote,
	)
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
		//TODO: add to msg
	} else {
		if len(out) == 0 {
			//TODO: add to msg
		}
	}
}

func runGitCmd(absGitDirectory string, gitArgs []string) (cmdOut string, err error) {
	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = absGitDirectory
	cmdOutBytes, err := cmd.CombinedOutput()
	return strings.Trim(string(cmdOutBytes), "\n"), err
}

func getGitBranch(absDir string) string {
	//TODO: this should never really throw an error unless we're calling it in a non-git dir
	cmdOut, err := runGitCmd(absDir, []string{"branch", "--show-current"})
	if err != nil {
		fmt.Printf("Error getting git branch: %s", cmdOut)
		log.Fatal("Error getting git branch")
	}
	return strings.TrimSpace(cmdOut)
}

func countRemotes(absDir string) int {
	cmdOut, err := runGitCmd(absDir, []string{"remote"})
	if err != nil {
		fmt.Printf("Error counting remotes. Error was: %s", err)
		log.Fatal("Error counting remotes")
	}
	remotesList := strings.Split(
		cmdOut,
		"\n",
	)
	if len(remotesList) == 1 {
		if remotesList[0] == "" {
			return 0
		}
		return 1
	}
	return len(remotesList)
}

func GetGitStats(absDir string) (GitStats, error) {
	var gitStats GitStats

	gitStats.CurrentBranch = getGitBranch(absDir)
	nRemotes := countRemotes(absDir)
	//TODO: change this to a count of the remotes
	gitStats.HasRemote = nRemotes == 1

	//TODO: pull out to func
	gitStats.CurrentBranch = getGitBranch(absDir)

	//TODO: pull out to func
	// Get ahead/behind count
	cmd := exec.Command("git",
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
		return gitStats, fmt.Errorf("failed to get ahead/behind count for remote.\nBranch was: %s\nError was: %w", gitStats.CurrentBranch, err)
	}
	parts := strings.Fields(string(aheadBehindOutput))
	if len(parts) == 2 {
		gitStats.CommitsBehindRemote, _ = strconv.Atoi(parts[0])
		gitStats.CommitsAheadOfRemote, _ = strconv.Atoi(parts[1])
	}

	//TODO: pull out to func
	// Get branch ahead/behind count
	masterBranch := "master"
	cmd = exec.Command("git", "merge-base", masterBranch, gitStats.CurrentBranch)
	cmd.Dir = absDir
	cmdOut, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed getting merge base.\nError: %s\nOut: %s", err, cmdOut))
	} else {
		mergeBase := strings.TrimSpace(string(cmdOut))
		cmd = exec.Command("git",
			"rev-list",
			"--count",
			fmt.Sprintf(
				"%s..%s",
				mergeBase,
				masterBranch,
			),
		)
		cmd.Dir = absDir
		cmdOut, err = cmd.CombinedOutput()

		if err != nil {
			fmt.Println(fmt.Sprintf("failed to get ahead/behind count for branch.\nError was: %s", err))
		} else {

			gitStats.CommitsBehindBranch, _ = strconv.Atoi(strings.TrimSpace(string(cmdOut)))
		}
	}

	//TODO: pull out to func
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

	gitStats.CommitsAheadOfBranch = -1

	gitStats.FilesAddedCount,
		gitStats.FilesRemovedCount,
		gitStats.FilesModifiedCount,
		gitStats.FilesUnstagedCount = GitFileStatus(gitStats.ChangedFiles)

	return gitStats, nil
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

func PrettyGitStats(g GitStats) string {
	nAheadRemote := fmt.Sprintf("\u2191%d", g.CommitsAheadOfRemote)
	if g.CommitsAheadOfRemote > 0 {
		nAheadRemote = colours.ColouredString(nAheadRemote, colours.Green)
	} else {
		nAheadRemote = colours.ColouredString(nAheadRemote, colours.White)
	}

	nBehindRemote := fmt.Sprintf("\u2193%d", g.CommitsBehindRemote)
	if g.CommitsBehindRemote > 0 {
		nBehindRemote = colours.ColouredString(nBehindRemote, colours.Red)
	} else {
		nBehindRemote = colours.ColouredString(nBehindRemote, colours.White)
	}

	nBehindBranch := fmt.Sprintf("\u2190 %d", g.CommitsBehindBranch)
	if g.CommitsBehindBranch > 0 {
		nBehindBranch = colours.ColouredString(nBehindBranch, colours.Red)
	} else {
		nBehindBranch = colours.ColouredString(" ", colours.White)
	}

	nAheadBranch := fmt.Sprintf("\u2192%d", g.CommitsAheadOfBranch)
	if g.CommitsAheadOfBranch > 0 {
		nAheadBranch = colours.ColouredString(nAheadBranch, colours.Green)
	} else {
		nAheadBranch = colours.ColouredString(" ", colours.White)
	}

	added := fmt.Sprintf("+%d", g.FilesAddedCount)
	if g.FilesAddedCount > 0 {
		added = colours.ColouredString(added, colours.Green)
	} else {
		added = colours.ColouredString(added, colours.White)
	}

	removed := fmt.Sprintf("-%d", g.FilesRemovedCount)
	if g.FilesRemovedCount > 0 {
		removed = colours.ColouredString(removed, colours.Red)
	} else {
		removed = colours.ColouredString(removed, colours.White)
	}

	modified := fmt.Sprintf("~%d", g.FilesModifiedCount)
	if g.FilesModifiedCount > 0 {
		modified = colours.ColouredString(modified, colours.Yellow)
	} else {
		modified = colours.ColouredString(modified, colours.White)
	}

	unstaged := fmt.Sprintf("U%d", g.FilesUnstagedCount)
	if g.FilesUnstagedCount > 0 {
		unstaged = colours.ColouredString(unstaged, colours.Red)
	} else {
		unstaged = colours.ColouredString(unstaged, colours.White)
	}

	return fmt.Sprintf(
		"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t",
		nAheadRemote,
		nBehindRemote,
		nAheadBranch,
		nBehindBranch,
		added,
		removed,
		modified,
		unstaged,
	)
}
