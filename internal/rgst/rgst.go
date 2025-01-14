package rgst

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	formatStats := collectGitStats(node)
	printDirTree(w, node, formatStats, opts.GitOptions)
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

func collectGitStats(root *t.Node) git.FormatStats {
	var formatStats git.FormatStats
	t.Walk(root, func(n *t.Node) {
		n.FolderTreeWidth = len(n.FolderName) + 4 + (n.GetDepth() * 2)
		if n.FolderTreeWidth > formatStats.MaxFolderTreeWidth {
			formatStats.MaxFolderTreeWidth = n.FolderTreeWidth
		}

		if n.IsGitRepo {
			gitStats, err := git.GetGitStats(n.AbsPath)
			if err != nil {
				panic(err)
			}
			n.GitStats = gitStats
			updateGitFormat(&formatStats, n.GitStats)

			branchNameLen := len(gitStats.CurrentBranch)
			if branchNameLen > formatStats.MaxBranchWidth {
				formatStats.MaxBranchWidth = branchNameLen
			}
			statsWidth := gitStats.StatsLen()
			if statsWidth > formatStats.MaxBranchWidth {
				formatStats.MaxBranchWidth = statsWidth
			}
		}
	})

	return formatStats
}

func printDirTree(w *tabwriter.Writer, root *t.Node, formatStats git.FormatStats, gitOpts git.GitOptions) {
	t.Walk(root, func(n *t.Node) {
		leftPad := strings.Repeat("  ", n.GetDepth())
		folderTreeText := fmt.Sprintf("%s|-- %s", leftPad, n.FolderName)
		commitStats := git.PrettyGitStats(n.GitStats, formatStats)

		var line string
		if n.IsGitRepo {
			line = fmt.Sprintf("%s\t%s\t%s", folderTreeText, n.GitStats.CurrentBranch, commitStats)
		} else {
			line = fmt.Sprintf("%s\t\t", folderTreeText)
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

func updateGitFormat(formatStats *git.FormatStats, gitStats git.GitStats) {
	//NOTE: adds one to account for the unicode char
	widthAhead := len(strconv.Itoa(gitStats.NCommitsAhead)) + 1
	widthBehind := len(strconv.Itoa(gitStats.NCommitsBehind)) + 1
	widthAdded := len(strconv.Itoa(gitStats.NFilesAdded)) + 1
	widthRemoved := len(strconv.Itoa(gitStats.NFilesRemoved)) + 1
	widthModified := len(strconv.Itoa(gitStats.NFilesModified)) + 1
	widthUnstaged := len(strconv.Itoa(gitStats.NFilesUnstaged)) + 1

	if widthAhead > formatStats.MaxAheadWidth {
		formatStats.MaxAheadWidth = widthAhead
	}
	if widthBehind > formatStats.MaxBehindWidth {
		formatStats.MaxBehindWidth = widthBehind
	}
	if widthAdded > formatStats.MaxAddedWidth {
		formatStats.MaxAddedWidth = widthAdded
	}
	if widthRemoved > formatStats.MaxRemovedWidth {
		formatStats.MaxRemovedWidth = widthRemoved
	}
	if widthModified > formatStats.MaxModifiedWidth {
		formatStats.MaxModifiedWidth = widthModified
	}
	if widthUnstaged > formatStats.MaxUnstagedWidth {
		formatStats.MaxUnstagedWidth = widthUnstaged
	}
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
