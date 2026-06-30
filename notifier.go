package main

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// DefaultNotifyDays — значение по умолчанию, если конфигурация не указывает иное.
const DefaultNotifyDays = 3

// Sender — интерфейс отправки сообщения пользователю.
// Реализуется платформами (Telegram, VK) через адаптеры.
type Sender interface {
	SendNotification(ctx context.Context, platform, chatID string, msg OutgoingMessage) error
}

// Notifier — планировщик напоминаний.
type Notifier struct {
	db           *sql.DB
	senders      map[string]Sender // platform -> Sender
	InactiveDays int               // через сколько дней неактивности отправлять напоминание
}

func NewNotifier(db *sql.DB) *Notifier {
	return &Notifier{
		db:           db,
		senders:      make(map[string]Sender),
		InactiveDays: DefaultNotifyDays,
	}
}

// RegisterSender регистрирует отправителя для конкретной платформы.
func (n *Notifier) RegisterSender(platform string, s Sender) {
	n.senders[platform] = s
}

// Run запускает планировщик в отдельной горутине.
// Каждые 12 часов проверяет пользователей и рассылает напоминания.
func (n *Notifier) Run(ctx context.Context) {
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	// Первая проверка — сразу при старте (через минуту, чтобы бот успел подняться)
	select {
	case <-ctx.Done():
		return
	case <-time.After(1 * time.Minute):
		n.check(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.check(ctx)
		}
	}
}

// check находит всех пользователей, у которых давно не было активности,
// и отправляет им напоминание.
func (n *Notifier) check(ctx context.Context) {
	users, err := n.inactiveUsers()
	if err != nil {
		log.Printf("notifier: ошибка получения пользователей: %v", err)
		return
	}

	for _, u := range users {
		sender, ok := n.senders[u.platform]
		if !ok {
			continue
		}

		msg := OutgoingMessage{
			Text: "Привет! Ты не заходил(а) уже несколько дней.\n\n" +
				"Напоминаю, что дневник ABC и раздел «Самочувствие» помогают " +
				"замечать связь между событиями и внутренним состоянием. " +
				"Даже короткая запись имеет значение.\n\n" +
				"Как ты сейчас?",
			Buttons: [][]string{
				{"Самочувствие", "Дневник ABC"},
				{"В меню"},
			},
		}

		if err := sender.SendNotification(ctx, u.platform, u.chatID, msg); err != nil {
			log.Printf("notifier: не удалось отправить напоминание %s:%s: %v",
				u.platform, u.chatID, err)
			continue
		}

		// Отмечаем, что напоминание уже отправлено — чтобы не спамить
		if err := n.markNotified(u.platform, u.chatID); err != nil {
			log.Printf("notifier: ошибка отметки %s:%s: %v", u.platform, u.chatID, err)
		}
	}
}

type userKey struct {
	platform string
	chatID   string
}

// inactiveUsers возвращает пользователей, которые:
// - писали боту хотя бы раз,
// - не проявляли активности N или более дней,
// - ещё не получали напоминание за этот период.
func (n *Notifier) inactiveUsers() ([]userKey, error) {
	// Создаём таблицу напоминаний если её ещё нет
	if err := n.ensureTable(); err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(n.InactiveDays) * 24 * time.Hour).Format(time.RFC3339)
	notifyWindow := time.Now().Add(-time.Duration(n.InactiveDays) * 24 * time.Hour).Format(time.RFC3339)

	rows, err := n.db.Query(`
		SELECT DISTINCT m.platform, m.chat_id
		FROM messages m
		WHERE m.direction = 'incoming'
		GROUP BY m.platform, m.chat_id
		HAVING MAX(m.sent_at) < ?
		AND NOT EXISTS (
			SELECT 1 FROM notifications n
			WHERE n.platform = m.platform
			  AND n.chat_id = m.chat_id
			  AND n.sent_at > ?
		)
	`, cutoff, notifyWindow)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []userKey
	for rows.Next() {
		var u userKey
		if err := rows.Scan(&u.platform, &u.chatID); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, rows.Err()
}

func (n *Notifier) markNotified(platform, chatID string) error {
	_, err := n.db.Exec(`
		INSERT INTO notifications (platform, chat_id, sent_at)
		VALUES (?, ?, ?)`,
		platform, chatID, time.Now().Format(time.RFC3339),
	)
	return err
}

func (n *Notifier) ensureTable() error {
	_, err := n.db.Exec(`
		CREATE TABLE IF NOT EXISTS notifications (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			platform   TEXT NOT NULL,
			chat_id    TEXT NOT NULL,
			sent_at    TEXT NOT NULL
		)`)
	return err
}
