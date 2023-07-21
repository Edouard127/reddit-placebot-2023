package main

import (
	"go.uber.org/zap"
	"sync"
	"time"
)

type Worker struct {
	waitList map[*Client]time.Time
	clients  []*Client

	ticker     *time.Ticker
	board      *Board
	clientLock sync.Mutex
}

func NewWorker(board *Board) (k *Worker) {
	return &Worker{
		waitList: make(map[*Client]time.Time, 0),
		clients:  make([]*Client, 0),
		ticker:   time.NewTicker(time.Second),
		board:    board,
	}
}

func (k *Worker) ClientJoin(client ...*Client) {
	k.clientLock.Lock()
	defer k.clientLock.Unlock()

	k.clients = append(k.clients, client...)
}

func (k *Worker) Run() {
	for {
		select {
		case <-k.ticker.C:
			changed := k.board.GetDifferentData()
			if len(changed) > 0 {
				split := splitMap(changed, len(k.clients))
				for i, client := range k.clients {
					client.Assign(split[i])

					if t, ok := k.waitList[client]; ok {
						if !t.After(time.Now()) {
							client.Logger.Info("Placing client after wait", zap.String("client", client.Username))
							k.waitList[client] = client.Place(k.board)
						}
					} else {
						client.Logger.Info("Placing client", zap.String("client", client.Username))
						k.waitList[client] = client.Place(k.board)
					}
				}
			}
		}
	}
}

func splitMap(data map[Point]Color, n int) []map[Point]Color {
	split := make([]map[Point]Color, n)
	i := 0
	for k, v := range data {
		split[i%n] = make(map[Point]Color)
		split[i%n][k] = v
		i++
	}
	return split
}
