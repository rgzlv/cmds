/*
Example command that defines 2 sub-commands, echo and req.

Echo outputs it's arguments and capitalizes them based on the flags.

Req makes a HTTP request with the method in flags and the URL in arguments.
*/
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rgzlv/cmds"
)

type rootFlags struct {
	verbose bool

	echo echoFlags
	req  reqFlags
}

type echoFlags struct {
	capitalize bool
}

type reqFlags struct {
	method string
}

func main() {
	flags := rootFlags{}
	cmd := &cmds.Command{
		Name: filepath.Base(os.Args[0]),

		Flags: func() *flag.FlagSet {
			fset := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ExitOnError)
			fset.BoolVar(&flags.verbose, "v", false, "verbose output")
			return fset
		}(),

		Runner: func(cmd *cmds.Command, args []string) error {
			return nil
		},

		Commands: []*cmds.Command{
			{
				Name: "echo",

				Flags: func() *flag.FlagSet {
					fset := flag.NewFlagSet("echo", flag.ExitOnError)
					fset.BoolVar(&flags.echo.capitalize, "c", false, "capitalize output")
					return fset
				}(),

				Runner: func(cmd *cmds.Command, args []string) error {
					if flags.verbose {
						log.Println("echoing output")
					}

					for _, arg := range args {
						if flags.echo.capitalize {
							fmt.Println(strings.ToUpper(arg))
						} else {
							fmt.Println(arg)
						}
					}

					return nil
				},
			},
			{
				Name: "req",

				Flags: func() *flag.FlagSet {
					fset := flag.NewFlagSet("echo", flag.ExitOnError)
					fset.StringVar(&flags.req.method, "m", "GET", "HTTP request method")
					return fset
				}(),

				Runner: func(cmd *cmds.Command, args []string) error {
					if len(args) != 1 {
						return errors.New("expected URL argument")
					}

					var reqFunc func(string) (*http.Response, error)
					switch flags.req.method {
					case "GET", "get":
						reqFunc = http.Get
					case "HEAD", "head":
						reqFunc = http.Head
					default:
						return fmt.Errorf("unrecognized HTTP method \"%s\"", args[0])
					}

					if flags.verbose {
						log.Println("making http request")
					}

					resp, err := reqFunc(args[0])
					if err != nil {
						return err
					}
					defer resp.Body.Close()
					b, err := io.ReadAll(resp.Body)
					if err != nil {
						return err
					}
					fmt.Println(string(b))

					return nil
				},
			},
		},
	}
	cmd.Flags.Usage = cmd.DefaultUsage()
	for _, cmd := range cmd.Commands {
		cmd.Flags.Usage = cmd.DefaultUsage()
	}

	if err := cmd.ParseRun(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
