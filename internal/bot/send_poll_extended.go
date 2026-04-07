package bot

// TODO: Remove this file when go-telegram/bot library adds AllowsRevoting, ShuffleOptions, HideResultsUntilCloses fields.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// ExtendedSendPollParams contains all sendPoll parameters including fields
// not yet supported by go-telegram/bot library.
type ExtendedSendPollParams struct {
	ChatID                 any                      `json:"chat_id"`
	MessageThreadID        int                      `json:"message_thread_id,omitempty"`
	Question               string                   `json:"question"`
	Options                []models.InputPollOption `json:"options"`
	IsAnonymous            *bool                    `json:"is_anonymous,omitempty"`
	AllowsMultipleAnswers  bool                     `json:"allows_multiple_answers,omitempty"`
	DisableNotification    bool                     `json:"disable_notification,omitempty"`
	ProtectContent         bool                     `json:"protect_content,omitempty"`
	CloseDate              int64                    `json:"close_date,omitempty"`
	AllowsRevoting         *bool                    `json:"allows_revoting,omitempty"`
	ShuffleOptions         bool                     `json:"shuffle_options,omitempty"`
	HideResultsUntilCloses bool                     `json:"hide_results_until_closes,omitempty"`
}

type telegramAPIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
}

// telegramAPIBaseURL is the base URL for Telegram Bot API.
// Overridden in tests.
var telegramAPIBaseURL = "https://api.telegram.org"

func sendPollExtended(ctx context.Context, b *tgbot.Bot, params *ExtendedSendPollParams) (*models.Message, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal poll params: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendPoll", telegramAPIBaseURL, b.Token())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send poll request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp telegramAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if !apiResp.OK {
		return nil, fmt.Errorf("telegram API error %d: %s", apiResp.ErrorCode, apiResp.Description)
	}

	var msg models.Message
	if err := json.Unmarshal(apiResp.Result, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal message: %w", err)
	}

	return &msg, nil
}
