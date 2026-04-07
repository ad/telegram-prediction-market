package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestSendPollExtended(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "getMe") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"result": map[string]interface{}{"id": 1, "is_bot": true, "first_name": "Test"},
			})
			return
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		resp := telegramAPIResponse{
			OK: true,
			Result: json.RawMessage(`{
				"message_id": 42,
				"chat": {"id": 123},
				"poll": {"id": "poll_123", "question": "Test?", "options": []}
			}`),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b, err := tgbot.New("test-token", tgbot.WithServerURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	// Override base URL for test
	origURL := telegramAPIBaseURL
	telegramAPIBaseURL = server.URL
	defer func() { telegramAPIBaseURL = origURL }()

	isAnonymous := false
	allowsRevoting := false
	params := &ExtendedSendPollParams{
		ChatID:   123,
		Question: "Will it rain?",
		Options: []models.InputPollOption{
			{Text: "Yes"},
			{Text: "No"},
		},
		IsAnonymous:            &isAnonymous,
		ProtectContent:         true,
		AllowsRevoting:         &allowsRevoting,
		ShuffleOptions:         true,
		CloseDate:              1700000000,
		HideResultsUntilCloses: true,
	}

	msg, err := sendPollExtended(context.Background(), b, params)
	if err != nil {
		t.Fatalf("sendPollExtended failed: %v", err)
	}

	if msg.ID != 42 {
		t.Errorf("expected message ID 42, got %d", msg.ID)
	}

	// Verify the JSON payload contains all new fields
	if v, ok := receivedBody["allows_revoting"].(bool); !ok || v != false {
		t.Errorf("expected allows_revoting=false, got %v", receivedBody["allows_revoting"])
	}
	if v, ok := receivedBody["shuffle_options"].(bool); !ok || v != true {
		t.Errorf("expected shuffle_options=true, got %v", receivedBody["shuffle_options"])
	}
	if v, ok := receivedBody["hide_results_until_closes"].(bool); !ok || v != true {
		t.Errorf("expected hide_results_until_closes=true, got %v", receivedBody["hide_results_until_closes"])
	}
	if v, ok := receivedBody["close_date"].(float64); !ok || int64(v) != 1700000000 {
		t.Errorf("expected close_date=1700000000, got %v", receivedBody["close_date"])
	}
}

func TestSendPollExtendedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "getMe") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":     true,
				"result": map[string]interface{}{"id": 1, "is_bot": true, "first_name": "Test"},
			})
			return
		}

		resp := telegramAPIResponse{
			OK:          false,
			ErrorCode:   400,
			Description: "Bad Request: poll must have at least 2 option",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b, err := tgbot.New("test-token", tgbot.WithServerURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	origURL := telegramAPIBaseURL
	telegramAPIBaseURL = server.URL
	defer func() { telegramAPIBaseURL = origURL }()

	params := &ExtendedSendPollParams{
		ChatID:   123,
		Question: "Test?",
		Options:  []models.InputPollOption{{Text: "Only one"}},
	}

	_, err = sendPollExtended(context.Background(), b, params)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Bad Request") {
		t.Errorf("expected error to contain 'Bad Request', got: %v", err)
	}
}
