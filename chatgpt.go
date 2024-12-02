package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/shafreeck/guru/chat"
)

type Message struct {
	Content string `json:"content"`
}

type Question struct {
	ChatGPTOptions
	// Messages []*Message `json:"messages"`
	Prompt string `json:"prompt"`
}

func (q *Question) New() any {
	return &Question{}
}
func (q *Question) Marshal() ([]byte, error) {
	return json.Marshal(q)
}

type AnswerChoice struct {
	FinishReason string `json:"finish_reason"`
	Index        int    `json:"index"`
}

type AnswerError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}
type AnswerChunk struct {
	// ID      string `json:"id"`
	// Object  string `json:"object"`
	// Created int64  `json:"created"`
	// Choices []struct {
	// 	Delta struct {
	// 		Content string `json:"content"`
	// 	} `json:"delta"`
	// 	FinishReason string `json:"finish_reason"`
	// 	Index        int    `json:"index"`
	// } `json:"choices"`
	IsEnd          bool        `json:"isEnd"`
	Content        string      `json:"content"`
	ConversationId string      `json:"conversation_id"`
	MessageId      string      `json:"message_id"`
	Error          AnswerError `json:"error"`
}

func (ac *AnswerChunk) New() any {
	return &AnswerChunk{}
}
func (ac *AnswerChunk) Unmarshal(data []byte) error {
	return json.Unmarshal(data, ac)
}
func (ac *AnswerChunk) SetError(err error) {
	ac.Error.Type = "guru_inner_error"
	ac.Error.Message = err.Error()
}

type ChatGPTOptions struct {
	// MaxTokens int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty" cortana:"--chatgpt.max_tokens, -, 0, The maximum number of tokens to generate in the chat completion."`
	// N         int `yaml:"n,omitempty" json:"n" cortana:"--chatgpt.n, -, 1, How many chat completion choices to generate for each input message."`
	// PresencePenalty  float32 `yaml:"presence_penalty,omitempty" json:"presence_penalty,omitempty" cortana:"--chatgpt.presence_penalty, -, 0, Number between -2.0 and 2.0. Positive values penalize new tokens based on whether they appear in the text so far, increasing the model's likelihood to talk about new topics."`
	// FrequencyPenalty float32 `yaml:"frequency_penalty,omitempty" json:"frequency_penalty,omitempty" cortana:"--chatgpt.frequency_penalty, -, 0, Number between -2.0 and 2.0. Positive values penalize new tokens based on their existing frequency in the text so far, decreasing the model's likelihood to repeat the same line verbatim."`
	// User             string  `yaml:"user,omitempty" json:"user,omitempty" cortana:"--chatgpt.user, -, , A unique identifier representing your end-user, which can help OpenAI to monitor and detect abuse."`
}

const ChatGPTAPIURL = "http://lbrowser-admin.loongnix.cn/api/admin/lbrowser/open/chat"

type ChatGPTClient struct {
	opts *ChatGPTOptions
	cli  *chat.Client[*Question, *AnswerChunk]
}

func NewChatGPTClient(cli *http.Client, opts *ChatGPTOptions) *ChatGPTClient {
	chatCli := chat.New[*Question, *AnswerChunk](cli, ChatGPTAPIURL)
	return &ChatGPTClient{opts: opts, cli: chatCli}
}

func (c *ChatGPTClient) Stream(ctx context.Context, q *Question) (chan *AnswerChunk, error) {
	return c.cli.Stream(ctx, q)
}
