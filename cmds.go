package cmds

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// Err is the most generic error and is used to wrap all the errors returned
// by this package, like [ErrCmd] and [ErrFlag] but not the ones returned by a
// call to [RunnerFunc].
var Err = errors.New("command error")

// ErrCmd indicates an error while parsing commands.
var ErrCmd = errors.New("command parse error")

// ErrFlag indicates an error while parsing flags using [flag].
var ErrFlag = errors.New("flag parse error")

// ErrorHandling defines how [Command.Parse] behaves if parsing fails.
// This only affects errors detected in [Command.Parse] itself and whatever
// non-exported functions that it might call in this package so flag parsing
// errors that occur as a result of calling [flag.FlagSet.Parse] still use the
// error handling associated with that [flag.FlagSet].
//
//go:generate stringer -type ErrorHandling
type ErrorHandling int

const (
	ReturnOnError ErrorHandling = iota

	// When the error is wrapped by ErrCmd, call os.Exit(3), if it's wrapped by
	// ErrFlag, call os.Exit(2), same as the flag package, otherwise, if the
	// Runner returned the error, call os.Exit(1).
	ExitOnError

	PanicOnError
)

// RunnerFunc is the function that will be run for the command.
// The passed in command is the leaf command that matched and the arguments
// are the arguments that remained after flag parsing.
// These are passed automatically when running the run method, you can pass
// any other command and arguments you want if running [Parse] and invoking
// the Runner separately.
type RunnerFunc func(cmd *Command, args []string) error

// Command defines a command to run as well as groups it's sub-commands.
//
// The root command (the one that will have it's run method invoked) should
// define it's Runner if there are no Commands for it, otherwise it's unused.
// The sub-commands should define both a Name and a Runner.
// The other fields are optional.
type Command struct {
	Name          string
	ShortDesc     string
	LongDesc      string
	Flags         *flag.FlagSet
	ErrorHandling ErrorHandling
	Runner        RunnerFunc

	Commands []*Command
}

// Find finds the sub-command with the given name.
func (cmd *Command) Find(name string) *Command {
	for _, sub := range cmd.Commands {
		if sub.Name == name {
			return sub
		}
	}

	return nil
}

// Parse parses the flags and commands in args and returns the leaf command
// that mached (the last command without set Commands) as well as the arguments
// that should be passed to it.
func (cmd *Command) Parse(args []string) (*Command, []string, error) {
	leafCmd, args, err := cmd.parse(args)
	if err != nil {
		err = handleError(err, cmd.ErrorHandling)
		return nil, nil, err
	}

	return leafCmd, args, err
}

func (cmd *Command) Run(args []string) error {
	return cmd.Runner(cmd, args)
}

// ParseRun parses the flags and commands in args, same as [Parse] and then
// runs the [RunnerFunc] for the leaf command.
func (cmd *Command) ParseRun(args []string) error {
	leafCmd, args, err := cmd.Parse(args)
	if err != nil {
		return err
	}

	if leafCmd.Runner == nil {
		err := fmt.Errorf("%w: %w", ErrCmd, errors.New("nil runner"))
		err = handleError(err, cmd.ErrorHandling)
		return err
	}

	return leafCmd.Runner(leafCmd, args)
}

func (cmd *Command) parse(args []string) (*Command, []string, error) {
	rootCmd := cmd
	for {
		if cmd.Flags == nil {
			var errHandling flag.ErrorHandling
			if rootCmd.Flags != nil {
				errHandling = rootCmd.Flags.ErrorHandling()
			} else {
				errHandling = flag.ExitOnError
			}
			cmd.Flags = flag.NewFlagSet(cmd.Name, errHandling)
			cmd.Flags.Usage = cmd.DefaultUsage()
		}

		if cmd.Name != rootCmd.Name && len(args) > 0 {
			args = args[1:]
		}

		if err := cmd.Flags.Parse(args); err != nil {
			return nil, nil, fmt.Errorf("%w: %w", ErrFlag, err)
		}
		args = cmd.Flags.Args()

		// Is leaf command.
		if len(cmd.Commands) == 0 {
			return cmd, args, nil
		}

		if len(args) == 0 {
			var err error
			if cmd.Name == rootCmd.Name {
				err = errors.New("missing command")
			} else {
				err = fmt.Errorf("missing command for \"%s\"", cmd.Name)
			}
			return nil, nil, fmt.Errorf("%w: %w", ErrCmd, err)
		}

		cmd = cmd.Find(args[0])
		if cmd == nil {
			return nil, nil, fmt.Errorf("%w: %w", ErrCmd, fmt.Errorf("no such command \"%s\"", args[0]))
		}
	}
}

// Default is the default command with some convenience functions, similar to
// how the [flag] package has a [flag.CommandLine] for the default
// [flag.FlagSet].
var Default = &Command{
	Name:          filepath.Base(os.Args[0]),
	ErrorHandling: ExitOnError,
}

func init() {
	Default.Flags = flag.CommandLine
	Default.Flags.Usage = Default.DefaultUsage()
}

// Parse runs [Command.Parse] on the [Default] command.
func Parse() (*Command, []string, error) {
	leafCmd, args, err := Default.Parse(os.Args[1:])
	if err != nil {
		return nil, nil, err
	}

	return leafCmd, args, nil
}

func Run(args []string) error {
	return Default.Run(args)
}

// ParseRun runs [Command.ParseRun] on the [Default] command.
func ParseRun() error {
	return Default.ParseRun(os.Args[1:])
}

// Flags returns the [flag.FlagSet] of the [Default] command.
func Flags() *flag.FlagSet {
	return Default.Flags
}

// Add adds the commands to the [Default] command.
func Add(cmds ...*Command) {
	Default.Commands = append(Default.Commands, cmds...)
}

// DefaultUsage returns a usage message for use in [flag.FlagSet.Usage] that
// outputs the command name on the first line followed by the long description,
// the sub-command names and short descriptions on the right of the names, and
// finally the flags for the current command.
func (cmd *Command) DefaultUsage() func() {
	return func() {
		var w io.Writer
		if cmd.Flags != nil {
			w = cmd.Flags.Output()
		} else {
			w = os.Stderr
		}

		if cmd.Name == "" {
			fmt.Fprintf(w, "Usage:\n")
		} else {
			fmt.Fprintf(w, "Usage of %s:\n", cmd.Name)
		}

		if cmd.LongDesc != "" {
			fmt.Fprintf(w, "\n%s\n", cmd.LongDesc)
		}

		if len(cmd.Commands) > 0 {
			var longest int
			for _, cmd := range cmd.Commands {
				if l := len(cmd.Name); l > longest {
					longest = l
				}
			}

			fmt.Fprintf(w, "\nCommands:\n")
			for _, sub := range cmd.Commands {
				if sub.Name != "" {
					fmt.Fprintf(w, "  %-*s  %s\n", longest+1, sub.Name, sub.ShortDesc)
				}
			}
		}

		if cmd.Flags != nil {
			var longest int
			cmd.Flags.VisitAll(func(f *flag.Flag) {
				if l := len(f.Name); l > longest {
					longest = l
				}
			})

			if longest != 0 {
				fmt.Fprintf(w, "\nFlags:\n")

				cmd.Flags.VisitAll(func(f *flag.Flag) {
					// So that flags with and without usage string are aligned equally.
					usage := f.Usage
					if usage != "" {
						usage += " "
					}

					fmt.Fprintf(w, "  -%-*s  %s(default: %s)\n", longest+1, f.Name, usage, f.DefValue)
				})
			}
		}
	}
}

func handleError(err error, errorHandling ErrorHandling) error {
	if errors.Is(err, ErrCmd) || errors.Is(err, ErrFlag) {
		err = fmt.Errorf("%w: %w", Err, err)
	}

	switch errorHandling {
	case ExitOnError:
		log.Println(err)
		if errors.Is(err, ErrCmd) {
			os.Exit(3)
		}
		if errors.Is(err, ErrFlag) {
			os.Exit(2)
		}
		os.Exit(1)
	case PanicOnError:
		panic(err)
	}

	return err
}
