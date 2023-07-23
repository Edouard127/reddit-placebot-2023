package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"sync"
	"time"
)

type CircularQueue[T any] struct {
	mu       sync.Mutex
	elements []T
	size     int
	head     int
	tail     int
}

func NewCircularQueue[T any](size int) *CircularQueue[T] {
	return &CircularQueue[T]{
		elements: make([]T, size),
		size:     size,
		head:     0,
		tail:     0,
	}
}

func (q *CircularQueue[T]) Enqueue(element ...T) *CircularQueue[T] {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.size += len(element)

	for _, e := range element {
		q.elements = append(q.elements, e)
		q.tail = (q.tail + 1) % q.size
	}

	return q
}

func (q *CircularQueue[T]) Dequeue() T {
	q.mu.Lock()
	defer q.mu.Unlock()

	element := q.elements[q.head]
	q.head = (q.head + 1) % q.size
	if q.End() {
		q.head = 0
	}
	return element
}

func (q *CircularQueue[T]) Peek() T {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.elements[q.head]
}

func (q *CircularQueue[T]) End() bool {
	return q.head == q.tail
}

type Pair[T any, U any] struct {
	First  T
	Second U
}

func listenForCircuit(interval time.Duration, current *http.Client) {
	dialer, _ := proxy.SOCKS5("tcp", "127.0.0.1:9050", nil, proxy.Direct)

	req, err := http.NewRequest("GET", "https://api.ipify.org?format=json", nil)
	if err != nil {
		fmt.Println(err)
	}
	
	resp, err := current.Do(req)
	if err != nil {
		fmt.Println(err)
	}

	var ip struct {
		Ip string `json:"ip"`
	}

	json.NewDecoder(resp.Body).Decode(&ip)

	for {
		newClient := &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialer.Dial(network, addr)
				},
			},
		}

		resp, err := newClient.Do(req)
		if err != nil {
			fmt.Println(err)
		}

		var newIp struct {
			Ip string `json:"ip"`
		}

		json.NewDecoder(resp.Body).Decode(&newIp)

		if newIp.Ip != ip.Ip {
			current = newClient
		}
		time.Sleep(interval)
	}
}
