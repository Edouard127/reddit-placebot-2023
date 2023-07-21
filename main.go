package main

import (
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/net/websocket"
	"os"
	"sync"
)

func main() {
}

func readClients(logger *zap.Logger, browser *Browser) (clients []*Client) {
	file, err := os.Open("data/users.json")
	if err != nil {
		panic(err)
	}

	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&clients)
	if err != nil {
		fmt.Println("Error read users.json. Is empty?")
	}

	for _, client := range clients {
		client.Logger = logger.With(zap.String("username", client.Username))
		client.Browser = browser
		// TODO: Proxy support
		client.Socket, _ = websocket.Dial("wss://gql-realtime-2.reddit.com/query", "", "https://hot-potato.reddit.com")
		client.AssignedData = NewCircularQueue[Pair[Point, Color]](0) // dynamic
	}

	return
}

func writeClients(clients ...*Client) {
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

func removeClient(clients []*Client, client *Client) []*Client {
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
