package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type VKClient struct {
	token      string
	groupID    int64
	apiVersion string
	engine     *Engine
	history    *HistoryStore
	reports    *ReportService
	httpClient *http.Client
}

type vkAPIResponse[T any] struct {
	Response T        `json:"response"`
	Error    *vkError `json:"error,omitempty"`
}

type vkError struct {
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

type vkLongPollServer struct {
	Key    string `json:"key"`
	Server string `json:"server"`
	Ts     string `json:"ts"`
}

type VKString string

func (s *VKString) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*s = ""
		return nil
	}
	if data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		*s = VKString(str)
		return nil
	}
	*s = VKString(string(data))
	return nil
}

type vkLongPollCheck struct {
	TS      VKString          `json:"ts"`
	Updates []vkLongPollEvent `json:"updates"`
	Failed  int               `json:"failed,omitempty"`
}

type vkLongPollEvent struct {
	Type   string        `json:"type"`
	Object vkEventObject `json:"object"`
}

type vkEventObject struct {
	Message vkMessage `json:"message"`
}

type vkMessage struct {
	ID      int64  `json:"id"`
	Date    int64  `json:"date"`
	PeerID  int64  `json:"peer_id"`
	FromID  int64  `json:"from_id"`
	Text    string `json:"text"`
	Payload string `json:"payload,omitempty"`
}

type vkPayload struct {
	Cmd string `json:"cmd"`
}

func NewVKClient(token string, groupID int64, apiVersion string, engine *Engine, history *HistoryStore, reports *ReportService) *VKClient {
	if apiVersion == "" {
		apiVersion = "5.199"
	}
	return &VKClient{
		token:      token,
		groupID:    groupID,
		apiVersion: apiVersion,
		engine:     engine,
		history:    history,
		reports:    reports,
		httpClient: &http.Client{
			Timeout: 35 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 0 {
					req.Method = via[0].Method
					req.Body = via[0].Body
					req.GetBody = via[0].GetBody
				}
				return nil
			},
		},
	}
}

func (c *VKClient) Run(ctx context.Context) error {
	log.Println("VK bot started")
	for {
		server, err := c.getLongPollServer(ctx)
		if err != nil {
			log.Printf("vk getLongPollServer error: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		if err := c.listenLongPoll(ctx, server); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("vk long poll error: %v", err)
			time.Sleep(2 * time.Second)
		}
	}
}

func (c *VKClient) getLongPollServer(ctx context.Context) (*vkLongPollServer, error) {
	params := url.Values{}
	params.Set("group_id", strconv.FormatInt(c.groupID, 10))
	var resp vkAPIResponse[vkLongPollServer]
	if err := c.callMethod(ctx, "groups.getLongPollServer", params, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("vk error %d: %s", resp.Error.ErrorCode, resp.Error.ErrorMsg)
	}
	return &resp.Response, nil
}

func (c *VKClient) listenLongPoll(ctx context.Context, server *vkLongPollServer) error {
	currentTs := server.Ts
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pollURL := fmt.Sprintf("%s?act=a_check&key=%s&ts=%s&wait=25", server.Server, url.QueryEscape(server.Key), url.QueryEscape(currentTs))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
		if err != nil {
			return err
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode >= 300 {
			return fmt.Errorf("vk long poll HTTP %d: %s", resp.StatusCode, string(body))
		}

		var parsed vkLongPollCheck
		if err := json.Unmarshal(body, &parsed); err != nil {
			return err
		}
		if parsed.Failed != 0 {
			return fmt.Errorf("vk long poll failed=%d", parsed.Failed)
		}
		currentTs = string(parsed.TS)
		for _, upd := range parsed.Updates {
			if upd.Type != "message_new" {
				continue
			}

			input := strings.TrimSpace(upd.Object.Message.Text)
			if input == "" && upd.Object.Message.Payload != "" {
				var payload vkPayload
				if err := json.Unmarshal([]byte(upd.Object.Message.Payload), &payload); err == nil && payload.Cmd != "" {
					input = strings.TrimSpace(payload.Cmd)
				}
			}
			if input == "" {
				continue
			}

			chatID := strconv.FormatInt(upd.Object.Message.PeerID, 10)
			sessionKey := "vk:" + chatID

			before := c.engine.sessions.Get(sessionKey)
			themeKey, state, questionIndex, score, resultLevel := snapshotFromSession(before, c.engine.content)

			incomingTime := time.Now()
			if upd.Object.Message.Date > 0 {
				incomingTime = time.Unix(upd.Object.Message.Date, 0)
			}

			if c.history != nil {
				_ = c.history.SaveMessage(HistoryMessage{
					Platform:      "vk",
					ChatID:        chatID,
					Direction:     "incoming",
					MessageText:   input,
					SentAt:        incomingTime,
					ThemeKey:      themeKey,
					State:         state,
					QuestionIndex: questionIndex,
					Score:         score,
					ResultLevel:   resultLevel,
				})
			}

			if isMyReportCommand(input) {
				if c.reports == nil {
					if err := c.sendMessage(ctx, upd.Object.Message.PeerID, OutgoingMessage{Text: "Формирование отчетов сейчас недоступно.", Buttons: mainMenuButtons()}); err != nil {
						log.Printf("vk messages.send error: %v", err)
					}
					continue
				}

				reportPath, err := c.reports.GenerateUserReport("vk", chatID)
				if err != nil {
					log.Printf("vk report error: %v", err)
					if sendErr := c.sendMessage(ctx, upd.Object.Message.PeerID, OutgoingMessage{
						Text:    "Не удалось сформировать отчет. Проверь, есть ли история диалога и корректно ли собирается PDF.",
						Buttons: mainMenuButtons(),
					}); sendErr != nil {
						log.Printf("vk error sending report failure message: %v", sendErr)
					}
					continue
				}

				err = c.sendDocument(ctx, upd.Object.Message.PeerID, reportPath, "Ваш персональный PDF-отчет готов.")
				if err != nil {
					log.Printf("vk sendDocument error: %v", err)
				}
				_ = os.Remove(reportPath)

				if err != nil {
					if sendErr := c.sendMessage(ctx, upd.Object.Message.PeerID, OutgoingMessage{
						Text:    "Не удалось отправить PDF-отчет.",
						Buttons: mainMenuButtons(),
					}); sendErr != nil {
						log.Printf("vk error sending PDF failure message: %v", sendErr)
					}
				}
				continue
			}

			if isDeleteHistoryCommand(input) {
				if c.history == nil {
					if err := c.sendMessage(ctx, upd.Object.Message.PeerID, OutgoingMessage{Text: "Хранилище истории не подключено.", Buttons: mainMenuButtons()}); err != nil {
						log.Printf("vk messages.send error: %v", err)
					}
					continue
				}

				if err := c.history.DeleteUserHistory("vk", chatID); err != nil {
					if sendErr := c.sendMessage(ctx, upd.Object.Message.PeerID, OutgoingMessage{Text: "Не удалось удалить историю.", Buttons: mainMenuButtons()}); sendErr != nil {
						log.Printf("vk error deleting history: %v", sendErr)
					}
				} else {
					if sendErr := c.sendMessage(ctx, upd.Object.Message.PeerID, OutgoingMessage{Text: "Ваша история диалога удалена.", Buttons: mainMenuButtons()}); sendErr != nil {
						log.Printf("vk error sending delete history confirmation: %v", sendErr)
					}
				}
				continue
			}

			responses := c.engine.HandleInput("vk", chatID, input)
			after := c.engine.sessions.Get(sessionKey)
			themeKey, state, questionIndex, score, resultLevel = snapshotFromSession(after, c.engine.content)

			for _, out := range responses {
				if c.history != nil {
					_ = c.history.SaveMessage(HistoryMessage{
						Platform:      "vk",
						ChatID:        chatID,
						Direction:     "outgoing",
						MessageText:   out.Text,
						SentAt:        time.Now(),
						ThemeKey:      themeKey,
						State:         state,
						QuestionIndex: questionIndex,
						Score:         score,
						ResultLevel:   resultLevel,
					})
				}

				if err := c.sendMessage(ctx, upd.Object.Message.PeerID, out); err != nil {
					log.Printf("vk messages.send error: %v", err)
				}
			}
		}
	}
}

func (c *VKClient) sendMessage(ctx context.Context, peerID int64, msg OutgoingMessage) error {
	params := url.Values{}
	params.Set("peer_id", strconv.FormatInt(peerID, 10))
	params.Set("random_id", strconv.Itoa(rand.New(rand.NewSource(time.Now().UnixNano())).Int()))
	params.Set("message", msg.Text)
	if len(msg.Buttons) > 0 {
		kb, err := buildVKKeyboard(msg.Buttons)
		if err != nil {
			return err
		}
		params.Set("keyboard", kb)
	}

	var resp vkAPIResponse[int]
	if err := c.callMethod(ctx, "messages.send", params, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("vk error %d: %s", resp.Error.ErrorCode, resp.Error.ErrorMsg)
	}
	return nil
}

func (c *VKClient) callMethod(ctx context.Context, method string, params url.Values, target any) error {
	params = cloneValues(params)
	params.Set("access_token", c.token)
	params.Set("v", c.apiVersion)
	endpoint := fmt.Sprintf("https://api.vk.com/method/%s", method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("vk HTTP %d: %s", resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, target)
}

func buildVKKeyboard(buttons [][]string) (string, error) {
	type action struct {
		Type    string `json:"type"`
		Label   string `json:"label"`
		Payload string `json:"payload,omitempty"`
	}
	type button struct {
		Action action `json:"action"`
		Color  string `json:"color"`
	}
	type keyboard struct {
		OneTime bool       `json:"one_time"`
		Buttons [][]button `json:"buttons"`
	}

	rows := make([][]button, 0, len(buttons))
	for _, row := range buttons {
		btnRow := make([]button, 0, len(row))
		for _, label := range row {
			payloadBytes, _ := json.Marshal(vkPayload{Cmd: label})
			btnRow = append(btnRow, button{
				Action: action{Type: "text", Label: label, Payload: string(payloadBytes)},
				Color:  vkButtonColor(label),
			})
		}
		rows = append(rows, btnRow)
	}
	kb := keyboard{OneTime: false, Buttons: rows}
	b, err := json.Marshal(kb)
	return string(b), err
}

func vkButtonColor(label string) string {
	switch normalize(label) {
	case normalize("Начать тест"), normalize("Да, начать"):
		return "positive"
	case normalize("Выйти"):
		return "negative"
	default:
		return "secondary"
	}
}

func cloneValues(v url.Values) url.Values {
	copyV := url.Values{}
	for k, vals := range v {
		newVals := make([]string, len(vals))
		copy(newVals, vals)
		copyV[k] = newVals
	}
	return copyV
}

type vkUploadServerResponse struct {
	UploadURL string `json:"upload_url"`
}

type vkUploadedDocument struct {
	File string `json:"file"`
}

type vkDoc struct {
	ID      int64 `json:"id"`
	OwnerID int64 `json:"owner_id"`
}

type vkDocsSaveResponse struct {
	Type string `json:"type"`
	Doc  vkDoc  `json:"doc"`
}

func (c *VKClient) sendDocument(ctx context.Context, peerID int64, filePath, caption string) error {
	uploadURL, err := c.getMessagesUploadServer(ctx, peerID)
	if err != nil {
		log.Printf("vk getMessagesUploadServer error: %v", err)
		return err
	}
	log.Printf("vk uploadURL ok")

	fileToken, err := c.uploadDocumentFile(ctx, uploadURL, filePath)
	if err != nil {
		log.Printf("vk uploadDocumentFile error: %v, повторная попытка с новым upload_url", err)
		// upload_url у VK может протухнуть между запросом и фактической загрузкой —
		// получаем свежий URL и пробуем ещё раз.
		uploadURL, err = c.getMessagesUploadServer(ctx, peerID)
		if err != nil {
			log.Printf("vk getMessagesUploadServer retry error: %v", err)
			return err
		}
		fileToken, err = c.uploadDocumentFile(ctx, uploadURL, filePath)
		if err != nil {
			log.Printf("vk uploadDocumentFile retry error: %v", err)
			return err
		}
	}
	log.Printf("vk uploadDocumentFile ok")

	attachment, err := c.saveUploadedDocument(ctx, fileToken, filepath.Base(filePath))
	if err != nil {
		log.Printf("vk saveUploadedDocument error: %v", err)
		return err
	}
	log.Printf("vk saveUploadedDocument ok: %s", attachment)

	err = c.sendMessageWithAttachment(ctx, peerID, caption, attachment)
	if err != nil {
		log.Printf("vk sendMessageWithAttachment error: %v", err)
		return err
	}
	log.Printf("vk sendMessageWithAttachment ok")

	return nil
}

func (c *VKClient) getMessagesUploadServer(ctx context.Context, peerID int64) (string, error) {
	params := url.Values{}
	params.Set("peer_id", strconv.FormatInt(peerID, 10))
	params.Set("type", "doc")

	var resp vkAPIResponse[vkUploadServerResponse]
	if err := c.callMethod(ctx, "docs.getMessagesUploadServer", params, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", fmt.Errorf("vk error %d: %s", resp.Error.ErrorCode, resp.Error.ErrorMsg)
	}
	if resp.Response.UploadURL == "" {
		return "", fmt.Errorf("VK не вернул upload_url")
	}
	log.Printf("vk upload_url: %s", resp.Response.UploadURL)

	return resp.Response.UploadURL, nil
}

func (c *VKClient) uploadDocumentFile(ctx context.Context, uploadURL, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("VK upload HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var uploaded vkUploadedDocument
	if err := json.Unmarshal(respBody, &uploaded); err != nil {
		return "", err
	}
	if uploaded.File == "" {
		return "", fmt.Errorf("VK upload не вернул file token")
	}

	return uploaded.File, nil
}

func (c *VKClient) saveUploadedDocument(ctx context.Context, fileToken, title string) (string, error) {
	params := url.Values{}
	params.Set("file", fileToken)
	params.Set("title", title)

	var resp vkAPIResponse[vkDocsSaveResponse]
	if err := c.callMethod(ctx, "docs.save", params, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", fmt.Errorf("vk error %d: %s", resp.Error.ErrorCode, resp.Error.ErrorMsg)
	}

	if resp.Response.Doc.ID == 0 {
		return "", fmt.Errorf("VK не вернул сохранённый документ")
	}

	doc := resp.Response.Doc
	return fmt.Sprintf("doc%d_%d", doc.OwnerID, doc.ID), nil
}

func (c *VKClient) sendMessageWithAttachment(ctx context.Context, peerID int64, text, attachment string) error {
	params := url.Values{}
	params.Set("peer_id", strconv.FormatInt(peerID, 10))
	params.Set("random_id", strconv.Itoa(rand.New(rand.NewSource(time.Now().UnixNano())).Int()))
	params.Set("message", text)
	params.Set("attachment", attachment)

	var resp vkAPIResponse[int]
	if err := c.callMethod(ctx, "messages.send", params, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("vk error %d: %s", resp.Error.ErrorCode, resp.Error.ErrorMsg)
	}
	return nil
}
