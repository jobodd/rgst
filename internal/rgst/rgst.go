package rgst

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"text/tabwriter"

	"github.com/jobodd/rgst/internal/git"
	t "github.com/jobodd/rgst/internal/tree"
)

type Options struct {
	Path          string
	RecurseDepth  uint
	GitOptions    git.GitOptions
	FilterOptions t.FilterOptions
}

func MainProcess(opts Options) error {

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	maxDirLength := 0

	// figure out the base path
	absolutePath, err := filepath.Abs(opts.Path)
	if err != nil {
		panic(err)
	}
	targetDir := filepath.Base(absolutePath)

	// create the directory node structure
	node := t.NewNode(targetDir, absolutePath, nil)
	t.GetGitDirectories(node, 0, opts.RecurseDepth, &maxDirLength)
	if t.FilterNodes(node, opts.FilterOptions) == nil {
		return nil
	}

	if opts.GitOptions.ShouldFetch || opts.GitOptions.ShouldFetchAll || opts.GitOptions.ShouldPull {
		updateGitRepos(node, opts.GitOptions)
	}

	// update the git stats for each directory
	collectGitStats(node, opts.GitOptions)
	folderTabCount := 8
	if opts.GitOptions.ShowMergeBase {
		folderTabCount += 2
	}
	printDirTree(w, node, opts.GitOptions, folderTabCount)
	w.Flush()

	return nil
}

func updateGitRepos(root *t.Node, gitOptions git.GitOptions) {
	var wg sync.WaitGroup

	t.Walk(root, func(n *t.Node) {
		if n.IsGitRepo {
			wg.Add(1)
			go func() {
				defer wg.Done()
				git.UpdateDirectory(n.AbsPath, gitOptions)
			}()
		}
	})
	wg.Wait()
}

func collectGitStats(root *t.Node, gitOpts git.GitOptions) {
	t.Walk(root, func(n *t.Node) {
		n.FolderTreeWidth = len(n.FolderName) + 4 + (n.GetDepth() * 2)

		if n.IsGitRepo {
			gitStats, err := git.GetGitStats(n.AbsPath, gitOpts)
			if err != nil {
				panic(err)
			}
			n.GitStats = gitStats

		}
	})
}

func printDirTree(w *tabwriter.Writer, root *t.Node, gitOpts git.GitOptions, folderTabCount int) {
	t.Walk(root, func(n *t.Node) {
		leftPad := strings.Repeat("  ", n.GetDepth())
		folderTreeText := fmt.Sprintf("%s|-- %s", leftPad, n.FolderName)
		commitStats := git.PrettyGitStats(n.GitStats, gitOpts)

		var line string
		if n.IsGitRepo {
			line = fmt.Sprintf("%s\t%s\t%s", folderTreeText, n.GitStats.CurrentBranch, commitStats)
		} else {
			line = fmt.Sprintf("%s%s", folderTreeText, strings.Repeat("\t", folderTabCount))
		}
		fmt.Fprintln(w, line)

		// check if we want to print files as well
		if gitOpts.ShowFiles {
			if len(n.GitStats.ChangedFiles) > 0 {
				for _, line := range n.GitStats.ChangedFiles {
					fileLine := fmt.Sprintf("%s   |-- %s", leftPad, line)
					fmt.Fprintln(w, fileLine)
				}
			}
		}
	})
}
