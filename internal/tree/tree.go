package tree

import "github.com/jobodd/rgst/internal/git"

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

func FilterNodes(node *Node) *Node {
	if node == nil {
		return nil
	}

	// Filter children recursively.
	var filteredChildren []*Node
	for _, child := range node.Children {
		filteredChild := FilterNodes(child)
		if filteredChild != nil {
			filteredChildren = append(filteredChildren, filteredChild)
		}
	}

	// Update the node's children to the filtered list.
	node.Children = filteredChildren

	// Check if this node or any of its children have Foo set to true.
	if node.IsGitRepo || len(filteredChildren) > 0 {
		return node
	}

	// If neither this node nor its children have Foo set to true, remove it.
	return nil
}
