package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type Newer interface {
	New() any
}
type Question interface {
	Newer
	Marshal() ([]byte, error)
}

type AnswerChunk interface {
	New() any
	Unmarshal([]byte) error
}

func newObj[T Newer]() T {
	var t T
	return t.New().(T)
}

type Error struct {
	Type    string
	Message string
}

type Chat[Q Question, AC AnswerChunk] interface {
	Stream(ctx context.Context, q Q) (chan AC, error)
}

type Client[Q Question, AC AnswerChunk] struct {
	cli *http.Client
	url string
}

func New[Q Question, AC AnswerChunk](cli *http.Client, url string) *Client[Q, AC] {
	return &Client[Q, AC]{cli: cli, url: url}
}

func (c *Client[Q, AC]) Stream(ctx context.Context, q Q) (chan AC, error) {
	ch := make(chan AC)
	data, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := c.cli.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		errbuf := bytes.NewBuffer(nil)
		for scanner.Scan() {
			ansc := newObj[AC]()
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			prefix := "data:"
			// it would be an error event if not data: prefixed
			if !strings.HasPrefix(line, prefix) {
				errbuf.WriteString(line)
				continue
			}
			if line == "data: [DONE]" {
				return
			}
			text := line[len(prefix):]

			if err := json.Unmarshal([]byte(text), ansc); err != nil {
				return
			}

			select {
			case <-ctx.Done():
				return
			case ch <- ansc:
			}
		}

		if errbuf.Len() == 0 {
			return
		}
		// send the error message
		ansc := newObj[AC]()
		if err := json.Unmarshal(errbuf.Bytes(), ansc); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case ch <- ansc:
		}
	}()
	return ch, nil
}
