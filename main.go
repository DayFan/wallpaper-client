package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DayFan/wallpaper-client/wallpaper"
)

type AuthError struct {
	err         error
	WrongSecret bool
}

type Config struct {
	AddrHTTP string
	AddrTCP  string
	Secret   string
	Path     string
	Username string
	Timeout  uint64
}

type Task struct {
	ImageName string
	Path      string
	Timeout   uint64
}

type Client struct {
	Conn        net.Conn
	Type        string
	Value       string
	Tasks       []Task
	WrongSecret bool
	Stop        chan bool
	Mutex       sync.Mutex
}

//Auth function for authentication on server
func (client *Client) Auth(secretString string) AuthError {
	secretBytes := []byte(secretString)
	res := make([]byte, 5)
	result := AuthError{}

	_, result.err = client.Conn.Write(secretBytes)
	if result.err != nil {
		return result
	}

	_, result.err = client.Conn.Read(res)
	if bytes.Equal(res, []byte("Error")) {
		result.err = errors.New("Wrong secret word")
		result.WrongSecret = true
	}

	return result
}

//GetTasks waiting for incoming tasks
//task string is 'image_name:timeout;'
func (client *Client) GetTasks() error {
	taskBytes := make([]byte, 0)
	buf := make([]byte, 64)

	readedBytes, err := client.Conn.Read(buf)

	for readedBytes <= 64 && readedBytes != 0 {
		taskBytes = append(taskBytes, buf[:readedBytes]...)

		client.Conn.SetReadDeadline(time.Now().Add(time.Second * 2))
		readedBytes, err = client.Conn.Read(buf)
	}

	client.Conn.SetReadDeadline(time.Time{})

	if err != nil {
		if neterr, ok := err.(net.Error); !(ok && neterr.Timeout()) {
			return err
		}
	}

	stringOfTasks := strings.Split(string(taskBytes), ";")
	tasks := make([]Task, len(stringOfTasks))
	for index, t := range stringOfTasks {
		tmp := strings.Split(t, ":")

		if len(tmp) != 2 {
			log.Printf("Not engouth arguments in string - (%s)\n", t)
			continue
		}

		timeout, err := strconv.ParseUint(tmp[1], 10, 64)
		if err != nil {
			continue
		}

		tasks[index] = Task{ImageName: tmp[0], Timeout: timeout}
		go client.LoadImage(index)
	}

	err = client.SendOK()
	if err != nil {
		fmt.Println(err.Error())
	}

	client.Mutex.Lock()
	client.Tasks = tasks
	client.Stop <- true
	client.Mutex.Unlock()

	return nil
}

//SendOK function for send to server 'OK' after loading task
func (client *Client) SendOK() error {
	_, err := client.Conn.Write([]byte("OK"))
	return err
}

//LoadImage function for loading images from http server and stored in task.Value
func (client *Client) LoadImage(taskIndex int) {
	task := &client.Tasks[taskIndex]
	url := strings.Join([]string{config.AddrHTTP, "static", "images", task.ImageName}, "/")
	resp, err := http.Get(url)

	if err != nil {
		log.Fatalf("Load image error. %s\n", err.Error())
	}
	defer resp.Body.Close()

	storePath := filepath.Join(config.Path, task.ImageName)
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

//StartTasks function for looped tasks
func (client *Client) StartTasks() {
	var taskIndex int
	var timeout time.Duration

	for {
		client.Mutex.Lock()

		if len(client.Tasks) > 0 {
			if err := wallpaper.Set(client.Tasks[taskIndex].Path); err != nil {
				log.Printf("Can't set a wallpaper. %s\n", err.Error())
				timeout = time.Millisecond * 100
			} else {
				log.Println("Wallpaper was changed.")
				timeout = time.Second * time.Duration(client.Tasks[taskIndex].Timeout)
			}
		} else {
			timeout = math.MaxInt64
		}

		taskIndex++
		if taskIndex == len(client.Tasks) {
			taskIndex = 0
		}

		client.Mutex.Unlock()

		select {
		case <-client.Stop:
			log.Println("New task list received. Stop current list")
			taskIndex = 0
		case <-time.After(timeout):
		}
	}
}

func (client *Client) CreateTasksFromLocalDir() {
	var filename string

	if config.Path == os.TempDir() {
		return
	}

	files, _ := ioutil.ReadDir(config.Path)
	for _, f := range files {
		filename = f.Name()
		if !f.IsDir() && strings.HasSuffix(filename, ".jpg") {
			client.Tasks = append(client.Tasks, Task{ImageName: filename, Timeout: config.Timeout, Path: filepath.Join(config.Path, filename)})
		}
	}
}

func (client *Client) Connect() error {
	var conn net.Conn
	var err error
	for {
		conn, err = net.Dial("tcp", config.AddrTCP)
		if err != nil {
			log.Printf("Dial TCP failed. %s\n", err.Error())
			select {
			case <-time.After(time.Second * 30):
			}
		} else {
			break
		}
	}

	client.Conn = conn

	if err := client.Auth(config.Secret); err.WrongSecret {
		log.Printf("Authentication failed. %s\n", err.err.Error())
		client.WrongSecret = err.WrongSecret
		return err.err
	}

	log.Println("Authentication was successful")

	return nil
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

	client := Client{Stop: make(chan bool, 1), Mutex: sync.Mutex{}}
	client.CreateTasksFromLocalDir()

	go client.StartTasks()

	for {
		if client.WrongSecret {
			time.Sleep(time.Microsecond)
		} else if client.Conn != nil {
			if err := client.GetTasks(); err != nil {
				log.Printf("Get task failed. %s\n", err.Error())
			}
		} else {
			if err := client.Connect(); err != nil {
				log.Printf("Connect failed. %s\n", err.Error())
			}

			defer client.Conn.Close()
		}
	}
}
