package task

import (
	"fmt"
	"os/exec"
)

func (t *Task) Set() error {
	return exec.Command("osascript", "-e", fmt.Sprintf(`tell application "Finder" to set desktop picture to POSIX file "%s"`, t.Path)).Run()
}
