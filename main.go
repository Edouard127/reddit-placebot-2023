package main

import (
	context "context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
	"golang.org/x/net/websocket"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"sync"
	"time"
)

func main() {
	logger, _ := zap.NewDevelopment()
	browser := NewBrowser(logger.With(zap.String("browser", "test")))
	defer browser.Browser.Close()

	minX, minY := flag.Int("minX", 0, "Min X"), flag.Int("minY", 0, "Min Y")
	flag.Parse()

	board := NewBoard(Point{*minX, *minY})
	worker := NewWorker(board)

	clients := readClients(logger, browser)

	var login sync.WaitGroup

	dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:9050", nil, proxy.Direct)
	if err != nil {
		panic(err)
	}

	jar, _ := cookiejar.New(nil)
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		},
		Jar: jar,
	}

	for _, client := range clients {
		login.Add(1)
		go func(c *Client) {
			c.HTTP = httpClient
			err := c.Login(board, &login)
			if err != nil {
				clients = removeClient(clients, c)
			}

			go listenForCircuit(time.Second*10, httpClient)
		}(client)
	}

	fmt.Println("Waiting for login to finish...")
	login.Wait()
	fmt.Println("Login finished!")

	writeClients(clients...)

	fmt.Println("Waiting for board data")
	board.WaitForData()
	fmt.Println("Board data received!")

	worker.ClientJoin(clients...)
	worker.Run()
}

func readClients(logger *zap.Logger, browser *Browser) (clients []*Client) {
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

	config, err := websocket.NewConfig("wss://gql-realtime-2.reddit.com/query", "https://hot-potato.reddit.com")
	if err != nil {
		panic(err)
	}

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

	dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:9050", nil, proxy.Direct)
	if err != nil {
		panic("I could not connect to the local tor node, is it running?")
	}

	config.Dialer = &net.Dialer{
		Resolver: &net.Resolver{
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialer.Dial(network, address)
			},
		},
	}

	if len(clients) == 0 {
		panic("No accounts found in data/users.json")
	}

	for _, client := range clients {
		client.Logger = logger.With(zap.String("username", client.Username))
		client.Browser = browser
		client.Socket, err = websocket.DialConfig(config)
		if err != nil {
			panic(err)
		}

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
