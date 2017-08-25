package wallpaper

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

//Set is function for set wallpaper
func Set(path string) error {
	p := strings.Join([]string{"file://", path}, "")
	// gsettings get org.gnome.desktop.background picture-uri
	cmd := exec.Command("gsettings", "set", "org.gnome.desktop.background", "picture-uri", p)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err.Error())
	}
	fmt.Printf("Return - %s\n", out.String())

	return nil
}
