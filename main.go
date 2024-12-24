package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
)

const (
	green = "\033[0;32m"
	red   = "\033[0;31m"
	reset = "\033[0m"
)

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
			&cli.StringFlag{
				Name:        "path",
				Aliases:     []string{"p"},
				Usage:       "Directory to process; defaults to pwd",
				Value:       ".",
				Destination: &path,
			},
		},
		Action: func(c *cli.Context) error {
			if c.Args().Len() > 1 {
				return errors.New("Too many arguments")
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
	folderName string
	absPath    string
	parent     *Node
	children   []*Node
	isGitRepo  bool
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

func GetTreePadding(node *Node) int {
	if node == nil {
		return 0
	}

	padding := len(node.folderName) + 4 + (node.GetDepth() * 2)
	for _, child := range node.children {
		childPadding := GetTreePadding(child)
		padding = max(padding, childPadding)
	}
	return padding
}

func mainProcess(path string, command string, recurseDepth uint, shouldFetch bool) error {
	maxDirLength := 0

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		log.Fatal(err)
	}
	targetDir := filepath.Base(absolutePath)

	// ahead, behind, added, deleted, modified, err := getGitStats(absolutePath)
	// fmt.Println("Git stats")
	// fmt.Printf("ahead: %d\n", ahead)
	// fmt.Printf("behind: %d\n", behind)
	// fmt.Printf("added: %d\n", added)
	// fmt.Printf("deleted: %d\n", deleted)
	// fmt.Printf("modified: %d\n", modified)

	node := NewNode(targetDir, absolutePath, nil)
	getGitDirectories(node, 0, recurseDepth, &maxDirLength)
	FilterNodes(node)

	maxTreeWidth := GetTreePadding(node)
	// print headers
	headers := "Dir"
	headers = headers + strings.Repeat(" ", maxTreeWidth-len(headers)) + " " + "Branch"
	fmt.Println(headers)

	// print tree
	Walk(node, func(n *Node) {
		leftPad := strings.Repeat("  ", n.GetDepth())
		text := fmt.Sprintf("%s|-- %s", leftPad, n.folderName)
		if n.isGitRepo {
			branch := getGitBranch(n.absPath)
			if err != nil {
				log.Fatal(err)
			}

			text = text + strings.Repeat(" ", maxTreeWidth-len(text)) + " " + strings.ReplaceAll(branch, "\n", "")

			ahead, behind, added, deleted, modified, err := getGitStats(n.absPath)
			if err != nil {
				log.Fatal(err)
			}
			text = text + fmt.Sprintf(" A%d B%d +%d -%d ~%d", ahead, behind, added, deleted, modified)
		}
		fmt.Printf("%s\n", text)
	})

	return nil
}

func getGitDirectories(node *Node, depth uint, recurseDepth uint, maxDirLength *int) {
	if depth > recurseDepth {
		// fmt.Println("Returning")
		return
	}
	// fmt.Printf("Will read path: %s\n", node.absPath)
	entries, err := os.ReadDir(node.absPath)
	// entries, err := os.ReadDir(node.folderName)
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// fmt.Printf("%s", entry.Name())
			dirPath := filepath.Join(node.absPath, entry.Name())
			gitPath := filepath.Join(dirPath, ".git")
			// fmt.Printf("Trying: %s\n", gitPath)

			// fmt.Printf("New Child: %s\n", entry.Name())
			childNodePtr := NewNode(entry.Name(), dirPath, node)
			node.children = append(node.children, childNodePtr)
			if _, err := os.Stat(gitPath); err == nil {
				childNodePtr.isGitRepo = true
			}

			// fmt.Println("recursing")
			// recurse
			getGitDirectories(childNodePtr, depth+1, recurseDepth, maxDirLength)
		}
	}

}

func getGitBranch(gitDirectory string) string {

	err := os.Chdir(gitDirectory)
	if err != nil {
		log.Fatal(err)
	}

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = gitDirectory

	branchOutput, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	// err = os.Chdir("..")
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
		log.Fatal("Attempting to pad a string longer than the pad length")
		return s
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
func getGitStats(absDir string) (ahead int, behind int, added int, deleted int, modified int, err error) {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = absDir
	remoteOutput, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to get current branch: %w", err)
	}
	wc := len(strings.Split(string(remoteOutput), "\n"))
	if wc != 3 { // two lines ending \n gives a count of three
		//TODO:
		fmt.Printf("Remote count was: %d", wc)
		return -99, -99, -99, -99, -99, nil
	}

	// Get the current branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = absDir
	branchOutput, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to get current branch: %w", err)
	}
	currentBranch := strings.TrimSpace(string(branchOutput))
	// fmt.Printf("Current branch: %s", currentBranch)

	// Get ahead/behind count
	cmd = exec.Command("git", "rev-list", "--count", "--left-right", fmt.Sprintf("origin/%s...%s", currentBranch, currentBranch))
	cmd.Dir = absDir

	aheadBehindOutput, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to get ahead/behind count: %w", err)
	}
	parts := strings.Fields(string(aheadBehindOutput))
	if len(parts) == 2 {
		behind, _ = strconv.Atoi(parts[0])
		ahead, _ = strconv.Atoi(parts[1])
	}

	// Get lines added, deleted, and modified
	cmd = exec.Command("git", "diff", "--stat", "HEAD")
	cmd.Dir = absDir
	diffOutput, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to get diff stats: %w", err)
	}
	diffLines := strings.Split(string(diffOutput), "\n")
	if len(diffLines) > 1 {
		lastLine := diffLines[len(diffLines)-2] // Typically, the last non-blank line has the summary
		if strings.Contains(lastLine, "insertions") || strings.Contains(lastLine, "deletions") {
			words := strings.Fields(lastLine)
			for i, word := range words {
				if strings.HasSuffix(word, "insertion(+),") || strings.HasSuffix(word, "insertion(+)") {
					added, _ = strconv.Atoi(words[i-1])
				} else if strings.HasSuffix(word, "deletion(-),") || strings.HasSuffix(word, "deletion(-)") {
					deleted, _ = strconv.Atoi(words[i-1])
				} else if strings.HasPrefix(word, "modified,") {
					modified, _ = strconv.Atoi(words[i-1])
				}
			}
		}
	}

	return ahead, behind, added, deleted, modified, nil
}
