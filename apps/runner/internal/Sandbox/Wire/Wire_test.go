package wire

import (
	"bytes"
	"errors"
	"io"
	"testing"

	agentv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/agent/v1"
)

type pipe struct {
	io.Reader
	io.Writer
}

func (pipe) Close() error { return nil }

func TestRoundTripFramesInBothDirections(t *testing.T) {
	var buf bytes.Buffer
	c := NewConn(pipe{Reader: &buf, Writer: &buf})
	frames := []*agentv1.Frame{
		{Body: &agentv1.Frame_Hello{Hello: &agentv1.Hello{ProtocolVersion: 1, AgentVersion: "1"}}},
		{Body: &agentv1.Frame_RunStep{RunStep: &agentv1.RunStep{Index: 2, Script: "echo hi", Env: map[string]string{"A": "b"}}}},
		{Body: &agentv1.Frame_Output{Output: &agentv1.Output{StepIndex: 2, Data: []byte("hi\n")}}},
		{Body: &agentv1.Frame_StepExit{StepExit: &agentv1.StepExit{Index: 2, ExitCode: 0}}},
	}
	for _, f := range frames {
		if err := c.Write(f); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	for i := range frames {
		got, err := c.Read()
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if got.String() != frames[i].String() {
			t.Fatalf("frame %d round-tripped to %v, want %v", i, got, frames[i])
		}
	}
}

func TestReadRejectsAnOversizedFrame(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0xff, 0xff, 0xff, 0xff})
	c := NewConn(pipe{Reader: buf, Writer: io.Discard})
	if _, err := c.Read(); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("want ErrFrameTooLarge, got %v", err)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x08, 0x01})
	seed := &agentv1.Frame{Body: &agentv1.Frame_RunStep{RunStep: &agentv1.RunStep{Index: 1, Script: "x"}}}
	var b bytes.Buffer
	_ = NewConn(pipe{Reader: &b, Writer: &b}).Write(seed)
	f.Add(b.Bytes())
	f.Fuzz(func(_ *testing.T, data []byte) {
		if f, err := Decode(data); err == nil {
			_ = f.String()
		}
	})
}
