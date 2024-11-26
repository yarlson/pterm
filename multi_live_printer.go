package pterm

import (
	"atomicgo.dev/schedule"
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

var DefaultMultiPrinter = MultiPrinter{
	printers:    []LivePrinter{},
	Writer:      os.Stdout,
	UpdateDelay: time.Millisecond * 200,
	buffers:     []*syncBuffer{},
	area:        DefaultArea,
}

// syncBuffer wraps bytes.Buffer with a mutex for thread safety
type syncBuffer struct {
	mu  sync.RWMutex
	buf bytes.Buffer
}

func (sb *syncBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *syncBuffer) String() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.buf.String()
}

type MultiPrinter struct {
	IsActive    bool
	Writer      io.Writer
	UpdateDelay time.Duration

	mu       sync.RWMutex // protects printers and buffers
	printers []LivePrinter
	buffers  []*syncBuffer
	area     AreaPrinter
}

func (p *MultiPrinter) SetWriter(writer io.Writer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Writer = writer
}

func (p MultiPrinter) WithWriter(writer io.Writer) *MultiPrinter {
	p.Writer = writer
	return &p
}

func (p MultiPrinter) WithUpdateDelay(delay time.Duration) *MultiPrinter {
	p.UpdateDelay = delay
	return &p
}

func (p *MultiPrinter) NewWriter() io.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()

	buf := &syncBuffer{}
	p.buffers = append(p.buffers, buf)
	return buf
}

func (p *MultiPrinter) getString() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var buffer bytes.Buffer
	for _, b := range p.buffers {
		s := b.String()
		s = strings.Trim(s, "\n")

		parts := strings.Split(s, "\r") // only get the last override
		s = parts[len(parts)-1]

		// check if s is empty, if so get one part before, repeat until not empty
		for i := len(parts) - 1; i >= 0 && s == ""; i-- {
			s = parts[i]
		}

		s = strings.Trim(s, "\n\r")
		buffer.WriteString(s)
		buffer.WriteString("\n")
	}
	return buffer.String()
}

func (p *MultiPrinter) Start() (*MultiPrinter, error) {
	p.mu.Lock()
	p.IsActive = true
	for _, printer := range p.printers {
		printer.GenericStart()
	}
	p.mu.Unlock()

	schedule.Every(p.UpdateDelay, func() bool {
		p.mu.RLock()
		isActive := p.IsActive
		p.mu.RUnlock()

		if !isActive {
			return false
		}

		p.area.Update(p.getString())
		return true
	})

	return p, nil
}

func (p *MultiPrinter) Stop() (*MultiPrinter, error) {
	p.mu.Lock()
	p.IsActive = false
	for _, printer := range p.printers {
		printer.GenericStop()
	}
	p.mu.Unlock()

	time.Sleep(time.Millisecond * 20)
	p.area.Update(p.getString())
	p.area.Stop()

	return p, nil
}

func (p MultiPrinter) GenericStart() (*LivePrinter, error) {
	p2, _ := p.Start()
	lp := LivePrinter(p2)
	return &lp, nil
}

func (p MultiPrinter) GenericStop() (*LivePrinter, error) {
	p2, _ := p.Stop()
	lp := LivePrinter(p2)
	return &lp, nil
}
