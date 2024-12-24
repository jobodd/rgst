package rgst

import (
	"fmt"
	"github.com/jobodd/rgst/internal/colours"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Node struct {
	folderName      string
	absPath         string
	parent          *Node
	children        []*Node
	isGitRepo       bool
	gitStats        GitStats
	folderTreeWidth int
	branchNameWidth int
	gitStatsWidth   int
}

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

type GitStats struct {
	currentBranch  string
	hasRemote      bool
	nCommitsAhead  int
	nCommitsBehind int
	nFilesAdded    int
	nFilesRemoved  int
	nFilesModified int
	nFilesUnstaged int
}

func (g *GitStats) StatsLen() int {
	return len(string(g.nCommitsAhead)) +
		len(string(g.nCommitsBehind)) +
		len(string(g.nFilesAdded)) +
		len(string(g.nFilesRemoved)) +
		len(string(g.nFilesModified)) +
		len(string(g.nFilesUnstaged))
}

func NewNode(folderName, absPath string, parent *Node) *Node {
	return &Node{
		folderName: folderName,
		absPath:    absPath,
		isGitRepo:  false,
		parent:     parent,
		children:   []*Node{},
	}
}
func (n *Node) GetDepth() int {
	var depth int
	GetDepth(n, &depth)
	return depth
}

func GetDepth(n *Node, depthPtr *int) {
	if n.parent == nil {
		return
	}
	(*depthPtr)++
	GetDepth(n.parent, depthPtr)
}

func Walk(node *Node, visit func(*Node)) {
	if node == nil {
		return
	}

	visit(node)

	for _, child := range node.children {
		Walk(child, visit)
	}
}

func FilterNodes(node *Node) *Node {
	if node == nil {
		return nil
	}

	// Filter children recursively.
	var filteredChildren []*Node
	for _, child := range node.children {
		filteredChild := FilterNodes(child)
		if filteredChild != nil {
			filteredChildren = append(filteredChildren, filteredChild)
		}
	}

	// Update the node's children to the filtered list.
	node.children = filteredChildren

	// Check if this node or any of its children have Foo set to true.
	if node.isGitRepo || len(filteredChildren) > 0 {
		return node
	}

	// If neither this node nor its children have Foo set to true, remove it.
	return nil
}

func printDirTree(root *Node, formatStats FormatStats) {
	Walk(root, func(n *Node) {
		leftPad := strings.Repeat("  ", n.GetDepth())
		folderTreeText := fmt.Sprintf("%s|-- %s", leftPad, n.folderName)
		rep := formatStats.maxFolderTreeWidth - len(folderTreeText)
		treePadding := strings.Repeat(" ", rep)
		branchPadding := strings.Repeat(" ", formatStats.maxBranchWidth-len(n.gitStats.currentBranch))
		commitStats := prettyGitStats(n.gitStats, formatStats)

		fmt.Printf("%s%s %s%s %s\n",
			folderTreeText,
			treePadding,
			n.gitStats.currentBranch,
			branchPadding,
			commitStats)
	})
}

func prettyGitStats(g GitStats, f FormatStats) string {
	var pad string

	pad = strings.Repeat(" ", f.maxAheadWidth-(len(strconv.Itoa(g.nCommitsAhead))+1))
	ahead := fmt.Sprintf("\u2191%d%s", g.nCommitsAhead, pad)
	if g.nCommitsAhead > 0 {
		ahead = colours.ColouredString(ahead, colours.Green)
	}

	pad = strings.Repeat(" ", f.maxBehindWidth-(len(strconv.Itoa(g.nCommitsBehind))+1))
	behind := fmt.Sprintf("\u2193%d%s", g.nCommitsBehind, pad)
	if g.nCommitsBehind > 0 {
		behind = colours.ColouredString(behind, colours.Red)
	}

	pad = strings.Repeat(" ", f.maxAddedWidth-(len(strconv.Itoa(g.nFilesAdded))+1))
	added := fmt.Sprintf("+%d%s", g.nFilesAdded, pad)
	if g.nFilesAdded > 0 {
		added = colours.ColouredString(added, colours.Green)
	}

	pad = strings.Repeat(" ", f.maxRemovedWidth-(len(strconv.Itoa(g.nFilesRemoved))+1))
	removed := fmt.Sprintf("-%d%s", g.nFilesRemoved, pad)
	if g.nFilesRemoved > 0 {
		removed = colours.ColouredString(removed, colours.Red)
	}

	pad = strings.Repeat(" ", f.maxModifiedWidth-(len(strconv.Itoa(g.nFilesModified))+1))
	modified := fmt.Sprintf("~%d%s", g.nFilesModified, pad)
	if g.nFilesModified > 0 {
		modified = colours.ColouredString(modified, colours.Yellow)
	}

	pad = strings.Repeat(" ", f.maxUnstagedWidth-(len(strconv.Itoa(g.nFilesUnstaged))+1))
	unstaged := fmt.Sprintf("U%d%s", g.nFilesUnstaged, pad)
	if g.nFilesUnstaged > 0 {
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

	node := NewNode(targetDir, absolutePath, nil)
	getGitDirectories(node, 0, recurseDepth, &maxDirLength)
	FilterNodes(node)

	formatStats := collectStats(node)
	printDirTree(node, formatStats)

	return nil
}

func collectStats(root *Node) FormatStats {
	var formatStats FormatStats
	Walk(root, func(n *Node) {
		n.folderTreeWidth = len(n.folderName) + 4 + (n.GetDepth() * 2)
		if n.folderTreeWidth > formatStats.maxFolderTreeWidth {
			formatStats.maxFolderTreeWidth = n.folderTreeWidth
		}

		if n.isGitRepo {
			gitStats, err := getGitStats(n.absPath)
			if err != nil {
				panic(err)
			}
			n.gitStats = gitStats
			updateGitFormat(&formatStats, n.gitStats)

			branchNameLen := len(gitStats.currentBranch)
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

func updateGitFormat(formatStats *FormatStats, gitStats GitStats) {
	//NOTE: adds one to account for the unicode char
	widthAhead := len(strconv.Itoa(gitStats.nCommitsAhead)) + 1
	widthBehind := len(strconv.Itoa(gitStats.nCommitsBehind)) + 1
	widthAdded := len(strconv.Itoa(gitStats.nFilesAdded)) + 1
	widthRemoved := len(strconv.Itoa(gitStats.nFilesRemoved)) + 1
	widthModified := len(strconv.Itoa(gitStats.nFilesModified)) + 1
	widthUnstaged := len(strconv.Itoa(gitStats.nFilesUnstaged)) + 1

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

func getGitDirectories(node *Node, depth uint, recurseDepth uint, maxDirLength *int) {
	if depth > recurseDepth {
		return
	}

	// check for the initial node
	if node.parent == nil {
		dirPath := filepath.Join(node.absPath)
		gitPath := filepath.Join(dirPath, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			node.isGitRepo = true
		}
	}

	entries, err := os.ReadDir(node.absPath)
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(node.absPath, entry.Name())
			gitPath := filepath.Join(dirPath, ".git")

			childNodePtr := NewNode(entry.Name(), dirPath, node)
			node.children = append(node.children, childNodePtr)
			if _, err := os.Stat(gitPath); err == nil {
				childNodePtr.isGitRepo = true
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

func getGitStats(absDir string) (GitStats, error) {
	var gitStats GitStats

	gitStats.currentBranch = strings.ReplaceAll(getGitBranch(absDir), "\n", "")
	if hasSingle, err := gitDirHasSingleRemote(absDir); err != nil {
		return gitStats, err
	} else {
		//TODO: handle multiple remotes
		gitStats.hasRemote = hasSingle
	}

	// Get the current branch
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = absDir
	branchOutput, err := cmd.Output()
	if err != nil {
		return gitStats, fmt.Errorf("failed to get current branch: %w", err)
	}
	gitStats.currentBranch = strings.TrimSpace(string(branchOutput))

	// Get ahead/behind count
	cmd = exec.Command("git",
		"rev-list",
		"--count",
		"--left-right",
		fmt.Sprintf("origin/%s...%s",
			gitStats.currentBranch,
			gitStats.currentBranch))
	cmd.Dir = absDir

	aheadBehindOutput, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(aheadBehindOutput), "unknown revision or path not in the working tree") {
			return gitStats, nil
		}
		return gitStats, fmt.Errorf("failed to get ahead/behind count.\nBranch was: %s\nError was: %w", gitStats.currentBranch, err)
	}
	parts := strings.Fields(string(aheadBehindOutput))
	if len(parts) == 2 {
		gitStats.nCommitsBehind, _ = strconv.Atoi(parts[0])
		gitStats.nCommitsAhead, _ = strconv.Atoi(parts[1])
	}

	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = absDir
	statusPorcelainOut, err := cmd.Output()
	if err != nil {
		return gitStats, fmt.Errorf("Failed to get git status --porcelain", err)
	}
	gitStats.nFilesAdded,
		gitStats.nFilesRemoved,
		gitStats.nFilesModified,
		gitStats.nFilesUnstaged = gitFileStatus(string(statusPorcelainOut))

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
