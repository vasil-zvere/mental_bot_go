package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type VKClient struct {
	token      string
	groupID    int64
	apiVersion string
	engine     *Engine
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

type vkLongPollCheck struct {
	Ts      string            `json:"ts"`
	Updates []vkLongPollEvent `json:"updates"`
	Failed  int               `json:"failed,omitempty"`
}

type vkLongPollEvent struct {
	Type   string         `json:"type"`
	Object vkEventObject  `json:"object"`
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

func NewVKClient(token string, groupID int64, apiVersion string, engine *Engine) *VKClient {
	if apiVersion == "" {
		apiVersion = "5.199"
	}
	return &VKClient{
		token:      token,
		groupID:    groupID,
		apiVersion: apiVersion,
		engine:     engine,
		httpClient: &http.Client{Timeout: 35 * time.Second},
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
		currentTs = parsed.Ts
		for _, upd := range parsed.Updates {
			if upd.Type != "message_new" {
				continue
			}
			input := upd.Object.Message.Text
			if input == "" && upd.Object.Message.Payload != "" {
				var p vkPayload
				if err := json.Unmarshal([]byte(upd.Object.Message.Payload), &p); err == nil && p.Cmd != "" {
					input = p.Cmd
				}
			}
			if strings.TrimSpace(input) == "" {
				continue
			}
			chatID := strconv.FormatInt(upd.Object.Message.PeerID, 10)
			responses := c.engine.HandleInput("vk", chatID, input)
			for _, out := range responses {
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
