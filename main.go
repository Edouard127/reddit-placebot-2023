package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/Edouard127/redditplacebot/board"
	"github.com/Edouard127/redditplacebot/client"
	"github.com/Edouard127/redditplacebot/util"
	"go.uber.org/zap"
	"net/http"
	"nhooyr.io/websocket"
	"os"
	"sync"
)

func main() {
	logger, _ := zap.NewDevelopment()
	browser := client.NewBrowser()
	defer browser.Browser.Close()

	minX, minY := flag.Int("minX", 0, "Min X"), flag.Int("minY", 0, "Min Y")
	flag.Parse()

	worker := NewWorker(board.NewBoard(board.Point{X: *minX, Y: *minY}))

	clients := readClients(logger, browser)

	var wg sync.WaitGroup

	for _, c := range clients {
		wg.Add(1)
		go func(c *client.Client) {
			err := c.Login(&wg)
			if err != nil {
				clients = removeClient(clients, c)
			}
		}(c)
	}

	fmt.Println("Waiting for wg to finish...")
	wg.Wait()
	fmt.Println("Login finished!")

	writeClients(clients...)

	worker.ClientJoin(clients...)
	worker.Run()
}

func readClients(logger *zap.Logger, browser *client.Browser) (clients []*client.Client) {
	file, err := os.Open("data/users.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			os.Create("data/users.json")
			panic("Please add users to the users.json file")
		}
		panic(err)
	}

	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&clients)
	if err != nil {
		panic(fmt.Errorf("I could not decode the users.json file: %v", err))
	}

	config := &websocket.DialOptions{
		HTTPHeader: http.Header{},
	}

	config.HTTPHeader.Add("Accept-Encoding", "gzip, deflate, br")
	config.HTTPHeader.Add("Accept-Language", "en-GB,en-US;q=0.9,en;q=0.8")
	config.HTTPHeader.Add("Cache-Control", "no-cache")
	config.HTTPHeader.Add("Pragma", "no-cache")
	config.HTTPHeader.Add("Sec-WebSocket-Extensions", "permessage-deflate; client_max_window_bits")
	config.HTTPHeader.Add("Sec-WebSocket-Key", "ito9k+J7oZkTKA3y7IS/Zw==")
	config.HTTPHeader.Add("Sec-WebSocket-Version", "13")
	config.HTTPHeader.Add("Upgrade", "websocket")
	config.HTTPHeader.Add("Origin", "https://garlic-bread.reddit.com")
	config.HTTPHeader.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36 OPR/100.0.0.0 (Edition std-2)")

	if len(clients) == 0 {
		panic("No accounts found in data/users.json")
	}

	for _, client := range clients {
		client.Logger = logger.With(zap.String("username", client.Username))
		client.Browser = browser
		client.WSconfig = config
		client.AssignedData = util.NewCircularQueue[util.Pair[board2.Point, board2.Color]](0) // dynamic
	}

	return
}

func writeClients(clients ...*client2.Client) {
	file, err := os.Create("data/users.json")
	if err != nil {
		panic(err)
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(clients)
	if err != nil {
		panic(err)
	}
}

var s sync.Mutex

func removeClient(clients []*client2.Client, client *client2.Client) []*client2.Client {
	s.Lock()
	defer s.Unlock()

	for i, c := range clients {
		if c == client {
			clients[i] = clients[len(clients)-1]
			return clients[:len(clients)-1]
		}
	}

	return clients
}
