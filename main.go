package main

import (
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/net/websocket"
	"os"
)

func main() {
}

func readClients(logger *zap.Logger, board *Board, browser *Browser) (clients []*Client) {
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
		client.Board = board
		client.Browser = browser
		// TODO: Proxy support
		client.Socket, _ = websocket.Dial("wss://gql-realtime-2.reddit.com/query", "", "https://hot-potato.reddit.com")
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
