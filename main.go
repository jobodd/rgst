package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
	isGitRepo  bool
	parent     *Node
	children   []*Node
}

//	func (n Node) GetParentPath() string {
//		if n.parent == nil {
//			return n.folderName
//		} else {
//			return n.parent.GetFullPath() + "/"
//		}
//	}
//
//	func (n Node) GetFullPath() string {
//		return n.GetParentPath() + n.folderName
//	}
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

	node := NewNode(targetDir, absolutePath, nil)
	getGitDirectories(node, 0, recurseDepth, &maxDirLength)
	FilterNodes(node)

	maxTreeWidth := GetTreePadding(node)
	Walk(node, func(n *Node) {
		leftPad := strings.Repeat("  ", n.GetDepth())
		text := fmt.Sprintf("%s|-- %s", leftPad, n.folderName)
		if n.isGitRepo {
			branch := getGitBranch(n.absPath)
			if err != nil {
				log.Fatal(err)
			}
			text = text + strings.Repeat(" ", maxTreeWidth-len(text)) + " git! "+ strings.ReplaceAll(branch, "\n", "")
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

	branch, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		log.Fatal(err)
	}

	// err = os.Chdir("..")
	return string(branch)
}

func checkGitStatus(gitDirectory string, shouldFetch bool, maxDirLength int, maxBranchLength int) error {
	branch := getGitBranch(gitDirectory)

	err := os.Chdir(gitDirectory)
	if err != nil {
		log.Fatal(err)
	}
	if shouldFetch {
		if err := exec.Command("git", "fetch").Run(); err != nil {
			return err
		}
	}

	status := checkUpToDate()
	// status = strings.Replace(status, "\n", "", -1)
	changes := checkChangesToCommit()
	// changes = strings.Replace(changes, "\n", "", -1)
	err = os.Chdir("..")

	// print outputs
	paddedDir := padText(gitDirectory, maxDirLength)
	paddedBranch := padText(branch, maxBranchLength)
	fmt.Printf("├──  %s %s %s %s\n", paddedDir, paddedBranch, status, changes)

	return nil
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

const MAX_STATUS_LENGTH = 14

func checkUpToDate() string {
	status := ""
	cmd := exec.Command("git", "diff", "--quiet", "@{upstream}", "HEAD")
	if err := cmd.Run(); err != nil {
		//TODO: differentiate between not up to date and no upstream at all
		status = padText("not up to date", MAX_STATUS_LENGTH)
		status = fmt.Sprintf("%s%s%s  ", red, status, reset)
	} else {
		status = padText("up to date", MAX_STATUS_LENGTH)
		status = fmt.Sprintf("%s%s%s  ", green, status, reset)
	}
	return status
}

func checkChangesToCommit() string {
	cmd := exec.Command("git", "diff-index", "--quiet", "HEAD", "--")
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("%schanges to commit%s", red, reset)
	} else {
		return fmt.Sprintf("%sno changes to commit%s", green, reset)
	}
}
