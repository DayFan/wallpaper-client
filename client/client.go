package client

import (
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
	Conn  net.Conn
	Type  string
	Value string
	Tasks []task.Task
	Stop  chan bool
	Mutex sync.Mutex
}

//Auth function for authentication on server
func (client *Client) Auth(secretString string) AuthError {
	secretBytes := []byte(secretString)
	res := make([]byte, 2)
	result := AuthError{}

	_, result.err = client.Conn.Write(secretBytes)
	if result.err != nil {
		return result
	}

	_, result.err = client.Conn.Read(res)

	if result.err != nil {
		return result
	}

	switch string(res) {
	case "NO":
		result.err = errors.New("Wrong secret word")
		result.WrongSecret = true
	case "OK":
		log.Println("Authentication was successful")
	default:
		result.err = fmt.Errorf("Unidentified response from the server to the secret word. Response: '%s'", res)
	}

	return result
}

//GetTasks waiting for incoming tasks
//task string is 'image_name:timeout;'
func (client *Client) GetTasks(addrHTTP *string, path *string) error {
	var index uint

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
	for _, str := range stringOfTasks {
		tmp := strings.Split(str, ":")

		if len(tmp) != 2 {
			log.Printf("Incorrect task format - (%s)\n", str)
			continue
		}

		timeout, err := strconv.ParseUint(tmp[1], 10, 64)
		if err != nil {
			continue
		}

		tasks[index].ImageName = tmp[0]
		tasks[index].Timeout = timeout
		go tasks[index].LoadImage(addrHTTP, path)

		index++
	}

	err = client.SendOK()
	if err != nil {
		return err
	}

	if index > 0 {
		client.Mutex.Lock()
		client.Tasks = tasks[:index]
		client.Stop <- true
		client.Mutex.Unlock()
	}

	return nil
}

//SendOK function for send to server 'OK' after loading task
func (client *Client) SendOK() error {
	_, err := client.Conn.Write([]byte("OK"))
	return err
}

//StartTasks function for looped tasks
func (client *Client) StartTasks(wg *sync.WaitGroup) {
	var taskIndex int
	var timeout time.Duration

	defer wg.Done()

	for {
		client.Mutex.Lock()

		if len(client.Tasks) > 0 {
			if client.Tasks[taskIndex].Path == "" {
				timeout = time.Millisecond * 100
			} else if err := client.Tasks[taskIndex].Set(); err != nil {
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

func (client *Client) Connect(addrTCP string, secret string) (error, bool) {
	conn, err := net.Dial("tcp", addrTCP)
	if err != nil {
		return err, false
	}

	client.Conn = conn

	if authError := client.Auth(secret); authError.err != nil {
		client.Conn.Close()
		client.Conn = nil
		return authError.err, authError.WrongSecret
	}

	return nil, false
}
