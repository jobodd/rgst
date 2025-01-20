package git

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/google/uuid"
)

var mainTmpDir string

var cmdsInitMaster [][]string
var cmdsFirstCommit [][]string
var cmdsCreateDevelopBranch [][]string
var cmdsCreateRemoteAndClone [][]string

func TestMain(m *testing.M) {
	// setup
	mainTmpDir = path.Join(os.TempDir(), uuid.NewString())
	os.Mkdir(mainTmpDir, 0700)
	setupCommonCommands()

	// run tests
	exitCode := m.Run()

	// can't defer as we skip this with os.Exit
	os.RemoveAll(mainTmpDir)
	os.Exit(exitCode)
}

func setupCommonCommands() {
	cmdsInitMaster = append(cmdsInitMaster, []string{"git", "init", "--initial-branch=master"})

	cmdsFirstCommit = append(cmdsFirstCommit, []string{"touch", "foo.txt"})
	cmdsFirstCommit = append(cmdsFirstCommit, []string{"git", "add", "--all"})
	cmdsFirstCommit = append(cmdsFirstCommit, []string{"git", "commit", "-m", "'first commit'"})

	cmdsCreateDevelopBranch = append(cmdsCreateDevelopBranch, []string{"git", "checkout", "-b", "develop"})
}

func createTmpSubDir() string {
	tmpDir := path.Join(mainTmpDir, uuid.NewString())
	os.Mkdir(tmpDir, 0700)
	return tmpDir
}

func setupRemoteAndClone() (tmpRemote string, tmpClone string) {
	tmpRemote = setupRemote()
	tmpClone = cloneFromRemote(tmpRemote)
	return tmpRemote, tmpClone
}
func setupRemote() (tmpRemote string) {
	tmpRemote = createTmpSubDir()
	runCmds(tmpRemote, cmdsInitMaster)
	return tmpRemote
}

func cloneFromRemote(tmpRemote string) (tmpClone string) {
	tmpClone = createTmpSubDir()
	var cmdsCloneFromRemote [][]string
	cmdsCloneFromRemote = append(cmdsCloneFromRemote, []string{"git", "clone", tmpRemote, "."})
	runCmds(tmpClone, cmdsCloneFromRemote)
	return tmpClone
}

func runCmds(absDir string, cmdsArgs [][]string) (cmdsInOut []string) {
	for _, args := range cmdsArgs {
		// fmt.Printf("Running: %s\n", args)
		cmdOut := runCmd(absDir, args[0], args[1:])
		// fmt.Println(cmdOut)
		cmdsInOut = append(cmdsInOut, fmt.Sprintf("%s -> %s", args, cmdOut))
	}
	return cmdsInOut
}

func runCmd(absGitDir string, command string, args []string) (cmdOut string) {
	cmd := exec.Command(command, args...)
	cmd.Dir = absGitDir
	cmdOutBytes, _ := cmd.CombinedOutput()
	return string(cmdOutBytes)
}

func TestGetBranchName_NoCommits(t *testing.T) {
	tmpDir := createTmpSubDir()
	defer os.RemoveAll(tmpDir)
	runCmds(tmpDir, cmdsInitMaster)

	got := getGitBranch(tmpDir)
	want := "master"
	if got != want {
		t.Fatalf(`Failed test: Got: %v, Want: %v`, got, want)
	}
}

func TestGetBranchName_Master(t *testing.T) {
	tmpDir := createTmpSubDir()
	defer os.RemoveAll(tmpDir)
	runCmds(tmpDir, cmdsInitMaster)
	runCmds(tmpDir, cmdsFirstCommit)

	got := getGitBranch(tmpDir)
	want := "master"
	if got != want {
		t.Fatalf(`Failed test: Got: %v, Want: %v`, got, want)
	}
}

func TestGetBranchName_Develop(t *testing.T) {
	tmpDir := createTmpSubDir()
	runCmds(tmpDir, cmdsInitMaster)
	runCmds(tmpDir, cmdsCreateDevelopBranch)

	got := getGitBranch(tmpDir)
	want := "develop"
	if got != want {
		t.Fatalf(`Failed test: Got: %v, Want: %v`, got, want)
	}
}

func TestCountRemotes_NoRemote(t *testing.T) {
	tmpDir := createTmpSubDir()
	runCmds(tmpDir, cmdsInitMaster)

	got := countRemotes(tmpDir)
	want := 0
	if got != want {
		t.Fatalf(`Failed test: Got: %v, Want: %v`, got, want)
	}
}

func TestCountRemotes_OneRemote(t *testing.T) {
	tmpRemote, tmpClone := setupRemoteAndClone()
	defer os.RemoveAll(tmpRemote)
	defer os.RemoveAll(tmpClone)

	got := countRemotes(tmpClone)
	want := 1
	if got != want {
		t.Fatalf(`Failed test: Got: %v, Want: %v`, got, want)
	}
}

func TestCountRemotes_TwoRemotes(t *testing.T) {
	tmpRemote1 := createTmpSubDir()
	runCmds(tmpRemote1, cmdsInitMaster)
	tmpRemote2 := createTmpSubDir()
	runCmds(tmpRemote2, cmdsInitMaster)

	tmpClone := createTmpSubDir()
	var cmdsAddRemotes [][]string
	cmdsAddRemotes = append(cmdsAddRemotes, []string{"git", "clone", tmpRemote1, "."})
	cmdsAddRemotes = append(cmdsAddRemotes, []string{"git", "remote", "add", "remote1", tmpRemote2})
	runCmds(tmpClone, cmdsAddRemotes)

	got := countRemotes(tmpClone)
	want := 2
	if got != want {
		t.Fatalf(`Failed test: Got: %v, Want: %v`, got, want)
	}
}

func TestCountRemotes_CheckStats_UncommittedChange(t *testing.T) {
	tmpDir := createTmpSubDir()
	runCmds(tmpDir, cmdsInitMaster)

	var cmds [][]string
	cmds = append(cmds, []string{"touch", "foo.txt"})
	runCmds(tmpDir, cmds)

	stats, err := GetGitStats(tmpDir)
	if err != nil {
		t.Fatalf("Failed test with error: %s", err)
	}
	fmt.Println(stats)

	got := stats.FilesUnstagedCount
	want := 1
	if got != want {
		t.Fatalf(`Failed test: Got: %v, Want: %v. GitStats was: %+v`, got, want, stats)
	}
}

func TestUnstaged(t *testing.T) {
	changedFiles := []string{
		"[ U] foo.txt",
	}

	_, _, _, unstaged := parsePorcelain(changedFiles)

	got := unstaged
	want := 1
	if got != want {
		t.Fatalf(`Failed test: Got: %v, Want: %v`, got, want)
	}

}

func TestCommit(t *testing.T) {
	changedFiles := []string{
		"[A ] foo.txt",
	}

	added, _, _, _ := parsePorcelain(changedFiles)

	got := added
	want := 1
	if got != want {
		t.Fatalf(`Failed test: Got: %v, Want: %v`, got, want)
	}
}

func TestCommitAndChange(t *testing.T) {
	changedFiles := []string{
		"[AU] foo.txt",
	}

	added, _, _, unstaged := parsePorcelain(changedFiles)
	got := added
	want := 1
	if got != want {
		t.Fatalf(`Failed test: Got %v added, Want: %v`, got, want)
	}
	got = unstaged
	want = 1
	if got != want {
		t.Fatalf(`Failed test: Got %v added, Want: %v`, got, want)
	}
}
