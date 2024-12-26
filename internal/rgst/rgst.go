package rgst

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jobodd/rgst/internal/colours"
	"github.com/jobodd/rgst/internal/git"
	t "github.com/jobodd/rgst/internal/tree"
)

type FormatStats struct {
	maxFolderTreeWidth int
	maxBranchWidth     int
	maxGitStatsWidth   int
	maxAheadWidth      int
	maxBehindWidth     int
	maxAddedWidth      int
	maxRemovedWidth    int
	maxModifiedWidth   int
	maxUnstagedWidth   int
}

func printDirTree(root *t.Node, formatStats FormatStats) {
	t.Walk(root, func(n *t.Node) {
		leftPad := strings.Repeat("  ", n.GetDepth())
		folderTreeText := fmt.Sprintf("%s|-- %s", leftPad, n.FolderName)
		rep := formatStats.maxFolderTreeWidth - len(folderTreeText)
		treePadding := strings.Repeat(" ", rep)
		branchPadding := strings.Repeat(" ", formatStats.maxBranchWidth-len(n.GitStats.CurrentBranch))
		commitStats := prettyGitStats(n.GitStats, formatStats)

		fmt.Printf("%s%s %s%s %s\n",
			folderTreeText,
			treePadding,
			n.GitStats.CurrentBranch,
			branchPadding,
			commitStats)
	})
}

func prettyGitStats(g git.GitStats, f FormatStats) string {
	var pad string

	pad = strings.Repeat(" ", f.maxAheadWidth-(len(strconv.Itoa(g.NCommitsAhead))+1))
	ahead := fmt.Sprintf("\u2191%d%s", g.NCommitsAhead, pad)
	if g.NCommitsAhead > 0 {
		ahead = colours.ColouredString(ahead, colours.Green)
	}

	pad = strings.Repeat(" ", f.maxBehindWidth-(len(strconv.Itoa(g.NCommitsBehind))+1))
	behind := fmt.Sprintf("\u2193%d%s", g.NCommitsBehind, pad)
	if g.NCommitsBehind > 0 {
		behind = colours.ColouredString(behind, colours.Red)
	}

	pad = strings.Repeat(" ", f.maxAddedWidth-(len(strconv.Itoa(g.NFilesAdded))+1))
	added := fmt.Sprintf("+%d%s", g.NFilesAdded, pad)
	if g.NFilesAdded > 0 {
		added = colours.ColouredString(added, colours.Green)
	}

	pad = strings.Repeat(" ", f.maxRemovedWidth-(len(strconv.Itoa(g.NFilesRemoved))+1))
	removed := fmt.Sprintf("-%d%s", g.NFilesRemoved, pad)
	if g.NFilesRemoved > 0 {
		removed = colours.ColouredString(removed, colours.Red)
	}

	pad = strings.Repeat(" ", f.maxModifiedWidth-(len(strconv.Itoa(g.NFilesModified))+1))
	modified := fmt.Sprintf("~%d%s", g.NFilesModified, pad)
	if g.NFilesModified > 0 {
		modified = colours.ColouredString(modified, colours.Yellow)
	}

	pad = strings.Repeat(" ", f.maxUnstagedWidth-(len(strconv.Itoa(g.NFilesUnstaged))+1))
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

func MainProcess(path string, command string, recurseDepth uint, shouldFetch bool) error {
	maxDirLength := 0

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	targetDir := filepath.Base(absolutePath)

	node := t.NewNode(targetDir, absolutePath, nil)
	getGitDirectories(node, 0, recurseDepth, &maxDirLength)
	t.FilterNodes(node)

	formatStats := collectStats(node)
	printDirTree(node, formatStats)

	return nil
}

func collectStats(root *t.Node) FormatStats {
	var formatStats FormatStats
	t.Walk(root, func(n *t.Node) {
		n.FolderTreeWidth = len(n.FolderName) + 4 + (n.GetDepth() * 2)
		if n.FolderTreeWidth > formatStats.maxFolderTreeWidth {
			formatStats.maxFolderTreeWidth = n.FolderTreeWidth
		}

		if n.IsGitRepo {
			gitStats, err := getGitStats(n.AbsPath)
			if err != nil {
				panic(err)
			}
			n.GitStats = gitStats
			updateGitFormat(&formatStats, n.GitStats)

			branchNameLen := len(gitStats.CurrentBranch)
			if branchNameLen > formatStats.maxBranchWidth {
				formatStats.maxBranchWidth = branchNameLen
			}
			statsWidth := gitStats.StatsLen()
			if statsWidth > formatStats.maxBranchWidth {
				formatStats.maxBranchWidth = statsWidth
			}
		}
	})

	return formatStats
}

func updateGitFormat(formatStats *FormatStats, gitStats git.GitStats) {
	//NOTE: adds one to account for the unicode char
	widthAhead := len(strconv.Itoa(gitStats.NCommitsAhead)) + 1
	widthBehind := len(strconv.Itoa(gitStats.NCommitsBehind)) + 1
	widthAdded := len(strconv.Itoa(gitStats.NFilesAdded)) + 1
	widthRemoved := len(strconv.Itoa(gitStats.NFilesRemoved)) + 1
	widthModified := len(strconv.Itoa(gitStats.NFilesModified)) + 1
	widthUnstaged := len(strconv.Itoa(gitStats.NFilesUnstaged)) + 1

	if widthAhead > formatStats.maxAheadWidth {
		formatStats.maxAheadWidth = widthAhead
	}
	if widthBehind > formatStats.maxBehindWidth {
		formatStats.maxBehindWidth = widthBehind
	}
	if widthAdded > formatStats.maxAddedWidth {
		formatStats.maxAddedWidth = widthAdded
	}
	if widthRemoved > formatStats.maxRemovedWidth {
		formatStats.maxRemovedWidth = widthRemoved
	}
	if widthModified > formatStats.maxModifiedWidth {
		formatStats.maxModifiedWidth = widthModified
	}
	if widthUnstaged > formatStats.maxUnstagedWidth {
		formatStats.maxUnstagedWidth = widthUnstaged
	}
}

func getGitDirectories(node *t.Node, depth uint, recurseDepth uint, maxDirLength *int) {
	if depth > recurseDepth {
		return
	}

	// check for the initial node
	if node.Parent == nil {
		dirPath := filepath.Join(node.AbsPath)
		gitPath := filepath.Join(dirPath, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			node.IsGitRepo = true
		}
	}

	entries, err := os.ReadDir(node.AbsPath)
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(node.AbsPath, entry.Name())
			gitPath := filepath.Join(dirPath, ".git")

			childNodePtr := t.NewNode(entry.Name(), dirPath, node)
			node.Children = append(node.Children, childNodePtr)
			if _, err := os.Stat(gitPath); err == nil {
				childNodePtr.IsGitRepo = true
			}

			getGitDirectories(childNodePtr, depth+1, recurseDepth, maxDirLength)
		}
	}

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

func padText(s string, maxDirLength int) string {
	// remove newlines
	s = strings.Replace(s, "\n", "", -1)

	diff := maxDirLength - len(s)
	switch true {
	case diff > 0:
		padding := strings.Repeat(" ", diff)
		return s + padding
	case diff == 0:
		return s
	default:
		fmt.Printf("Failed to pad: %s", s)
		panic("Attempting to pad a string longer than the pad length")
	}
}

func isGitDirectory(basePath string, directory fs.DirEntry) (bool, error) {
	fp := filepath.Join(basePath, directory.Name(), ".git")
	info, err := os.Stat(fp)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func gitDirHasSingleRemote(absDir string) (bool, error) {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = absDir
	remoteOutput, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to get current branch: %w", err)
	}
	wc := len(strings.Split(string(remoteOutput), "\n"))

	return wc == 3, nil
}

func getGitStats(absDir string) (git.GitStats, error) {
	var gitStats git.GitStats

	gitStats.CurrentBranch = strings.ReplaceAll(getGitBranch(absDir), "\n", "")
	if hasSingle, err := gitDirHasSingleRemote(absDir); err != nil {
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
		gitStats.NFilesUnstaged = gitFileStatus(string(statusPorcelainOut))

	return gitStats, nil
}

func gitFileStatus(statusOutput string) (int, int, int, int) {
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
