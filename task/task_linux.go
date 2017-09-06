package task

import (
	"fmt"
	"os/exec"
	"strings"
)

func (t *Task) Set() error {
	imagePath := fmt.Sprintf("'%s'", strings.Join([]string{"file://", t.Path}, ""))
	cmd := exec.Command("gsettings", "set", "org.gnome.desktop.background", "picture-uri", imagePath)
	err := cmd.Run()
	return err
}
