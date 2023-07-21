package main

import (
	"flag"
	"fmt"
	"go.uber.org/zap"
	"sync"
	"testing"
)

func TestLoadBMP(t *testing.T) {
	fmt.Println(ImageColorConvert(LoadBMP(-500, -500)))
}

func TestClient_Login(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	browser := NewBrowser(logger.With(zap.String("browser", "test")))

	minX, minY := flag.Int("minX", -500, "Min X"), flag.Int("minY", -500, "Min Y")
	maxX, maxY := flag.Int("maxX", 0, "Max X"), flag.Int("maxY", 0, "Max Y")

	board := NewBoard(Point{*minX, *minY}, Point{*maxX, *maxY})
	worker := NewWorker(board)

	clients := readClients(logger, browser)

	var login sync.WaitGroup

	for _, client := range clients {
		login.Add(1)
		go func(c *Client) {
			err := c.Login(board, &login)
			if err != nil {
				clients = removeClient(clients, c)
			}
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
	go worker.Run()

	for {
	}
}
