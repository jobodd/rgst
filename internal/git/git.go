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

func getAheadBehindRemote(absDir string, currentBranch string) (ahead int, behind int) {
	cmd := exec.Command("git",
		"rev-list",
		"--count",
		"--left-right",
		fmt.Sprintf("origin/%s...%s",
			currentBranch,
			currentBranch))
	cmd.Dir = absDir

	aheadBehindOutput, err := cmd.CombinedOutput()
	if err != nil {
		// no remote
		return -1, -1
		// if strings.Contains(string(aheadBehindOutput), "unknown revision or path not in the working tree") {
		// 	// no remote
		// 	return gitStats, fmt.Errorf(string(aheadBehindOutput))
		// }
		// return gitStats, fmt.Errorf("failed to get ahead/behind count for remote.\nBranch was: %s\nError was: %w", gitStats.CurrentBranch, err)
	}
	parts := strings.Fields(string(aheadBehindOutput))
	if len(parts) != 2 {
		log.Fatal("Something went wrong")
		return -1, -1
	}
	ahead, err1 := strconv.Atoi(parts[0])
	behind, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		log.Fatal(fmt.Sprintf("Something went wrong getting the ahead/behind for remote.\nAhead gave: %s\nBehind gave: %s\n", err1, err2))
	}
	return ahead, behind
}

func getAheadBehindBranched(absDir string, currentBranch string) (ahead int, behind int) {
	masterBranch := "master"
	cmd := exec.Command("git", "merge-base", masterBranch, currentBranch)
	cmd.Dir = absDir
	cmdOut, err := cmd.CombinedOutput()
	if err != nil {
		return -1, -1
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
			return -1, -1
		} else {
			behind, err := strconv.Atoi(strings.TrimSpace(string(cmdOut)))
			if err != nil {
				return -1, -1
			}
			return -1, behind
		}
	}
}

func GetGitStats(absDir string) (GitStats, error) {
	var gitStats GitStats

	gitStats.CurrentBranch = getGitBranch(absDir)
	nRemotes := countRemotes(absDir)
	//TODO: change this to a count of the remotes
	gitStats.HasRemote = nRemotes == 1

	gitStats.CurrentBranch = getGitBranch(absDir)

	gitStats.CommitsBehindRemote, gitStats.CommitsAheadOfRemote =
		getAheadBehindRemote(absDir, gitStats.CurrentBranch)

	gitStats.CommitsAheadOfBranch, gitStats.CommitsBehindBranch =
		getAheadBehindBranched(absDir, gitStats.CurrentBranch)

	gitStats.ChangedFiles = getChangedFiles(absDir)

	gitStats.FilesAddedCount,
		gitStats.FilesRemovedCount,
		gitStats.FilesModifiedCount,
		gitStats.FilesUnstagedCount = parsePorcelain(gitStats.ChangedFiles)

	return gitStats, nil
}

func getChangedFiles(absDir string) (changedFiles []string) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = absDir
	statusPorcelainOut, err := cmd.Output()
	if err != nil {
		log.Fatal("something went wrong getting file changelist")
		return []string{}
	}
	porcelainStatus := strings.TrimRight(string(statusPorcelainOut), " \n")
	changedFiles = strings.Split(porcelainStatus, "\n")
	if len(changedFiles) == 1 && changedFiles[0] == "" {
		changedFiles = []string{}
	} else {
		for i := 0; i < len(changedFiles); i++ {
			changedFiles[i] = fmt.Sprintf(
				"[%s]%s",
				changedFiles[i][0:2],
				changedFiles[i][2:],
			)
		}
	}
	return changedFiles
}

func parsePorcelain(porcelainLines []string) (int, int, int, int) {
	added, removed, modified, unstaged := 0, 0, 0, 0
	//TODO: this was rushed; sanity check these
	for _, line := range porcelainLines {
		switch line[1] {
		case '!':
			log.Fatalf("Unhandled file status: `%s`\n", line)
		case 'M', 'T', 'R', 'C':
			modified++
		case 'A', '?':
			added++
		case 'D':
			removed++
		case ' ':
		// no staged changes
		default:
			fmt.Printf("Unhandled file status in the index: `%s`\n", line)
		}

		switch line[2] {
		case '!':
			log.Fatalf("Unhandled file status: `%s`\n", line)
		case 'M', 'T', 'R', 'C', 'A', 'D', '?', 'U':
			unstaged++
		case ' ':
		// no unstaged changes
		default:
			log.Fatalf("Unhandled file status in the working tree: `%s`\n", line)
		}

	}

	return added, removed, modified, unstaged
}

func PrettyGitStats(g GitStats) string {
	nAheadRemote := fmt.Sprintf("\u2191%d", g.CommitsAheadOfRemote)
	if g.CommitsAheadOfRemote == -1 {
		nAheadRemote = "-"
	}
	if g.CommitsAheadOfRemote > 0 {
		nAheadRemote = colours.ColouredString(nAheadRemote, colours.Green)
	} else {
		nAheadRemote = colours.ColouredString(nAheadRemote, colours.White)
	}

	nBehindRemote := fmt.Sprintf("\u2193%d", g.CommitsBehindRemote)
	if g.CommitsBehindRemote == -1 {
		nBehindRemote = "-"
	}
	if g.CommitsBehindRemote > 0 {
		nBehindRemote = colours.ColouredString(nBehindRemote, colours.Red)
	} else {
		nBehindRemote = "-"
		nBehindRemote = colours.ColouredString(nBehindRemote, colours.White)
	}

	nBehindBranch := fmt.Sprintf("\u2190 %d", g.CommitsBehindBranch)
	if g.CommitsBehindBranch == -1 {
		nBehindBranch = "-"
	}
	if g.CommitsBehindBranch > 0 {
		nBehindBranch = colours.ColouredString(nBehindBranch, colours.Red)
	} else {
		nBehindBranch = colours.ColouredString(" ", colours.White)
	}

	nAheadBranch := fmt.Sprintf("\u2192%d", g.CommitsAheadOfBranch)
	if g.CommitsAheadOfBranch == -1 {
		nAheadBranch = "-"
	}
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
