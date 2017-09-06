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
	var wg sync.WaitGroup

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

	wg.Add(1)
	go client.StartTasks(&wg)

	for {
		if client.Conn != nil {
			if err := client.GetTasks(&config.AddrHTTP, &config.Path); err != nil {
				log.Printf("Get task failed. Reason: %s\n", err.Error())
				client.Conn.Close()
				client.Conn = nil
			}
		} else {
			err, wrongSecret := client.Connect(config.AddrTCP, config.Secret)
			if err != nil {
				log.Printf("Connect failed. Reason: %s\n", err.Error())

				if wrongSecret {
					break
				}

				time.Sleep(time.Second * 30)
			}
		}
	}

	if len(client.Tasks) > 0 {
		wg.Wait()
	} else {
		log.Println("Task list is empty. Exit")
	}
}
