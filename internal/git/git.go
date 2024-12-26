package git

import "strconv"

type GitStats struct {
	CurrentBranch  string
	HasRemote      bool
	NCommitsAhead  int
	NCommitsBehind int
	NFilesAdded    int
	NFilesRemoved  int
	NFilesModified int
	NFilesUnstaged int
}

func (g *GitStats) StatsLen() int {
	return len(strconv.Itoa(g.NCommitsAhead)) +
		len(strconv.Itoa(g.NCommitsBehind)) +
		len(strconv.Itoa(g.NFilesAdded)) +
		len(strconv.Itoa(g.NFilesRemoved)) +
		len(strconv.Itoa(g.NFilesModified)) +
		len(strconv.Itoa(g.NFilesUnstaged))
}
