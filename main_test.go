package main

import (
	"flag"
	"fmt"
	"go.uber.org/zap"
	"testing"
)

func TestLoadBMP(t *testing.T) {
	fmt.Println(ImageColorConvert(LoadBMP()))
}

func TestClient_Login(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	browser := NewBrowser(logger.With(zap.String("browser", "test")))

	minX, minY := flag.Int("minX", 0, "Min X"), flag.Int("minY", 0, "Min Y")
	maxX, maxY := flag.Int("maxX", 0, "Max X"), flag.Int("maxY", 0, "Max Y")

	clients := readClients(logger, NewBoard(Point{*minX, *minY}, Point{*maxX, *maxY}), browser)

	for _, client := range clients {
		client.Login()
		client.Save()
	}

	clients[0].Place(Point{-480, 499}, Colors[31])

	writeClients(clients...)

	for {
	}
}
