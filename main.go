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
				Value: 0,
				Destination: &recurseDepth,
			},
			&cli.StringFlag{
				Name:        "command",
				Aliases:     []string{"c", "cmd"},
				Usage:       "Command to run in each directory",
				Value: "git status",
				Destination: &command,
			},
			&cli.StringFlag{
				Name:        "path",
				Aliases:     []string{"p"},
				Usage:       "Directory to process; defaults to pwd",
				Value: ".",
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


type pathTo struct {
	base string
	name string
}

func mainProcess(path string, command string, recurseDepth uint, shouldFetch bool) error {
	// fmt.Printf("Will recurse to a depth of: %d\n\n", recurseDepth)
	// fmt.Printf("Will process from base path: %s\n", path)
	// fmt.Printf("Will run command: %s\n", command)

	maxDirLength := 0
	gitDirs := getGitDirectories(path, 0, recurseDepth, &maxDirLength)
	fmt.Println(strings.Repeat("=", maxDirLength))
	fmt.Println(path)
	fmt.Println(strings.Repeat("=", maxDirLength))
	for _, gitDir := range gitDirs {
		requiredPadding := maxDirLength-len(gitDir.name)
		pad := strings.Repeat(" ", requiredPadding)
		fmt.Printf("%s%s%s:\n", gitDir.base, gitDir.name, pad)
	}


	return nil
}

func getGitDirectories(basePath string, depth uint, recurseDepth uint, maxDirLength *int) ([]pathTo) {
	if depth > recurseDepth {
		return []pathTo{}
	}

	var gitDirectories []pathTo
	entries, err := os.ReadDir(basePath)
	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(basePath, entry.Name())
			gitPath := filepath.Join(dirPath, ".git")

			if _, err := os.Stat(gitPath); err == nil {
				gitDirectories = append(gitDirectories, pathTo{dirPath, entry.Name()})
				dirNameLength := len(entry.Name())
				if dirNameLength > *maxDirLength {
					*maxDirLength = dirNameLength
				}
			}

			gitSubDirectories := getGitDirectories(dirPath, depth+1, recurseDepth, maxDirLength)
			if len(gitDirectories) > 0 {
				gitDirectories = append(gitDirectories, gitSubDirectories...)
			}
		}
	}

	return gitDirectories
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

func isGitDirectory(basePath string, directory fs.DirEntry) ( bool, error  ){
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
