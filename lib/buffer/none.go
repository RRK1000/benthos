// Copyright (c) 2014 Ashley Jeffs
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package buffer

import (
	"sync/atomic"
	"time"

	"github.com/Jeffail/benthos/lib/metrics"
	"github.com/Jeffail/benthos/lib/types"
	"github.com/Jeffail/benthos/lib/util/service/log"
)

//------------------------------------------------------------------------------

func init() {
	Constructors["none"] = TypeSpec{
		constructor: NewEmpty,
		description: `
Selecting no buffer (default) is the lowest latency option since no extra work
is done to messages that pass through. With this option back pressure from the
output will be directly applied down the pipeline.`,
	}
}

//------------------------------------------------------------------------------

// Empty is an empty buffer, simply forwards messages on directly.
type Empty struct {
	running int32

	messagesOut chan types.Transaction
	messagesIn  <-chan types.Transaction

	closeChan chan struct{}
	closed    chan struct{}
}

// NewEmpty creates a new buffer interface but doesn't buffer messages.
func NewEmpty(config Config, log log.Modular, stats metrics.Type) (Type, error) {
	e := &Empty{
		running:     1,
		messagesOut: make(chan types.Transaction),
		closeChan:   make(chan struct{}),
		closed:      make(chan struct{}),
	}
	return e, nil
}

//------------------------------------------------------------------------------

// loop is an internal loop of the empty buffer.
func (e *Empty) loop() {
	defer func() {
		atomic.StoreInt32(&e.running, 0)

		close(e.messagesOut)
		close(e.closed)
	}()

	var open bool
	for atomic.LoadInt32(&e.running) == 1 {
		var inT types.Transaction
		select {
		case inT, open = <-e.messagesIn:
			if !open {
				return
			}
		case <-e.closeChan:
			return
		}
		select {
		case e.messagesOut <- inT:
		case <-e.closeChan:
			return
		}
	}
}

//------------------------------------------------------------------------------

// StartReceiving assigns a messages channel for the output to read.
func (e *Empty) StartReceiving(msgs <-chan types.Transaction) error {
	if e.messagesIn != nil {
		return types.ErrAlreadyStarted
	}
	e.messagesIn = msgs
	go e.loop()
	return nil
}

// TransactionChan returns the channel used for consuming messages from this
// input.
func (e *Empty) TransactionChan() <-chan types.Transaction {
	return e.messagesOut
}

// ErrorsChan returns the errors channel.
func (e *Empty) ErrorsChan() <-chan []error {
	return nil
}

// StopConsuming instructs the buffer to no longer consume data.
func (e *Empty) StopConsuming() {
	e.CloseAsync()
}

// CloseAsync shuts down the StackBuffer output and stops processing messages.
func (e *Empty) CloseAsync() {
	if atomic.CompareAndSwapInt32(&e.running, 1, 0) {
		close(e.closeChan)
	}
}

// WaitForClose blocks until the StackBuffer output has closed down.
func (e *Empty) WaitForClose(timeout time.Duration) error {
	select {
	case <-e.closed:
	case <-time.After(timeout):
		return types.ErrTimeout
	}
	return nil
}

//------------------------------------------------------------------------------
