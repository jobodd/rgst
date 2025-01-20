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
				Destination: &rgstOpts.GitOptions.ShouldFetch,
			},
			&cli.BoolFlag{
				Name:        "fetch-all",
				Aliases:     []string{""},
				Usage:       "Fetch the latest changes from remote, all branches",
				Destination: &rgstOpts.GitOptions.ShouldFetch,
			},
			&cli.BoolFlag{
				Name:        "pull",
				Aliases:     []string{"p"},
				Usage:       "Pull the latest changes from remote",
				Destination: &rgstOpts.GitOptions.ShouldPull,
			},
			&cli.BoolFlag{
				Name:        "files",
				Aliases:     []string{},
				Usage:       "Show the list of files changed for each git directory",
				Destination: &rgstOpts.GitOptions.ShowFiles,
			},
			&cli.BoolFlag{
				Name:        "merge-base",
				Aliases:     []string{"m"},
				Usage:       "Show how far ahead/behind the current branch is from its merge base",
				Destination: &rgstOpts.GitOptions.ShowMergeBase,
			},
			&cli.StringFlag{
				Name:        "regular-expression",
				Aliases:     []string{"e"},
				Usage:       "Filter directories with an regular expression",
				Value:       "",
				Destination: &rgstOpts.FilterOptions.RegExp,
			},
			&cli.BoolFlag{
				Name:        "invert-match",
				Aliases:     []string{"v"},
				Usage:       "Invert the regular expression match",
				Destination: &rgstOpts.FilterOptions.ShouldInvertRegExp,
			},
		},
		Action: func(c *cli.Context) error {
			if err := checkArgs(c, &rgstOpts); err != nil {
				return err
			}
			var MAX_RECURSE_DEPTH uint = 5
			if rgstOpts.RecurseDepth > MAX_RECURSE_DEPTH {
				fmt.Printf(
					"Warning: Depth of %d exceeds max recursion limit of %d.\nLimiting to %d\n\n",
					rgstOpts.RecurseDepth,
					MAX_RECURSE_DEPTH,
					MAX_RECURSE_DEPTH,
				)
			}
			rgstOpts.RecurseDepth = min(MAX_RECURSE_DEPTH, rgstOpts.RecurseDepth)
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

	if err := checkFilterOptions(rgstOpts); err != nil {
		return err
	}

	return nil
}

func checkFilterOptions(rgstOpts *rgst.Options) error {
	if rgstOpts.FilterOptions.RegExp != "" {
		rgstOpts.FilterOptions.ShouldFilter = true
	} else if rgstOpts.FilterOptions.ShouldInvertRegExp {
		return errors.New("Can't invert without a match. (See --help for flags: --regular-expression and --invert-match)")
	} else {
		rgstOpts.FilterOptions.ShouldFilter = false
	}

	return nil
}
