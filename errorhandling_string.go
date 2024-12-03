// Code generated by "stringer -type ErrorHandling"; DO NOT EDIT.

package cmds

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ReturnOnError-0]
	_ = x[ExitOnError-1]
	_ = x[PanicOnError-2]
}

const _ErrorHandling_name = "ReturnOnErrorExitOnErrorPanicOnError"

var _ErrorHandling_index = [...]uint8{0, 13, 24, 36}

func (i ErrorHandling) String() string {
	if i < 0 || i >= ErrorHandling(len(_ErrorHandling_index)-1) {
		return "ErrorHandling(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _ErrorHandling_name[_ErrorHandling_index[i]:_ErrorHandling_index[i+1]]
}
