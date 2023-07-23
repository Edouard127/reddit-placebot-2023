package main

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"go.uber.org/zap"
	"sync"
)

type Browser struct {
	*zap.Logger
	mu     sync.Mutex
	free   bool // We use a free variable to determine if the client can have access to the browser since rod doesn't support multithreading
	caller *Client
	*rod.Browser
}

func NewBrowser(logger *zap.Logger) *Browser {
	return &Browser{Logger: logger, free: true, Browser: rod.New().ControlURL(launcher.New().Leakless(false).MustLaunch()).MustConnect()}
}

func (br *Browser) CanAccess() bool {
	return br.waitForAccess() // block the thread until the browser is free
}

func (br *Browser) waitForAccess() bool {
	for !br.free {
	}

	return true
}

// Request the browser for the client to use.
func (br *Browser) Request(call *Client) {
	br.mu.Lock()
	defer br.mu.Unlock()

	br.CanAccess()
	br.caller = call
	br.free = false
}

// Free the browser for other clients to use.
// MUST BE CALLED AFTER EVERY CLIENT ACTION
func (br *Browser) Free() {
	br.free = false
	br.new()
}

func (br *Browser) new() {
	br.Browser.Close()
	br.Browser = rod.New().MustConnect()
	br.free = true
}
