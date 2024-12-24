package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
)

const (
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Reset  = "\033[0m"
)

func colouredInt(i int, colour string) string {
	return colouredString(strconv.Itoa(i), colour)
}
func colouredString(s string, colour string) string {
	return colour + s + Reset
}

func main() {
	var shouldFetch bool
	var recurseDepth uint
	var path string
	var command string

	app := &cli.App{
		Name:  "Recursive git status",
		Usage: "Check the status of Git repositories in subdirectories",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "fetch",
				Aliases:     []string{"f"},
				Usage:       "Fetch the latest changes from origin",
				Destination: &shouldFetch,
			},
			&cli.UintFlag{
				Name:        "depth",
				Aliases:     []string{"d"},
				Usage:       "Set the recursion depth to check for git repos",
				Value:       0,
				Destination: &recurseDepth,
			},
			&cli.StringFlag{
				Name:        "command",
				Aliases:     []string{"c", "cmd"},
				Usage:       "Command to run in each directory",
				Value:       "git status",
				Destination: &command,
			},
			// &cli.StringFlag{
			// 	Name:        "path",
			// 	Aliases:     []string{"p"},
			// 	Usage:       "Directory to process; defaults to pwd",
			// 	Value:       ".",
			// 	Destination: &path,
			// },
		},
		Action: func(c *cli.Context) error {
			if c.Args().Len() > 1 {
				return errors.New("Too many arguments")
			}

			if c.Args().Len() == 1 {
				path = c.Args().Get(0)
			}

			// TODO: warn user max depth exceeeded
			recurseDepth = min(5, recurseDepth)
			return mainProcess(path, command, recurseDepth, shouldFetch)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}

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
		ahead = colouredString(ahead, Green)
	}

	pad = strings.Repeat(" ", f.maxBehindWidth-(len(strconv.Itoa(g.nCommitsBehind))+1))
	behind := fmt.Sprintf("\u2193%d%s", g.nCommitsBehind, pad)
	if g.nCommitsBehind > 0 {
		behind = colouredString(behind, Red)
	}

	pad = strings.Repeat(" ", f.maxAddedWidth-(len(strconv.Itoa(g.nFilesAdded))+1))
	added := fmt.Sprintf("+%d%s", g.nFilesAdded, pad)
	if g.nFilesAdded > 0 {
		added = colouredString(added, Green)
	}

	pad = strings.Repeat(" ", f.maxRemovedWidth-(len(strconv.Itoa(g.nFilesRemoved))+1))
	removed := fmt.Sprintf("-%d%s", g.nFilesRemoved, pad)
	if g.nFilesRemoved > 0 {
		removed = colouredString(removed, Red)
	}

	pad = strings.Repeat(" ", f.maxModifiedWidth-(len(strconv.Itoa(g.nFilesModified))+1))
	modified := fmt.Sprintf("~%d%s", g.nFilesModified, pad)
	if g.nFilesModified > 0 {
		modified = colouredString(modified, Yellow)
	}

	pad = strings.Repeat(" ", f.maxUnstagedWidth-(len(strconv.Itoa(g.nFilesUnstaged))+1))
	unstaged := fmt.Sprintf("x%d%s", g.nFilesUnstaged, pad)
	if g.nFilesUnstaged > 0 {
		unstaged = colouredString(unstaged, Red)
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

func mainProcess(path string, command string, recurseDepth uint, shouldFetch bool) error {
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
	fmt.Println(formatStats.maxAheadWidth)
	fmt.Println(formatStats.maxBehindWidth)
	fmt.Println(formatStats.maxAddedWidth)
	fmt.Println(formatStats.maxRemovedWidth)
	fmt.Println(formatStats.maxModifiedWidth)
	fmt.Println(formatStats.maxUnstagedWidth)
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

	// Get lines added, deleted, and modified
	cmd = exec.Command("git", "diff", "--stat", "HEAD")
	cmd.Dir = absDir
	diffOutput, err := cmd.Output()
	if err != nil {
		return gitStats, fmt.Errorf("failed to get diff stats: %w", err)
	}
	diffLines := strings.Split(string(diffOutput), "\n")
	if len(diffLines) > 1 {
		lastLine := diffLines[len(diffLines)-2] // Typically, the last non-blank line has the summary
		if strings.Contains(lastLine, "insertions") || strings.Contains(lastLine, "deletions") {
			words := strings.Fields(lastLine)
			for i, word := range words {
				if strings.HasSuffix(word, "insertion(+),") || strings.HasSuffix(word, "insertion(+)") {
					gitStats.nFilesAdded, _ = strconv.Atoi(words[i-1])
				} else if strings.HasSuffix(word, "deletion(-),") || strings.HasSuffix(word, "deletion(-)") {
					gitStats.nFilesRemoved, _ = strconv.Atoi(words[i-1])
				} else if strings.HasPrefix(word, "modified,") {
					gitStats.nFilesModified, _ = strconv.Atoi(words[i-1])
				}
			}
		}
	}

	return gitStats, nil
}
