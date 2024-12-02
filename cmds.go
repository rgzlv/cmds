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
type ErrorHandling int

const (
	ReturnOnError ErrorHandling = iota

	// When the error is wrapped by ErrCmd, call os.Exit(3), if it's wrapped by
	// ErrFlag, call os.Exit(2), same as the flag package, otherwise, if the
	// Runner returned the error, call os.Exit(1).
	ExitOnError

	PanicOnError
)

type RunnerFunc func(cmd *Command, args []string) error

type Command struct {
	Name          string
	ShortDesc     string
	LongDesc      string
	Flags         *flag.FlagSet
	ErrorHandling ErrorHandling
	Runner        RunnerFunc

	Commands []*Command
}

func (cmd *Command) Find(name string) *Command {
	for _, sub := range cmd.Commands {
		if sub.Name == name {
			return sub
		}
	}

	return nil
}

func (cmd *Command) Parse(args []string) (*Command, []string, error) {
	leafCmd, args, err := cmd.parse(args)
	if err != nil {
		err = handleError(err, cmd.ErrorHandling)
		return nil, nil, err
	}

	return leafCmd, args, err
}

func (cmd *Command) Run(args []string) error {
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
			var flagErrorHandling flag.ErrorHandling
			if rootCmd.Flags != nil {
				flagErrorHandling = rootCmd.Flags.ErrorHandling()
			} else {
				flagErrorHandling = flag.ExitOnError
			}
			cmd.Flags = flag.NewFlagSet(cmd.Name, flagErrorHandling)
			cmd.Flags.Usage = cmd.DefaultUsage()
		}

		if err := cmd.Flags.Parse(args); err != nil {
			return nil, nil, fmt.Errorf("%w: %w", ErrFlag, err)
		}
		args = cmd.Flags.Args()

		if cmd.Name != rootCmd.Name && len(args) > 0 {
			args = args[1:]
		}

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

var Default = &Command{
	Name:          filepath.Base(os.Args[0]),
	ErrorHandling: ExitOnError,
}

func init() {
	Default.Flags = flag.CommandLine
	Default.Flags.Usage = Default.DefaultUsage()
}

func Parse() (*Command, []string, error) {
	leafCmd, args, err := Default.Parse(os.Args[1:])
	if err != nil {
		return nil, nil, err
	}

	return leafCmd, args, nil
}

func Flags() *flag.FlagSet {
	return Default.Flags
}

func Add(cmds ...*Command) {
	Default.Commands = append(Default.Commands, cmds...)
}

func Run() error {
	return Default.Run(os.Args[1:])
}

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
