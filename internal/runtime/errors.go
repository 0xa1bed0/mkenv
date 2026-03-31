package runtime

import "fmt"

// ExitCodeError is returned when a command exits with a non-zero status code.
// Finalize handles it by calling os.Exit with the code, suppressing the error
// message (the command's own output was already streamed).
type ExitCodeError struct {
	Code int
}

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
