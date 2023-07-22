package main

import (
	"net/http"
	"net/url"
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

func ValidateProxies(proxies []string) []string {
	c := make(chan int, len(proxies)/2)
	valid := make([]string, 0)
	for i, proxy := range proxies {
		c <- 1
		go func(i int, proxy string) {
			defer func() { <-c }()

			host, _ := url.Parse(proxy)
			client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(host)}, Timeout: 5 * time.Second}
			// We don't care about the response, we just want to know if the proxy is valid
			_, err := client.Get("https://example.com")
			if err != nil {
				return
			}
			valid = append(valid, proxy)
		}(i, proxy)
	}
	return valid
}
