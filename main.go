package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DayFan/wallpaper-client/client"
)

type Config struct {
	AddrHTTP string
	AddrTCP  string
	Secret   string
	Path     string
	Username string
	Timeout  uint64
}

var config Config

func init() {
	var defaultDir = os.TempDir()

	flag.StringVar(&config.AddrTCP, "tcp", "localhost:8008", "Address to tcp server.")
	flag.StringVar(&config.AddrHTTP, "http", "http://localhost:5000", "Address to http server.")
	flag.StringVar(&config.Username, "user", "", "Username on service")
	flag.StringVar(&config.Secret, "secret", "", "Secret word to connect to the server.")
	flag.StringVar(&config.Path, "path", defaultDir, "Path to directory where will be store downloaded images.")
	flag.Uint64Var(&config.Timeout, "timeout", 30, "Only used in offline mode. Timeout in seconds for change wallpaper.")
}

func main() {
	var err error

	flag.Parse()

	if !filepath.IsAbs(config.Path) {
		if config.Path, err = filepath.Abs(config.Path); err != nil {
			log.Fatalln(err.Error())
		}
	}

	if _, err = os.Stat(config.Path); err != nil {
		log.Fatalln(err.Error())
	}

	client := client.Client{Stop: make(chan bool, 1), Mutex: sync.Mutex{}}
	client.CreateTasksFromLocalDir(config.Path, config.Timeout)

	go client.StartTasks()

	for {
		if client.WrongSecret {
			time.Sleep(time.Second)
		} else if client.Conn != nil {
			if err := client.GetTasks(&config.AddrHTTP, &config.Path); err != nil {
				log.Printf("Get task failed. Reason: %s\n", err.Error())
				client.Conn.Close()
				client.Conn = nil
			}
		} else {
			if err := client.Connect(config.AddrTCP, config.Secret); err != nil {
				log.Printf("Connect failed. Reason: %s\n", err.Error())
				time.Sleep(time.Second * 30)
			} else {
				defer client.Conn.Close()
			}
		}
	}
}
