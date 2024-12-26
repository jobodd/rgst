package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jobodd/rgst/internal/rgst"
	"github.com/urfave/cli/v2"
)

func main() {
	var rgstOpts rgst.Options

	app := &cli.App{
		Name:  "Recursive git status",
		Usage: "Check the status of Git repositories in subdirectories",
		Flags: []cli.Flag{
			&cli.UintFlag{
				Name:        "depth",
				Aliases:     []string{"d"},
				Usage:       "Set the recursion depth to check for git repos",
				Value:       0,
				Destination: &rgstOpts.RecurseDepth,
			},
			&cli.BoolFlag{
				Name:        "fetch",
				Aliases:     []string{"f"},
				Usage:       "Fetch the latest changes from origin",
				Destination: &rgstOpts.ShouldFetch,
			},
			// &cli.StringFlag{
			// 	Name:        "command",
			// 	Aliases:     []string{"c", "cmd"},
			// 	Usage:       "Command to run in each directory",
			// 	Value:       "git status",
			// 	Destination: &rgstOpts.command,
			// },
		},
		Action: func(c *cli.Context) error {
			if err := checkArgs(c, &rgstOpts); err != nil {
				return err
			}

			// TODO: warn user max depth exceeeded
			rgstOpts.RecurseDepth = min(5, rgstOpts.RecurseDepth)
			return rgst.MainProcess(rgstOpts)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
	}
}

func checkArgs(c *cli.Context, rgstOpts *rgst.Options) error {
	if c.Args().Len() > 1 {
		return errors.New("Too many arguments")
	}

	if c.Args().Len() == 1 {
		rgstOpts.Path = c.Args().Get(0)
	}

	return nil
}
