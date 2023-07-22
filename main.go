package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/net/websocket"
	"net"
	"os"
	"sync"
)

func main() {
}

func readClients(logger *zap.Logger, browser *Browser, proxyReader *CircularQueue[string]) (clients []*Client) {
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

	config, _ := websocket.NewConfig("wss://gql-realtime-2.reddit.com/query", "https://hot-potato.reddit.com")
	config.Header.Add("Accept-Encoding", "gzip, deflate, br")
	config.Header.Add("Accept-Language", "en-GB,en-US;q=0.9,en;q=0.8")
	config.Header.Add("Cache-Control", "no-cache")
	config.Header.Add("Pragma", "no-cache")
	config.Header.Add("Sec-WebSocket-Extensions", "permessage-deflate; client_max_window_bits")
	config.Header.Add("Sec-WebSocket-Key", "ito9k+J7oZkTKA3y7IS/Zw==")
	config.Header.Add("Sec-WebSocket-Protocol", "graphql-ws")
	config.Header.Add("Sec-WebSocket-Version", "13")
	config.Header.Add("Upgrade", "websocket")
	config.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36 OPR/100.0.0.0 (Edition std-2)")

	config.Dialer = &net.Dialer{
		Resolver: &net.Resolver{
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return net.Dial("tcp", proxyReader.Dequeue())
			},
		},
	}

	for _, client := range clients {
		client.Logger = logger.With(zap.String("username", client.Username))
		client.Browser = browser
		client.Socket, _ = websocket.DialConfig(config)
		client.AssignedData = NewCircularQueue[Pair[Point, Color]](0) // dynamic
		client.ProxyRotation = proxyReader
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

func loadProxies() []string {
	file, err := os.Open("data/proxies.txt")
	if err != nil {
		panic(err)
	}

	defer file.Close()

	var proxies []string
	decoder := bufio.NewScanner(file)
	for decoder.Scan() {
		proxies = append(proxies, decoder.Text())
	}

	if err != nil {
		fmt.Println("Error read proxies.txt. Is empty?")
	}

	return proxies
}
