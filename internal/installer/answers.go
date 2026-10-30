package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

const MaxAnswersBytes = 64 << 10

var ErrInvalidAnswers = errors.New("invalid answers")

func DecodeAnswers(r io.Reader, into any) error {
	data, err := io.ReadAll(io.LimitReader(r, MaxAnswersBytes+1))
	if err != nil {
		return fmt.Errorf("read answers: %w", err)
	}
	if len(data) > MaxAnswersBytes {
		return fmt.Errorf("%w: the file is larger than %d bytes", ErrInvalidAnswers, MaxAnswersBytes)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(into); err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%w: the file is empty", ErrInvalidAnswers)
		}
		return fmt.Errorf("%w: %w", ErrInvalidAnswers, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: only one YAML document is allowed", ErrInvalidAnswers)
	}
	return nil
}

func EncodeAnswers(header string, v any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(header)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encode answers: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode answers: %w", err)
	}
	return buf.Bytes(), nil
}
