package main

import (
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
	var recurseDepth uint = 0

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
				Destination: &recurseDepth,
			},
		},
		Action: func(c *cli.Context) error {
			recurseDepth = min(5, recurseDepth)
			return iterateDirectories(recurseDepth, shouldFetch)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}

func iterateDirectories(recurseDepth uint, shouldFetch bool) error {
	fmt.Printf("Will recurse to a depth of: %d\n\n", recurseDepth)

	entries, err := os.ReadDir("./")
	if err != nil {
		log.Fatal(err)
	}

	gitDirectories, maxDirLength, maxBranchLength := getGitDirectories(entries)

	for _, dir := range gitDirectories {
		err := checkGitStatus(dir, shouldFetch, maxDirLength, maxBranchLength)
		if err != nil {
			return err
		}
	}

	return nil
}

func getGitDirectories(entries []fs.DirEntry) ([]fs.DirEntry, int, int) {
	maxDirLength := 0
	maxBranchLength := 0
	var gitDirectories []fs.DirEntry
	for _, dir := range entries {
		if !dir.IsDir() {
			continue
		}
		if isGitDirectory(dir) {
			gitDirectories = append(gitDirectories, dir)
			if len(dir.Name()) > maxDirLength {
				maxDirLength = len(dir.Name())
			}

			branchName, err := getGitBranch(dir)
			if err != nil {
				log.Fatal(err)
			}
			if len(branchName) > maxBranchLength {
				maxBranchLength = len(branchName)
			}
		}
	}
	return gitDirectories, maxDirLength, maxBranchLength
}

func getGitBranch(gitDirectory fs.DirEntry) (string, error) {
	err := os.Chdir(gitDirectory.Name())
	if err != nil {
		log.Fatal(err)
	}

	branch, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()

	err = os.Chdir("..")
	return string(branch), err
}

func checkGitStatus(gitDirectory fs.DirEntry, shouldFetch bool, maxDirLength int, maxBranchLength int) error {
	branch, err := getGitBranch(gitDirectory)
	if err != nil {
		return err
	}

	err = os.Chdir(gitDirectory.Name())
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
	paddedDir := padText(gitDirectory.Name(), maxDirLength)
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

func isGitDirectory(directory fs.DirEntry) bool {
	fp := filepath.Join(directory.Name(), ".git")
	_, err := os.Stat(fp)
	return err == nil
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
