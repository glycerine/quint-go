//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package cli

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

// The loop polls the borrowed descriptor directly, so no worker can remain
// blocked in Read after the session ends. Only the private wake pipe changes
// flags or closes. The process host is the sole reader of its input descriptor.
type replUnixInput struct {
	fd, wakeReadFD, wakeWriteFD int
	wakeRead, wakeWrite         *os.File
	mu                          sync.Mutex
	interrupts                  uint64
	closed                      bool
	buffer                      []byte
	searched                    int
	eof                         bool
	stop                        func()
}

func newREPLFileInput(ctx context.Context, input *os.File) (replFileInput, bool, error) {
	fd, err := replInputFD(input)
	if err != nil {
		return nil, true, err
	}
	read, write, err := os.Pipe()
	if err != nil {
		return nil, true, err
	}
	r := &replUnixInput{fd: fd, wakeRead: read, wakeWrite: write, wakeReadFD: int(read.Fd()), wakeWriteFD: int(write.Fd())}
	for _, fd := range []int{r.wakeReadFD, r.wakeWriteFD} {
		if err := unix.SetNonblock(fd, true); err != nil {
			read.Close()
			write.Close()
			return nil, true, err
		}
	}
	finished := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(finished)
		r.wake()
	})
	var stopOnce sync.Once
	r.stop = func() {
		stopOnce.Do(func() {
			if !stop() {
				<-finished
			}
		})
	}
	return r, true, nil
}

func (r *replUnixInput) wake() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		// EAGAIN means another wake is pending. The event/ctx is retained
		// separately, so an extra byte is unnecessary.
		_, _ = unix.Write(r.wakeWriteFD, []byte{1})
	}
}

func (r *replUnixInput) Interrupt() {
	r.mu.Lock()
	if !r.closed {
		r.interrupts++
	}
	r.mu.Unlock()
	r.wake()
}

func (r *replUnixInput) Close() error {
	r.stop()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	readErr := r.wakeRead.Close()
	writeErr := r.wakeWrite.Close()
	if readErr != nil {
		return readErr
	}
	return writeErr
}

func (r *replUnixInput) Next(ctx context.Context) (REPLInputEvent, error) {
	var data [4096]byte
	fds := []unix.PollFd{{Fd: int32(r.fd), Events: unix.POLLIN}, {Fd: int32(r.wakeReadFD), Events: unix.POLLIN}}
	for {
		if err := ctx.Err(); err != nil {
			return REPLInputEvent{}, err
		}
		r.mu.Lock()
		if r.interrupts > 0 {
			r.interrupts--
			r.mu.Unlock()
			// Completed lines remain queued; an unfinished line is discarded.
			r.buffer = r.buffer[:bytes.LastIndexByte(r.buffer, '\n')+1]
			r.searched = 0
			return REPLInputEvent{Kind: REPLInputInterrupt}, nil
		}
		closed := r.closed
		r.mu.Unlock()
		if closed {
			return REPLInputEvent{}, os.ErrClosed
		}
		if found := bytes.IndexByte(r.buffer[r.searched:], '\n'); found >= 0 {
			end := r.searched + found
			if end >= 16*1024*1024 {
				return REPLInputEvent{}, bufio.ErrTooLong
			}
			line := strings.TrimSuffix(string(r.buffer[:end]), "\r")
			r.buffer = r.buffer[end+1:]
			r.searched = 0
			return REPLInputEvent{Line: line}, nil
		}
		r.searched = len(r.buffer)
		if len(r.buffer) >= 16*1024*1024 {
			return REPLInputEvent{}, bufio.ErrTooLong
		}
		if r.eof {
			if len(r.buffer) != 0 {
				line := strings.TrimSuffix(string(r.buffer), "\r")
				r.buffer = nil
				r.searched = 0
				return REPLInputEvent{Line: line}, nil
			}
			return REPLInputEvent{Kind: REPLInputEOF}, nil
		}
		if _, err := unix.Poll(fds, -1); err != nil {
			if err == unix.EINTR {
				continue
			}
			return REPLInputEvent{}, err
		}
		if fds[1].Revents != 0 {
			for {
				if n, err := unix.Read(r.wakeReadFD, data[:]); err != nil || n == 0 {
					break
				}
			}
			continue
		}
		if fds[0].Revents != 0 {
			n, err := unix.Read(r.fd, data[:])
			if err != nil && err != io.EOF {
				if err == unix.EINTR || err == unix.EAGAIN {
					continue
				}
				return REPLInputEvent{}, err
			}
			if n == 0 {
				r.eof = true
			} else {
				r.buffer = append(r.buffer, data[:n]...)
			}
		}
	}
}
