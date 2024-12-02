package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/shafreeck/guru/chat"
	"github.com/shafreeck/guru/tui"
)

type ChatOptions struct {
	ChatGPTOptions    `yaml:"chatgpt"`
	System            string `yaml:"system"`
	Oneshot           bool   `yaml:"oneshot"`
	Executor          string `yaml:"executor"`
	Feedback          bool   `yaml:"feedback"`
	Verbose           bool   `yaml:"verbose"`
	Renderer          string `yaml:"renderer"`
	NonInteractive    bool   `yaml:"non-interactive"`
	DisableAutoShrink bool   `yaml:"disable-auto-shrink"`
	Text              string `yaml:"-"`
}

type ChatCommand struct {
	c         chat.Chat[*Question, *AnswerChunk]
	ap        *AwesomePrompts
	sess      *Session
	isVerbose bool
}

func NewChatCommand(sess *Session, ap *AwesomePrompts, httpCli *http.Client, opts *ChatCommandOptions) *ChatCommand {
	c := NewChatGPTClient(httpCli, &opts.ChatGPTOptions)
	return &ChatCommand{c: c, sess: sess, ap: ap, isVerbose: opts.Verbose}
}

func (c *ChatCommand) Talk(opts *ChatOptions) (string, error) {
	if opts.Oneshot {
		c.sess.ClearMessage()
	}

	if opts.Text != "" {
		c.sess.Append(&Message{Content: opts.Text})
	}

	// return if there is nothing to ask
	if len(c.sess.Messages()) == 0 {
		return "", nil
	}

	return c.stream(context.Background(), opts)
}
func (c *ChatCommand) verbose(text string) {
	if !c.isVerbose {
		return
	}
	c.sess.out.Println(text)
}

func (c *ChatCommand) stream(ctx context.Context, opts *ChatOptions) (string, error) {
retry:
	q := &Question{
		// ChatGPTOptions: opts.ChatGPTOptions,
		// Messages:       c.sess.Messages(),
		ConversationId: c.sess.sid,
		Prompt: c.sess.Messages()[len(c.sess.Messages())-1].Content,
	}

	// issue a request to the api
	s, err := tui.Display[tui.Model[chan *AnswerChunk], chan *AnswerChunk](ctx,
		tui.NewSpinnerModel("", func() (chan *AnswerChunk, error) {
			return c.c.Stream(ctx, q)
		}))
	if err != nil {
		return "", err
	}
	// ctrl+c interrupted
	if s == nil {
		return "", nil
	}

	// handle the stream and print the delta text, the whole
	// content is returned when finished
	content, err := tui.Display[tui.Model[string], string](ctx, tui.NewStreamModel(s, opts.Renderer, func(event *AnswerChunk) (string, error) {
		// if event.Error.Message != "" {
		// 	return "", fmt.Errorf("%s: %s", event.Error.Code, event.Error.Message)
		// }
		var once *sync.Once = new(sync.Once)
		once.Do(func() {
			if strings.HasPrefix(c.sess.sid, "temporary-chat") {
				c.sess.sid = event.ConversationId

			}
		})
		return event.Content, nil
	}))

	// The token limit exceeded. auto shrink and retry if enabled
	if c.IsTokenExceeded(err) {
		if opts.DisableAutoShrink {
			return "", fmt.Errorf("%w\n\nUse `:messages shrink <expr>` to reduce the tokens", err)
		}

		n := c.sess.mm.autoShrink()

		// Nothing to shrink, return.
		// This is the case that the last message is large enough
		// to exceed the token limit.
		if n == 0 {
			return "", err
		}

		word := "message"
		if n > 1 {
			word = "messages"
		}
		c.sess.out.Printf("%d %s shrinked because of tokens limitation\n", n, word)
		goto retry
	}
	if err != nil {
		return "", err
	}

	// Print to output if the tui is not renderable
	// in case the the stdout is not terminal
	if !tui.IsRenderable() {
		c.sess.out.Print(content)
	}
	// append the response
	c.sess.Append(&Message{Content: content})

	return content, nil
}

func (c *ChatCommand) IsTokenExceeded(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "context_length_exceeded") {
		return true
	}
	return false
}
