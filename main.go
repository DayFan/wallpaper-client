package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DayFan/wallpaper-client/wallpaper"
)

type Config struct {
	AddrHTTP string
	AddrTCP  string
	Secret   string
	Path     string
	Username string
}

type Task struct {
	ImageName string
	Path      string
	Timeout   uint64
}

type Client struct {
	Conn  net.Conn
	Type  string
	Value string
	Tasks []Task
	Stop  chan bool
}

//Auth function for authentication on server
func (client *Client) Auth(secretString string) {
	secretBytes := []byte(secretString)
	res := make([]byte, 2)

	_, err := client.Conn.Write(secretBytes)
	if err != nil {
		log.Fatalf("Connection problem when upon attempt send secret word. %s\n", err.Error())
	}

	_, err = client.Conn.Read(res)
	if err != nil {
		log.Fatalf("Error when upon attempt send secret word. %s\n", err.Error())
	}

	if !bytes.Equal(res, []byte("OK")) {
		log.Fatalf("Authentication error. The response from server does not contain 'OK'\n")
	}

	log.Println("Authentication was successful")
}

//GetTasks waiting for incoming tasks
//task string is 'image_name:timeout;'
func (client *Client) GetTasks() error {
	var taskIndex int64
	tasks := make([]byte, 0)
	buf := make([]byte, 64)

	readedBytes, err := client.Conn.Read(buf)

	if len(client.Tasks) > 1 {
		client.Stop <- true
	}

	for readedBytes <= 64 && readedBytes != 0 {
		tasks = append(tasks, buf[:readedBytes]...)

		client.Conn.SetReadDeadline(time.Now().Add(time.Second * 2))
		readedBytes, err = client.Conn.Read(buf)
	}

	client.Conn.SetReadDeadline(time.Time{})

	if err != nil {
		if neterr, ok := err.(net.Error); !(ok && neterr.Timeout()) {
			return err
		}
	}

	client.Tasks = make([]Task, 0)
	for _, t := range strings.Split(string(tasks), ";") {
		tmp := strings.Split(t, ":")

		if len(tmp) != 2 {
			log.Printf("Not engouth arguments in string - (%s)\n", t)
			continue
		}

		timeout, err := strconv.ParseUint(tmp[1], 10, 64)
		if err != nil {
			continue
		}

		client.Tasks = append(client.Tasks, Task{ImageName: tmp[0], Timeout: timeout})
		go client.LoadImage(taskIndex)
		taskIndex++
	}

	err = client.SendOK()
	if err != nil {
		fmt.Println(err.Error())
	}

	return nil
}

//SendOK function for send to server 'OK' after loading task
func (client *Client) SendOK() error {
	_, err := client.Conn.Write([]byte("OK"))
	return err
}

//LoadImage function for loading images from http server and stored in task.Value
func (client *Client) LoadImage(taskIndex int64) {
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
	var taskIndex int64
	var timeout time.Duration

	lastTaskIndex := int64(len(client.Tasks) - 1)
	currentTask := client.Tasks[0]

	for {
		if currentTask.Path != "" {
			if err := wallpaper.Set(currentTask.Path); err != nil {
				log.Fatalf("Can't set a wallpaper. %s\n", err.Error())
			}

			timeout = time.Second * time.Duration(currentTask.Timeout)
		} else {
			timeout = time.Millisecond * 100
		}

		if taskIndex == lastTaskIndex {
			taskIndex = 0
		} else {
			taskIndex++
		}

		currentTask = client.Tasks[taskIndex]

		select {
		case <-client.Stop:
			log.Println("New task list received. Stop current list")
			return
		case <-time.After(timeout):
		}
	}
}

//StartTask function for set on task
func (client *Client) StartTask() {
	for client.Tasks[0].Path == "" {
		time.Sleep(time.Millisecond * 100)
	}

	if err := wallpaper.Set(client.Tasks[0].Path); err != nil {
		log.Fatalf("Can't set a wallpaper. %s\n", err.Error())
	}
}

var config Config

func init() {
	flag.StringVar(&config.AddrTCP, "tcp", "localhost:8008", "Address to tcp server.")
	flag.StringVar(&config.AddrHTTP, "http", "http://localhost:5000", "Address to http server.")
	flag.StringVar(&config.Username, "user", "", "Username on service")
	flag.StringVar(&config.Secret, "secret", "", "Secret word to connect to the server.")
	flag.StringVar(&config.Path, "path", os.TempDir(), "Path to directory where will be store downloaded images.")
}

func main() {
	flag.Parse()

	if _, err := os.Stat(config.Path); err != nil {
		log.Fatalln(err.Error())
	}

	conn, err := net.Dial("tcp", config.AddrTCP)
	if err != nil {
		log.Fatalf("Dial TCP failed. %s\n", err.Error())
	}

	defer conn.Close()

	client := Client{Conn: conn, Stop: make(chan bool, 1)}

	client.Auth(config.Secret)

	for {
		if err := client.GetTasks(); err != nil {
			log.Fatalf("Get task failed. %s\n", err.Error())
		}

		switch len(client.Tasks) {
		case 0:
		case 1:
			go client.StartTask()
		default:
			go client.StartTasks()
		}
	}
}
