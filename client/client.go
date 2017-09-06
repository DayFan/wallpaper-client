package client

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DayFan/wallpaper-client/task"
)

type AuthError struct {
	err         error
	WrongSecret bool
}

type Client struct {
	Conn        net.Conn
	Type        string
	Value       string
	Tasks       []task.Task
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
func (client *Client) GetTasks(addrHTTP *string, path *string) error {
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
	tasks := make([]task.Task, len(stringOfTasks))
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

		tasks[index] = task.Task{ImageName: tmp[0], Timeout: timeout}
		go tasks[index].LoadImage(addrHTTP, path)
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

//StartTasks function for looped tasks
func (client *Client) StartTasks() {
	var taskIndex int
	var timeout time.Duration

	for {
		client.Mutex.Lock()

		if len(client.Tasks) > 0 {
			if err := client.Tasks[taskIndex].Set(); err != nil {
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

func (client *Client) CreateTasksFromLocalDir(path string, timeout uint64) {
	var filename string

	if path == os.TempDir() {
		return
	}

	files, _ := ioutil.ReadDir(path)
	for _, f := range files {
		filename = f.Name()
		if !f.IsDir() && strings.HasSuffix(filename, ".jpg") {
			client.Tasks = append(client.Tasks, task.Task{ImageName: filename, Timeout: timeout, Path: filepath.Join(path, filename)})
		}
	}
}

func (client *Client) Connect(addrTCP string, secret string) error {
	var conn net.Conn
	var err error
	for {
		conn, err = net.Dial("tcp", addrTCP)
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

	errAuth := client.Auth(secret)
	if errAuth.WrongSecret {
		log.Printf("Authentication failed. %s\n", errAuth.err.Error())
		client.WrongSecret = errAuth.WrongSecret
		return errAuth.err
	}

	log.Println("Authentication was successful")

	return nil
}
