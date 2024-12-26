package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jobodd/rgst/internal/rgst"
	"github.com/urfave/cli/v2"
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
		},
		Action: func(c *cli.Context) error {
			if c.Args().Len() > 1 {
				return errors.New("Too many arguments")
			}

			if c.Args().Len() == 1 {
				path = c.Args().Get(0)
			}

			// TODO: warn user max depth exceeeded
			recurseDepth = min(5, recurseDepth)
			return rgst.MainProcess(path, command, recurseDepth, shouldFetch)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}
