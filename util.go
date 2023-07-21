package main

import "sync"

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

func (q *CircularQueue[T]) Enqueue(element ...T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, e := range element {
		q.elements[q.tail] = e
		q.tail = (q.tail + 1) % q.size
	}
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
