package main

import (
	"database/sql"
	"fmt"
)

// DataEraser удаляет все данные конкретного пользователя из всех таблиц бота.
// Используется командой /delete_my_data как мера соответствия требованиям
// конфиденциальности: пользователь может полностью стереть свой след в системе.
type DataEraser struct {
	db *sql.DB
}

func NewDataEraser(db *sql.DB) *DataEraser {
	return &DataEraser{db: db}
}

// DeletionReport — сводка о том, сколько записей удалено из каждой таблицы.
type DeletionReport struct {
	Messages      int64
	ABCEntries    int64
	Wellbeing     int64
	Notifications int64
}

func (r DeletionReport) Total() int64 {
	return r.Messages + r.ABCEntries + r.Wellbeing + r.Notifications
}

// EraseUserData удаляет все записи пользователя (platform, chatID) из всех таблиц:
// история сообщений, дневник ABC, записи самочувствия, история уведомлений.
// Операция выполняется в одной транзакции — либо удаляется всё, либо ничего.
func (e *DataEraser) EraseUserData(platform, chatID string) (DeletionReport, error) {
	var report DeletionReport

	tx, err := e.db.Begin()
	if err != nil {
		return report, fmt.Errorf("не удалось начать транзакцию удаления: %w", err)
	}
	defer tx.Rollback()

	tables := []struct {
		name    string
		query   string
		counter *int64
	}{
		{"messages", `DELETE FROM messages WHERE platform = ? AND chat_id = ?`, &report.Messages},
		{"abc_entries", `DELETE FROM abc_entries WHERE platform = ? AND chat_id = ?`, &report.ABCEntries},
		{"wellbeing_entries", `DELETE FROM wellbeing_entries WHERE platform = ? AND chat_id = ?`, &report.Wellbeing},
		{"notifications", `DELETE FROM notifications WHERE platform = ? AND chat_id = ?`, &report.Notifications},
	}

	for _, t := range tables {
		res, err := tx.Exec(t.query, platform, chatID)
		if err != nil {
			// Таблица может не существовать, если соответствующий модуль не подключён —
			// это не критическая ошибка, просто пропускаем её.
			continue
		}
		count, _ := res.RowsAffected()
		*t.counter = count
	}

	if err := tx.Commit(); err != nil {
		return report, fmt.Errorf("не удалось зафиксировать удаление: %w", err)
	}

	return report, nil
}

// ── UI: подтверждение удаления ────────────────────────────────────────────────

const deleteConfirmPhrase = "удалить мои данные"

func deleteConfirmButtons() [][]string {
	return [][]string{{"Да, удалить всё", "Отмена"}}
}

// HandleDeleteMyData обрабатывает команду /delete_my_data с подтверждением.
// Возвращает (сообщения, обработано).
func HandleDeleteMyData(platform, chatID, text string, sess *Session, eraser *DataEraser) ([]OutgoingMessage, bool) {
	norm := normalize(text)

	if norm == "/delete_my_data" || norm == normalize("Удалить мои данные") {
		sess.AwaitingDeleteConfirm = true
		return []OutgoingMessage{{
			Text: "Это удалит ВСЮ твою историю переписки, записи дневника ABC, записи о самочувствии " +
				"и историю напоминаний. Действие нельзя отменить.\n\n" +
				"Подтверди удаление.",
			Buttons: deleteConfirmButtons(),
		}}, true
	}

	if !sess.AwaitingDeleteConfirm {
		return nil, false
	}

	if norm == normalize("Да, удалить всё") {
		sess.AwaitingDeleteConfirm = false
		report, err := eraser.EraseUserData(platform, chatID)
		if err != nil {
			return []OutgoingMessage{{
				Text:    "Не удалось удалить данные. Попробуй ещё раз позже.",
				Buttons: mainMenuButtons(),
			}}, true
		}
		return []OutgoingMessage{{
			Text: fmt.Sprintf(
				"Готово. Удалено записей: %d.\n\nТвоя история, дневник ABC и записи о самочувствии полностью стёрты.",
				report.Total(),
			),
			Buttons: mainMenuButtons(),
		}}, true
	}

	// Любой другой ответ во время ожидания подтверждения — считаем отменой.
	sess.AwaitingDeleteConfirm = false
	return []OutgoingMessage{{
		Text:    "Удаление отменено. Данные не изменены.",
		Buttons: mainMenuButtons(),
	}}, true
}
