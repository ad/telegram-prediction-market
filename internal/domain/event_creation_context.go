package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrInvalidContextData is returned when context data is invalid
	ErrInvalidContextData = errors.New("invalid context data")
	// ErrMissingRequiredField is returned when a required field is missing
	ErrMissingRequiredField = errors.New("missing required field")
)

// EventCreationContext holds data during event creation flow
type EventCreationContext struct {
	GroupID               int64     `json:"group_id"`
	Question              string    `json:"question"`
	EventType             EventType `json:"event_type"`
	Options               []string  `json:"options"`
	Deadline              time.Time `json:"deadline"`
	LastBotMessageID      int       `json:"last_bot_message_id"`
	LastUserMessageID     int       `json:"last_user_message_id"`
	LastErrorMessageID    int       `json:"last_error_message_id"`
	ConfirmationMessageID int       `json:"confirmation_message_id"`
	ChatID                int64     `json:"chat_id"`
}

// ToMap converts EventCreationContext to a map for JSON serialization
func (c *EventCreationContext) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"group_id":                c.GroupID,
		"question":                c.Question,
		"event_type":              string(c.EventType),
		"options":                 c.Options,
		"deadline":                c.Deadline.Format(time.RFC3339),
		"last_bot_message_id":     c.LastBotMessageID,
		"last_user_message_id":    c.LastUserMessageID,
		"last_error_message_id":   c.LastErrorMessageID,
		"confirmation_message_id": c.ConfirmationMessageID,
		"chat_id":                 c.ChatID,
	}
}

// FromMap populates EventCreationContext from a map after JSON deserialization
func (c *EventCreationContext) FromMap(data map[string]interface{}) error {
	if data == nil {
		return ErrInvalidContextData
	}

	// Parse group_id (handle both int64 and float64 from JSON)
	if groupID, ok := data["group_id"].(float64); ok {
		c.GroupID = int64(groupID)
	} else if groupID, ok := data["group_id"].(int64); ok {
		c.GroupID = groupID
	} else if groupID, ok := data["group_id"].(int); ok {
		c.GroupID = int64(groupID)
	}

	// Parse question
	if question, ok := data["question"].(string); ok {
		c.Question = question
	}

	// Parse event_type
	if eventType, ok := data["event_type"].(string); ok {
		c.EventType = EventType(eventType)
	}

	// Parse options
	if options, ok := data["options"].([]interface{}); ok {
		c.Options = make([]string, len(options))
		for i, opt := range options {
			if optStr, ok := opt.(string); ok {
				c.Options[i] = optStr
			}
		}
	}

	// Parse deadline
	if deadlineStr, ok := data["deadline"].(string); ok {
		deadline, err := time.Parse(time.RFC3339, deadlineStr)
		if err != nil {
			return fmt.Errorf("failed to parse deadline: %w", err)
		}
		c.Deadline = deadline
	}

	// Parse last_bot_message_id
	if lastBotMsgID, ok := data["last_bot_message_id"].(float64); ok {
		c.LastBotMessageID = int(lastBotMsgID)
	}

	// Parse last_user_message_id
	if lastUserMsgID, ok := data["last_user_message_id"].(float64); ok {
		c.LastUserMessageID = int(lastUserMsgID)
	}

	// Parse last_error_message_id
	if lastErrorMsgID, ok := data["last_error_message_id"].(float64); ok {
		c.LastErrorMessageID = int(lastErrorMsgID)
	}

	// Parse confirmation_message_id
	if confirmMsgID, ok := data["confirmation_message_id"].(float64); ok {
		c.ConfirmationMessageID = int(confirmMsgID)
	}

	// Parse chat_id (handle both int64 and float64 from JSON)
	if chatID, ok := data["chat_id"].(float64); ok {
		c.ChatID = int64(chatID)
	} else if chatID, ok := data["chat_id"].(int64); ok {
		c.ChatID = chatID
	} else if chatID, ok := data["chat_id"].(int); ok {
		c.ChatID = int64(chatID)
	}

	return nil
}

// Validate validates the EventCreationContext for required fields
func (c *EventCreationContext) Validate() error {
	if c.ChatID == 0 {
		return fmt.Errorf("%w: chat_id", ErrMissingRequiredField)
	}

	// Other fields may be optional depending on the state
	return nil
}
