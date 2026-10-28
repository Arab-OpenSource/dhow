package logs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"google.golang.org/protobuf/proto"

	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const recordHeader = 4

var (
	ErrDiskFull     = errors.New("the runner disk is full")
	ErrSpillCorrupt = errors.New("the log spill file is damaged")
)

type SpillStore interface {
	io.WriterAt
	io.ReaderAt
	Truncate(size int64) error
	Close() error
}

type spill struct {
	store SpillStore
	path  string
	read  int64
	write int64
	count int
}

func OpenSpillFile(path string) (SpillStore, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, config.PrivateFileMode) //nolint:gosec // The spill file lives in the attempt's own 0700 directory.
	if err != nil {
		return nil, fmt.Errorf("create the log spill file: %w", err)
	}
	return f, nil
}

func encodeRecord(c *runnerv1.LogChunk) ([]byte, error) {
	body, err := proto.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode a log chunk: %w", err)
	}
	out := make([]byte, recordHeader, recordHeader+len(body))
	binary.BigEndian.PutUint32(out, uint32(len(body))) //nolint:gosec // A chunk is at most 64 KiB plus a small header.
	return append(out, body...), nil
}

func decodeRecord(r io.ReaderAt, off int64, maxBody int) (*runnerv1.LogChunk, int64, error) {
	var head [recordHeader]byte
	if _, err := r.ReadAt(head[:], off); err != nil {
		return nil, 0, errors.Join(ErrSpillCorrupt, err)
	}
	n := int(binary.BigEndian.Uint32(head[:]))
	if n > maxBody {
		return nil, 0, ErrSpillCorrupt
	}
	body := make([]byte, n)
	if _, err := r.ReadAt(body, off+recordHeader); err != nil {
		return nil, 0, errors.Join(ErrSpillCorrupt, err)
	}
	var c runnerv1.LogChunk
	if err := proto.Unmarshal(body, &c); err != nil {
		return nil, 0, errors.Join(ErrSpillCorrupt, err)
	}
	return &c, int64(recordHeader + n), nil
}

func (s *spill) push(c *runnerv1.LogChunk) error {
	rec, err := encodeRecord(c)
	if err != nil {
		return err
	}
	if _, err := s.store.WriteAt(rec, s.write); err != nil {
		if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) {
			return errors.Join(ErrDiskFull, err)
		}
		return fmt.Errorf("write the log spill file: %w", err)
	}
	s.write += int64(len(rec))
	s.count++
	return nil
}

func (s *spill) pop() (*runnerv1.LogChunk, error) {
	c, n, err := decodeRecord(s.store, s.read, maxRecordBody)
	if err != nil {
		return nil, err
	}
	s.read += n
	s.count--
	if s.count == 0 {
		s.read, s.write = 0, 0
		if err := s.store.Truncate(0); err != nil {
			return nil, fmt.Errorf("reset the log spill file: %w", err)
		}
	}
	return c, nil
}

func (s *spill) close() error {
	err := s.store.Close()
	if s.path != "" {
		if rmErr := os.Remove(s.path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
	}
	return err
}
