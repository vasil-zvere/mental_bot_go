package main

import (
	"context"
	"strconv"
)

// ── Telegram Sender ───────────────────────────────────────────────────────────

type TelegramSender struct {
	client *TelegramClient
}

func NewTelegramSender(c *TelegramClient) *TelegramSender {
	return &TelegramSender{client: c}
}

func (s *TelegramSender) SendNotification(ctx context.Context, platform, chatID string, msg OutgoingMessage) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return err
	}
	return s.client.sendMessage(ctx, id, msg)
}

// ── VK Sender ─────────────────────────────────────────────────────────────────

type VKSender struct {
	client *VKClient
}

func NewVKSender(c *VKClient) *VKSender {
	return &VKSender{client: c}
}

func (s *VKSender) SendNotification(ctx context.Context, platform, chatID string, msg OutgoingMessage) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return err
	}
	return s.client.sendMessage(ctx, id, msg)
}
