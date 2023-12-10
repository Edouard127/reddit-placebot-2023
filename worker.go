package main

import (
	"github.com/Edouard127/redditplacebot/board"
	"github.com/Edouard127/redditplacebot/client"
	"sync"
	"time"
)

type Worker struct {
	waitList map[*client.Client]time.Time
	clients  []*client.Client

	ticker     *time.Ticker
	board      *board.Board
	clientLock sync.Mutex
}

func NewWorker(b *board.Board) (k *Worker) {
	return &Worker{
		waitList: make(map[*client.Client]time.Time, 0),
		clients:  make([]*client.Client, 0),
		ticker:   time.NewTicker(time.Second),
		board:    b,
	}
}

func (k *Worker) ClientJoin(client ...*client.Client) {
	k.clientLock.Lock()
	defer k.clientLock.Unlock()

	k.board.SetController(client[0])

	k.clients = append(k.clients, client...)
}

func (k *Worker) Run() {
	for {
		select {
		case <-k.ticker.C:
			changed := k.board.GetDifferentData()
			if len(changed) > 0 {
				split := splitMap(changed, len(k.clients))
				for i, c := range k.clients {
					c.Assign(split[i])

					if t, ok := k.waitList[c]; ok {
						if !t.After(time.Now()) {
							c.Logger.Info("Placing c after wait")
							k.waitList[c] = c.Place(k.board)
						}
					} else {
						c.Logger.Info("Placing c")
						k.waitList[c] = c.Place(k.board)
					}
				}
			}
		}
	}
}

func splitMap(data map[board2.Point]board2.Color, n int) []map[board2.Point]board2.Color {
	split := make([]map[board2.Point]board2.Color, n)
	i := 0
	for k, v := range data {
		split[i%n] = make(map[board2.Point]board2.Color)
		split[i%n][k] = v
		i++
	}
	return split
}
