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
				Usage:       "Fetch the latest changes from remote",
				Destination: &rgstOpts.ShouldFetch,
			},
			&cli.BoolFlag{
				Name:        "pull",
				Aliases:     []string{"p"},
				Usage:       "Pull the latest changes from remote",
				Destination: &rgstOpts.ShouldPull,
			},
			&cli.BoolFlag{
				Name:        "show-files",
				Aliases:     []string{},
				Usage:       "Show the list of files changed for each git directory",
				Destination: &rgstOpts.ShowFiles,
			},
			&cli.StringFlag{
				Name:        "regular-expression",
				Aliases:     []string{"e"},
				Usage:       "Filter directories with an regular expression",
				Value:       "",
				Destination: &rgstOpts.RegExp,
			},
			&cli.BoolFlag{
				Name:        "invert-match",
				Aliases:     []string{"v"},
				Usage:       "Invert the regular expression match",
				Destination: &rgstOpts.ShouldInvertRegExp,
			},
			// &cli.StringFlag{
			// 	Name:        "command",
			// 	Aliases:     []string{"r"},
			// 	Usage:       "Run a git command in each git directory",
			// 	Value:       "",
			// 	Destination: &rgstOpts.Command,
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
