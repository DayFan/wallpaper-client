package task

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Task struct {
	ImageName string
	Path      string
	Timeout   uint64
}

//LoadImage function for loading images from http server and stored in task.Value
func (task *Task) LoadImage(addrHTTP *string, path *string) {
	url := strings.Join([]string{*addrHTTP, "static", "images", task.ImageName}, "/")
	resp, err := http.Get(url)

	if err != nil {
		log.Fatalf("Load image error. %s\n", err.Error())
	}
	defer resp.Body.Close()

	storePath := filepath.Join(*path, task.ImageName)
	file, err := os.Create(storePath)
	if err != nil {
		log.Fatalf("Create image error. %s\n", err.Error())
	}

	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		log.Fatalf("Read image from socket error. %s\n", err.Error())
	}

	task.Path = storePath
}
