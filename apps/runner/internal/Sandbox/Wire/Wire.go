package wire

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"google.golang.org/protobuf/proto"

	agentv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/agent/v1"
)

const (
	ProtocolVersion = 1
	MaxFrameBytes   = 1 << 20
)

var ErrFrameTooLarge = errors.New("the control frame is larger than the protocol allows")

type Conn struct {
	r    *bufio.Reader
	w    io.Writer
	c    io.Closer
	wmu  sync.Mutex
	rhdr [4]byte
	whdr [4]byte
	rbuf []byte
}

func NewConn(rw io.ReadWriteCloser) *Conn {
	return &Conn{r: bufio.NewReaderSize(rw, 64<<10), w: rw, c: rw}
}

func (c *Conn) Read() (*agentv1.Frame, error) {
	if _, err := io.ReadFull(c.r, c.rhdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(c.rhdr[:])
	if n > MaxFrameBytes {
		return nil, ErrFrameTooLarge
	}
	if cap(c.rbuf) < int(n) {
		c.rbuf = make([]byte, n)
	}
	buf := c.rbuf[:n]
	if _, err := io.ReadFull(c.r, buf); err != nil {
		return nil, err
	}
	var f agentv1.Frame
	if err := proto.Unmarshal(buf, &f); err != nil {
		return nil, fmt.Errorf("decode a control frame: %w", err)
	}
	return &f, nil
}

func (c *Conn) Write(f *agentv1.Frame) error {
	body, err := proto.Marshal(f)
	if err != nil {
		return fmt.Errorf("encode a control frame: %w", err)
	}
	if len(body) > MaxFrameBytes {
		return ErrFrameTooLarge
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	binary.BigEndian.PutUint32(c.whdr[:], uint32(len(body))) //nolint:gosec // len(body) was just checked against MaxFrameBytes, which is well below 2^32.
	if _, err := c.w.Write(c.whdr[:]); err != nil {
		return err
	}
	if _, err := c.w.Write(body); err != nil {
		return err
	}
	return nil
}

func (c *Conn) Close() error { return c.c.Close() }

func Decode(raw []byte) (*agentv1.Frame, error) {
	var f agentv1.Frame
	if err := proto.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("decode a control frame: %w", err)
	}
	return &f, nil
}
