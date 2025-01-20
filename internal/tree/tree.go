package tree

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/jobodd/rgst/internal/git"
)

type Node struct {
	FolderName      string
	AbsPath         string
	Parent          *Node
	Children        []*Node
	IsGitRepo       bool
	GitStats        git.GitStats
	FolderTreeWidth int
	BranchNameWidth int
	GitStatsWidth   int
}

type FilterOptions struct {
	ShouldFilter       bool
	Regex              string
	ShouldInvertRegExp bool
}

func NewNode(folderName, absPath string, parent *Node) *Node {
	return &Node{
		FolderName: folderName,
		AbsPath:    absPath,
		IsGitRepo:  false,
		Parent:     parent,
		Children:   []*Node{},
	}
}
func (n *Node) GetDepth() int {
	var depth int
	GetDepth(n, &depth)
	return depth
}

func GetDepth(n *Node, depthPtr *int) {
	if n.Parent == nil {
		return
	}
	(*depthPtr)++
	GetDepth(n.Parent, depthPtr)
}

func Walk(node *Node, visit func(*Node)) {
	if node == nil {
		return
	}

	visit(node)

	for _, child := range node.Children {
		Walk(child, visit)
	}
}

func FilterNodes(node *Node, filterOpts FilterOptions) *Node {
	if node == nil {
		return nil
	}

	// Filter children recursively.
	var filteredChildren []*Node
	for _, child := range node.Children {
		filteredChild := FilterNodes(child, filterOpts)
		if filteredChild != nil {
			filteredChildren = append(filteredChildren, filteredChild)
		}
	}

	// Update the node's children to the filtered list.
	node.Children = filteredChildren

	// Check if this node matches
	keepNode := false
	if node.IsGitRepo {
		keepNode = true

		if filterOpts.ShouldFilter {
			m, err := regexp.MatchString(filterOpts.Regex, node.AbsPath)
			if err != nil {
				log.Fatal(err)
			}
			if !m {
				keepNode = false
			}

			if filterOpts.ShouldInvertRegExp {
				keepNode = !keepNode
			}
		}
	}

	// Keep the node if it matches, or has any matching children
	if keepNode || len(filteredChildren) > 0 {
		return node
	}

	return nil
}

func isGitDirectory(basePath string, directory fs.DirEntry) (bool, error) {
	fp := filepath.Join(basePath, directory.Name(), ".git")
	info, err := os.Stat(fp)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func GetGitDirectories(node *Node, depth uint, recurseDepth uint, maxDirLength *int) {
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

			childNodePtr := NewNode(entry.Name(), dirPath, node)
			node.Children = append(node.Children, childNodePtr)
			if _, err := os.Stat(gitPath); err == nil {
				childNodePtr.IsGitRepo = true
			}

			GetGitDirectories(childNodePtr, depth+1, recurseDepth, maxDirLength)
		}
	}

}
