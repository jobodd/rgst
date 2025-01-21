# Recursive Git Status

I got bored of running `ls -d */ | xargs -I {} sh -c "cd {} && git status"` all the time, and so `rgst` was born.

`rgst` is a tool for showing the git status of multiple repositories at once; aiming to display all the things I
need to know about my git repos, without having to check on them each in turn.


## Installation

go install github.com/jobodd/rgst/cmd/rgst@latest

## Usage

Basic usage calls rgst on the current directory, showing the current branch, number of commits ahead/behind the remote, as well as files added/modified/removed/unstaged.
```sh
$ rgst
|-- rgst develop ↑0 ↓0 +1 -0 ~0 U1
````

`rgst` takes a single argument as a path, and optional flags
```sh
$ rgst --files ~/dev/rgst
|-- rgst develop ↑0 ↓0 +1 -0 ~0 U1 
   |-- [AM] README
````

Run against a different directory, recurse one level down
```sh
$ rgst --depth 1  ~/dev/examples 
|-- examples                                                                                          
  |-- dbms                                                                                            
    |-- mysql-server trunk  ↑0 ↓993 +0 -0 ~0 U0 
    |-- postgres     master ↑0 ↓54  +2 -0 ~0 U2 
    |-- sqlite       master ↑0 ↓1   +0 -0 ~0 U0 
  |-- languages                                                                                       
    |-- go           master ↑0 ↓0   +0 -0 ~0 U0 
    |-- rust         master ↑0 ↓192 +0 -0 ~0 U0 
  |-- ziglings.org   HEAD   ↑0 ↓115 +0 -0 ~0 U0 
```

Filter directories with regex
```sh
$ rgst --depth 1 --regex lang  ~/dev/examples
|-- examples                                                                                     
  |-- languages                                                                                  
    |-- go      master ↑0 ↓0   +0 -0 ~0 U0 
    |-- rust    master ↑0 ↓192 +0 -0 ~0 U0 

$ rgst --depth 1 --regex lang -v  ~/dev/examples
|-- examples                                                                                          
  |-- dbms                                                                                            
    |-- mysql-server trunk  ↑0 ↓993 +0 -0 ~0 U0 
    |-- postgres     master ↑0 ↓54  +2 -0 ~0 U2 
    |-- sqlite       master ↑0 ↓1   +0 -0 ~0 U0 
  |-- ziglings.org   HEAD   ↑0 ↓115 +0 -0 ~0 U0 
```

See `--help` for additional flags
```sh
$ rgst --help
NAME:
   Recursive git status - Check the status of Git repositories in subdirectories

USAGE:
   Recursive git status [global options] command [command options]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --depth value, -d value  Set the recursion depth to check for git repos. Max: 5 (default: 0)
   --fetch, -f              Fetch the latest changes from remote (default: false)
   --fetch-all,             Fetch the latest changes from remote, all branches (default: false)
   --pull, -p               Pull the latest changes from remote (default: false)
   --files                  Show the list of files changed for each git directory (default: false)
   --regex value, -e value  Filter directories with an regular expression
   --invert-match, -v       Invert the regular expression match (default: false)
   --help, -h               show help
```
