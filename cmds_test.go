package cmds

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

// TODO: Test idempotence.

func TestRunnerNil(t *testing.T) {
	cmd := &Command{}
	expectError(t, cmd.ParseRun(nil))
}

func TestRunnerNop(t *testing.T) {
	cmd := &Command{
		Runner: nopRunner,
	}
	expectErrorNone(t, cmd.ParseRun(nil))
}

func TestArgs(t *testing.T) {
	sentArgs := []string{"a", "b"}
	cmd := &Command{
		Runner: func(cmd *Command, recvArgs []string) error {
			expectEq(t, recvArgs, sentArgs)
			return nil
		},
	}
	expectErrorNone(t, cmd.ParseRun(sentArgs))
}

func TestArgsCmd(t *testing.T) {
	sentArgs := []string{"a", "b"}
	cmd := &Command{
		Flags: func() *flag.FlagSet {
			fset := flag.NewFlagSet("test", flag.ContinueOnError)
			fset.Bool("c", false, "")
			return fset
		}(),
		Commands: []*Command{
			{
				Name: "sub",
				Flags: func() *flag.FlagSet {
					fset := flag.NewFlagSet("sub", flag.ContinueOnError)
					fset.Bool("d", false, "")
					return fset
				}(),
				Runner: func(cmd *Command, recvArgs []string) error {
					expectEq(t, recvArgs, sentArgs)
					return nil
				},
			},
		},
	}
	args := []string{"-c", "sub", "-d"}
	args = append(args, sentArgs...)
	expectErrorNone(t, cmd.ParseRun(args))
}

func TestCmdNoFlags(t *testing.T) {
	cmd := &Command{
		Commands: []*Command{
			{
				Name: "sub0",
			},
			{
				Name: "sub1",
			},
		},
	}

	expectError(t, cmd.ParseRun(nil))
	expectError(t, cmd.ParseRun([]string{"sub0"}))
	expectError(t, cmd.ParseRun([]string{"sub1"}))

	var sub0Ran bool
	cmd.Commands[0].Runner = func(cmd *Command, args []string) error {
		sub0Ran = true
		return nil
	}
	var sub1Ran bool
	cmd.Commands[1].Runner = func(cmd *Command, args []string) error {
		sub1Ran = true
		return nil
	}

	expectError(t, cmd.ParseRun(nil))

	expectErrorNone(t, cmd.ParseRun([]string{"sub0"}))
	expectTrue(t, sub0Ran)
	expectFalse(t, sub1Ran)
	sub0Ran = false

	expectErrorNone(t, cmd.ParseRun([]string{"sub1"}))
	expectTrue(t, sub1Ran)
	expectFalse(t, sub0Ran)
	sub1Ran = false

	expectError(t, cmd.ParseRun([]string{"sub2"}))
	expectFalse(t, sub0Ran)
	expectFalse(t, sub1Ran)
}

func TestFlagsSimple(t *testing.T) {
	type flags struct {
		A bool
		B int
		C string
	}
	fl := flags{}
	cmd := &Command{
		Runner:        nopRunner,
		Flags:         refFlagSet(&fl),
		ErrorHandling: ReturnOnError,
	}

	expectErrorNone(t, cmd.ParseRun(nil))
	expectEq(t, fl, flags{})

	expectErrorNone(t, cmd.ParseRun([]string{"-a", "-b", "42", "-c", "a b"}))
	expectEq(t, fl, flags{
		A: true,
		B: 42,
		C: "a b",
	})

	out := cmd.Flags.Output()
	cmd.Flags.SetOutput(io.Discard)
	expectError(t, cmd.ParseRun([]string{"-z"}))
	cmd.Flags.SetOutput(out)
}

func TestFlagsCmd(t *testing.T) {
	type flags struct {
		A bool
	}
	type sub0Flags struct {
		B bool
	}
	type sub1Flags struct {
		C bool
	}
	fl := flags{}
	fl0 := sub0Flags{}
	fl1 := sub1Flags{}
	cmd := &Command{
		Flags:         refFlagSet(&fl),
		ErrorHandling: ReturnOnError,
		Commands: []*Command{
			{
				Name:   "sub0",
				Runner: nopRunner,
				Flags:  refFlagSet(&fl0),
			},
			{
				Name:   "sub1",
				Runner: nopRunner,
				Flags:  refFlagSet(&fl1),
			},
		},
	}

	expectErrorNone(t, cmd.ParseRun([]string{"-a", "sub0", "-b"}))
	expectTrue(t, fl.A)
	expectTrue(t, fl0.B)
	expectFalse(t, fl1.C)
	fl.A = false
	fl0.B = false
}

func TestErrReturn(t *testing.T) {
	errRun := errors.New("run error")
	cmd := &Command{
		Flags: func() *flag.FlagSet {
			fset := flag.NewFlagSet("test", flag.ContinueOnError)
			fset.Bool("a", false, "")
			fset.SetOutput(io.Discard)
			return fset
		}(),

		ErrorHandling: ReturnOnError,

		Commands: []*Command{
			{
				Name: "sub",
				Runner: func(cmd *Command, args []string) error {
					if len(args) > 0 && args[0] == "error" {
						return errRun
					}
					return nil
				},
			},
		},
	}

	err := cmd.ParseRun(nil)
	expectErrorIs(t, err, Err)
	expectErrorIs(t, err, ErrCmd)
	expectErrorNot(t, err, ErrFlag)

	err = cmd.ParseRun([]string{"invalid"})
	expectErrorIs(t, err, Err)
	expectErrorIs(t, err, ErrCmd)
	expectErrorNot(t, err, ErrFlag)

	err = cmd.ParseRun([]string{"-x"})
	expectErrorIs(t, err, Err)
	expectErrorNot(t, err, ErrCmd)
	expectErrorIs(t, err, ErrFlag)

	expectErrorNone(t, cmd.ParseRun([]string{"sub"}))

	err = cmd.ParseRun([]string{"sub", "error"})
	expectErrorIs(t, err, errRun)
	expectErrorNot(t, err, Err)
	expectErrorNot(t, err, ErrCmd)
	expectErrorNot(t, err, ErrFlag)
}

func nopRunner(*Command, []string) error {
	return nil
}

func expectTrue(t *testing.T, v bool) {
	t.Helper()
	if !v {
		t.Errorf("expected true boolean, got false")
	}
}

func expectFalse(t *testing.T, v bool) {
	t.Helper()
	if v {
		t.Errorf("expected false boolean, got true")
	}
}

func expectEq(t *testing.T, a, b any) {
	t.Helper()
	expectEqValues(t, a, b, true)
}

func expectNeq(t *testing.T, a, b any) {
	t.Helper()
	expectEqValues(t, a, b, false)
}

func expectEqValues(t *testing.T, a, b any, expectEq bool) {
	t.Helper()
	eq := reflect.DeepEqual(a, b)
	if eq && !expectEq {
		t.Errorf("expected unequal values, got %#+v == %#+v", a, b)
	}
	if !eq && expectEq {
		t.Errorf("expected equal values, got %#+v != %#+v", a, b)
	}
}

func expectError(t *testing.T, err error) {
	t.Helper()
	expectErrorValue(t, err, true)
}

func expectErrorNone(t *testing.T, err error) {
	t.Helper()
	expectErrorValue(t, err, false)
}

func expectErrorValue(t *testing.T, err error, expected bool) {
	t.Helper()
	if err != nil && !expected {
		t.Errorf("expected nil error, got \"%v\"", err)
	}
	if err == nil && expected {
		t.Errorf("expected non-nil error, got \"%v\"", err)
	}
}

func expectErrorIs(t *testing.T, err, target error) {
	t.Helper()
	expectError(t, err)
	if !errors.Is(err, target) {
		t.Errorf("expected error \"%v\", got \"%v\"", target, err)
	}
}

func expectErrorNot(t *testing.T, err, target error) {
	t.Helper()
	expectError(t, err)
	if errors.Is(err, target) {
		t.Errorf("expected error not to be \"%v\", got \"%v\"", target, err)
	}
}

// refFlagSet returns a [flag.FlagSet] with the flag names, types and default
// values obtained from the passed in flags, which should be a pointer to a
// struct that contains bool, int, string or struct values that contain just
// those fields recursively.
// The fsets argument shouldn't be set, it's there just to make writing this
// function recursively simpler.
// Fields in flags should be exported so [reflect] can reflect on them.
// Field values in flags are used as the default values for the [flag.FlagSet]
// flags.
func refFlagSet(flags any, fsets ...*flag.FlagSet) *flag.FlagSet {
	var fset *flag.FlagSet
	if len(fsets) == 0 {
		fset = flag.NewFlagSet("test", flag.ContinueOnError)
	} else {
		fset = fsets[0]
	}
	typ := reflect.TypeOf(flags)

	if k := typ.Kind(); k != reflect.Pointer {
		panic(fmt.Sprintf("expected kind \"%v\", got \"%v\"", reflect.Pointer, k))
	}

	typ = typ.Elem()
	val := reflect.ValueOf(flags).Elem()

	for i := 0; i < val.NumField(); i++ {
		name := typ.Field(i).Name
		name = strings.ToLower(name[:1]) + name[1:]
		switch fval := val.Field(i).Addr().Interface().(type) {
		case *bool:
			fset.BoolVar(fval, name, *fval, "")
		case *int:
			fset.IntVar(fval, name, *fval, "")
		case *string:
			fset.StringVar(fval, name, *fval, "")
		default:
			if val.Field(i).Kind() != reflect.Struct {
				panic(fmt.Sprintf("unhandled field type \"%v\"", reflect.TypeOf(fval)))
			}
			ptr := val.Field(i).Addr().Interface()
			_ = refFlagSet(ptr, fset)
		}
	}

	return fset
}
