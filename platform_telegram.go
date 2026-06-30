package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type TelegramClient struct {
	token      string
	engine     *Engine
	history    *HistoryStore
	reports    *ReportService
	httpClient *http.Client
	offset     int64
}

type tgUpdateResponse struct {
	OK     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

type tgUpdate struct {
	UpdateID int64      `json:"update_id"`
	Message  *tgMessage `json:"message,omitempty"`
}

type tgMessage struct {
	MessageID int64   `json:"message_id"`
	Text      string  `json:"text"`
	Chat      tgChat  `json:"chat"`
	From      *tgUser `json:"from,omitempty"`
}

type tgChat struct {
	ID int64 `json:"id"`
}

type tgUser struct {
	ID int64 `json:"id"`
}

type tgSendMessageRequest struct {
	ChatID      int64           `json:"chat_id"`
	Text        string          `json:"text"`
	ReplyMarkup json.RawMessage `json:"reply_markup,omitempty"`
}

func NewTelegramClient(token string, engine *Engine, history *HistoryStore, reports *ReportService) *TelegramClient {
	return &TelegramClient{
		token:      token,
		engine:     engine,
		history:    history,
		reports:    reports,
		httpClient: &http.Client{Timeout: 35 * time.Second},
	}
}

func (c *TelegramClient) Run(ctx context.Context) error {
	log.Println("Telegram bot started")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, err := c.getUpdates(ctx)
		if err != nil {
			log.Printf("telegram getUpdates error: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		for _, upd := range updates {
			if upd.UpdateID >= c.offset {
				c.offset = upd.UpdateID + 1
			}
			if upd.Message == nil || upd.Message.Text == "" {
				continue
			}

			chatID := strconv.FormatInt(upd.Message.Chat.ID, 10)
			sessionKey := "telegram:" + chatID
			before := c.engine.sessions.Get(sessionKey)
			themeKey, state, questionIndex, score, resultLevel := snapshotFromSession(before, c.engine.content)

			_ = c.history.SaveMessage(HistoryMessage{
				Platform:      "telegram",
				ChatID:        chatID,
				Direction:     "incoming",
				MessageText:   upd.Message.Text,
				SentAt:        time.Now(),
				ThemeKey:      themeKey,
				State:         state,
				QuestionIndex: questionIndex,
				Score:         score,
				ResultLevel:   resultLevel,
			})

			if isMyReportCommand(upd.Message.Text) {
				reportPath, err := c.reports.GenerateUserReport("telegram", chatID)
				if err != nil {
					_ = c.sendMessage(ctx, upd.Message.Chat.ID, OutgoingMessage{
						Text:    "Не удалось сформировать отчет. Проверь, есть ли история диалога и шрифт DejaVuSans.ttf.",
						Buttons: mainMenuButtons(),
					})
					continue
				}

				if err := c.sendDocument(ctx, upd.Message.Chat.ID, reportPath, "Ваш персональный PDF-отчет готов."); err != nil {
					_ = c.sendMessage(ctx, upd.Message.Chat.ID, OutgoingMessage{
						Text:    "Не удалось отправить PDF-отчет.",
						Buttons: mainMenuButtons(),
					})
				}
				_ = os.Remove(reportPath)
				continue
			}

			if isDeleteHistoryCommand(upd.Message.Text) {
				if err := c.history.DeleteUserHistory("telegram", chatID); err != nil {
					_ = c.sendMessage(ctx, upd.Message.Chat.ID, OutgoingMessage{
						Text:    "Не удалось удалить историю.",
						Buttons: mainMenuButtons(),
					})
				} else {
					_ = c.sendMessage(ctx, upd.Message.Chat.ID, OutgoingMessage{
						Text:    "Ваша история диалога удалена.",
						Buttons: mainMenuButtons(),
					})
				}
				continue
			}

			responses := c.engine.HandleInput("telegram", chatID, upd.Message.Text)
			after := c.engine.sessions.Get(sessionKey)
			themeKey, state, questionIndex, score, resultLevel = snapshotFromSession(after, c.engine.content)

			for _, resp := range responses {
				_ = c.history.SaveMessage(HistoryMessage{
					Platform:      "telegram",
					ChatID:        chatID,
					Direction:     "outgoing",
					MessageText:   resp.Text,
					SentAt:        time.Now(),
					ThemeKey:      themeKey,
					State:         state,
					QuestionIndex: questionIndex,
					Score:         score,
					ResultLevel:   resultLevel,
				})

				if err := c.sendMessage(ctx, upd.Message.Chat.ID, resp); err != nil {
					log.Printf("telegram sendMessage error: %v", err)
				}
			}
		}
	}
}

func (c *TelegramClient) getUpdates(ctx context.Context) ([]tgUpdate, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=30&offset=%d", c.token, c.offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("telegram HTTP %d: %s", resp.StatusCode, string(body))
	}

	var parsed tgUpdateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if !parsed.OK {
		return nil, fmt.Errorf("telegram API returned ok=false")
	}
	return parsed.Result, nil
}

func (c *TelegramClient) sendMessage(ctx context.Context, chatID int64, msg OutgoingMessage) error {
	payload := tgSendMessageRequest{
		ChatID: chatID,
		Text:   msg.Text,
	}
	if len(msg.Buttons) > 0 {
		markup, err := buildTelegramKeyboard(msg.Buttons)
		if err != nil {
			return err
		}
		payload.ReplyMarkup = markup
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func buildTelegramKeyboard(buttons [][]string) (json.RawMessage, error) {
	type tgButton struct {
		Text string `json:"text"`
	}
	type tgKeyboard struct {
		Keyboard        [][]tgButton `json:"keyboard"`
		ResizeKeyboard  bool         `json:"resize_keyboard"`
		OneTimeKeyboard bool         `json:"one_time_keyboard"`
	}
	rows := make([][]tgButton, 0, len(buttons))
	for _, row := range buttons {
		btnRow := make([]tgButton, 0, len(row))
		for _, title := range row {
			btnRow = append(btnRow, tgButton{Text: title})
		}
		rows = append(rows, btnRow)
	}
	kb := tgKeyboard{Keyboard: rows, ResizeKeyboard: true, OneTimeKeyboard: false}
	b, err := json.Marshal(kb)
	return json.RawMessage(b), err
}

func (c *TelegramClient) sendDocument(ctx context.Context, chatID int64, filePath, caption string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return err
	}
	if caption != "" {
		if err := writer.WriteField("caption", caption); err != nil {
			return err
		}
	}

	part, err := writer.CreateFormFile("document", filepath.Base(filePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram sendDocument HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
