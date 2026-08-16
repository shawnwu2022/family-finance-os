package llm

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	errInvalidBaseURL       = errors.New("invalid LLM base URL")
	errInvalidStreamHandler = errors.New("stream handler is required")
)

func errorsInvalidBaseURL() error {
	return errInvalidBaseURL
}

func errorsInvalidStreamHandler() error {
	return errInvalidStreamHandler
}

type responsesStreamWire struct {
	Type  string              `json:"type"`
	Delta string              `json:"delta"`
	Item  responsesOutputItem `json:"item"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
	Response struct {
		ID    string `json:"id"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"response"`
}

func parseResponsesSSE(reader io.Reader, handler StreamHandler) error {
	if reader == nil {
		return fmt.Errorf("%w: nil stream", ErrProviderResponse)
	}
	if handler == nil {
		return errorsInvalidStreamHandler()
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)

	var eventName string
	dataLines := make([]string, 0, 1)
	dispatch := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		name := eventName
		eventName = ""
		if data == "[DONE]" {
			return nil
		}

		var wire responsesStreamWire
		if err := json.Unmarshal([]byte(data), &wire); err != nil {
			return fmt.Errorf("%w: decode SSE event %q: %v", ErrProviderResponse, name, err)
		}
		if wire.Type == "" {
			wire.Type = name
		}

		switch wire.Type {
		case "response.output_text.delta":
			return handler(StreamEvent{Type: StreamEventTextDelta, TextDelta: wire.Delta})
		case "response.output_item.done":
			if wire.Item.Type != "function_call" {
				return nil
			}
			call, err := toolCallFromWire(wire.Item.CallID, wire.Item.Name, wire.Item.Arguments)
			if err != nil {
				return err
			}
			return handler(StreamEvent{Type: StreamEventToolCall, ToolCall: &call})
		case "response.completed":
			return handler(StreamEvent{Type: StreamEventCompleted, ResponseID: wire.Response.ID})
		case "response.failed", "error", "response.error":
			message := "LLM response stream failed"
			if wire.Response.Error != nil && strings.TrimSpace(wire.Response.Error.Message) != "" {
				message = wire.Response.Error.Message
			} else if wire.Error != nil && strings.TrimSpace(wire.Error.Message) != "" {
				message = wire.Error.Message
			}
			return fmt.Errorf("%w: %s", ErrProviderResponse, message)
		default:
			// Responses emits lifecycle events that are not needed by the V1 caller.
			// Ignore only well-formed unknown events; malformed JSON already fails above.
			return nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Responses SSE read: %w", err)
	}
	if len(dataLines) > 0 {
		if err := dispatch(); err != nil {
			return err
		}
	}
	return nil
}
