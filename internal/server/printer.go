package server

import (
	"errors"
	"sync"

	"github.com/shlewislee/typos/internal/printer"
)

type PrinterConn struct {
	p         *printer.Printer
	mu        sync.Mutex
	connected bool
}

func NewPrinterConn(p *printer.Printer) *PrinterConn {
	return &PrinterConn{
		p:         p,
		connected: true,
	}
}

func (pc *PrinterConn) Execute(fn func(*printer.Printer) error) (err error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.connected {
		return errors.New("printer offline")
	}

	return fn(pc.p)
}

func (pc *PrinterConn) Reconnect() (s string, err error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.p.Close()

	if err = pc.p.OpenSerial(); err != nil {
		pc.connected = false
		return "", err
	}

	status, err := pc.p.Status()
	if err != nil {
		pc.connected = false
		return "", err
	}

	pc.connected = true
	return status, nil
}

func (pc *PrinterConn) Status() (string, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if !pc.connected {
		return "", errors.New("printer offline")
	}
	return pc.p.Status()
}

func (pc *PrinterConn) IsConnected() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.connected
}
