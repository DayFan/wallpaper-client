package wallpaper

import (
	"fmt"
	"os/exec"
	"strings"
)

//Set is function for set wallpaper
func Set(path string) error {
	imagePath := fmt.Sprintf("'%s'", strings.Join([]string{"file://", path}, ""))
	cmd := exec.Command("gsettings", "set", "org.gnome.desktop.background", "picture-uri", imagePath)
	err := cmd.Run()
	return err
}
