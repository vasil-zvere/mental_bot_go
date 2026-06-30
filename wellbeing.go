package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ─── Wellbeing flow states ────────────────────────────────────────────────────

type WellbeingState string

const (
	WBStateIdle       WellbeingState = ""
	WBStateVenting    WellbeingState = "wb_venting"    // шаг 0: дать выговориться
	WBStateEvent      WellbeingState = "wb_event"      // шаг 1: что произошло
	WBStateReaction   WellbeingState = "wb_reaction"   // шаг 2: как отреагировал
	WBStateEmotion    WellbeingState = "wb_emotion"    // шаг 3: что почувствовал
	WBStateAction     WellbeingState = "wb_action"     // шаг 4: что сделал
	WBStateThoughts   WellbeingState = "wb_thoughts"   // шаг 5: какие мысли возникли
	WBStateDone       WellbeingState = "wb_done"
)

// WellbeingEntry — одна запись о самочувствии
type WellbeingEntry struct {
	ID        int64
	Platform  string
	ChatID    string
	CreatedAt time.Time
	Venting   string // свободный рассказ о самочувствии
	Event     string // что произошло
	Reaction  string // как отреагировал
	Emotion   string // что почувствовал
	Action    string // что сделал
	Thoughts  string // какие мысли возникли
}

// WellbeingDraft — черновик в памяти (один на сессию)
type WellbeingDraft struct {
	State     WellbeingState
	Venting   string
	Event     string
	Reaction  string
	Emotion   string
	Action    string
	Thoughts  string
}

// ─── Store ────────────────────────────────────────────────────────────────────

type WellbeingStore struct {
	db *sql.DB
}

func NewWellbeingStore(db *sql.DB) (*WellbeingStore, error) {
	schema := `
	CREATE TABLE IF NOT EXISTS wellbeing_entries (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		platform    TEXT NOT NULL,
		chat_id     TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		venting     TEXT,
		event       TEXT,
		reaction    TEXT,
		emotion     TEXT,
		action      TEXT,
		thoughts    TEXT
	);`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &WellbeingStore{db: db}, nil
}

func (s *WellbeingStore) Save(e WellbeingEntry) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO wellbeing_entries
			(platform, chat_id, created_at, venting, event, reaction, emotion, action, thoughts)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Platform,
		e.ChatID,
		e.CreatedAt.Format(time.RFC3339),
		e.Venting,
		e.Event,
		e.Reaction,
		e.Emotion,
		e.Action,
		e.Thoughts,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *WellbeingStore) LoadAll(platform, chatID string) ([]WellbeingEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, platform, chat_id, created_at, venting, event, reaction, emotion, action, thoughts
		FROM wellbeing_entries
		WHERE platform = ? AND chat_id = ?
		ORDER BY created_at DESC`,
		platform, chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []WellbeingEntry
	for rows.Next() {
		var e WellbeingEntry
		var createdAt string
		if err := rows.Scan(
			&e.ID, &e.Platform, &e.ChatID, &createdAt,
			&e.Venting, &e.Event, &e.Reaction, &e.Emotion, &e.Action, &e.Thoughts,
		); err != nil {
			return nil, err
		}
		t, _ := time.Parse(time.RFC3339, createdAt)
		e.CreatedAt = t
		result = append(result, e)
	}
	return result, rows.Err()
}

// ─── UI helpers ───────────────────────────────────────────────────────────────

func wbMenuButtons() [][]string {
	return [][]string{
		{"💬 Рассказать о самочувствии"},
		{"📋 Мои записи о самочувствии"},
		{"В меню"},
	}
}

func wbCancelButtons() [][]string {
	return [][]string{{"Пропустить", "Отменить"}}
}

// ─── Flow handler ─────────────────────────────────────────────────────────────

// HandleWellbeing обрабатывает входящий текст в рамках флоу «жалоба на самочувствие».
// Возвращает (messages, handled).
func HandleWellbeing(
	platform, chatID, text string,
	sess *Session,
	draft *WellbeingDraft,
	store *WellbeingStore,
) ([]OutgoingMessage, bool) {

	norm := normalize(text)

	// ── Вход в раздел ─────────────────────────────────────────────────────
	if norm == normalize("💬 Рассказать о самочувствии") || norm == normalize("самочувствие") {
		*draft = WellbeingDraft{State: WBStateVenting}
		return []OutgoingMessage{{
			Text: "💙 Я здесь и готова выслушать тебя.\n\n" +
				"Расскажи, как ты себя чувствуешь прямо сейчас — в свободной форме, столько, сколько хочется. " +
				"Это пространство только для тебя.",
			Buttons: wbCancelButtons(),
		}}, true
	}

	// ── Просмотр записей ──────────────────────────────────────────────────
	if norm == normalize("📋 Мои записи о самочувствии") {
		entries, err := store.LoadAll(platform, chatID)
		if err != nil || len(entries) == 0 {
			return []OutgoingMessage{{
				Text:    "Записей пока нет. Нажми «💬 Рассказать о самочувствии», чтобы начать.",
				Buttons: wbMenuButtons(),
			}}, true
		}
		var b strings.Builder
		limit := 5
		if len(entries) < limit {
			limit = len(entries)
		}
		b.WriteString(fmt.Sprintf("📋 Твои записи о самочувствии (%d последних):\n", limit))
		for i := 0; i < limit; i++ {
			e := entries[i]
			b.WriteString(fmt.Sprintf(
				"\n─────────────\n🗓 %s\n💬 Как чувствовал(а): %s\n📌 Событие: %s\n💭 Реакция: %s\n😔 Эмоции: %s\n⚡ Действия: %s\n🧠 Мысли: %s\n",
				e.CreatedAt.Format("02.01.2006 15:04"),
				truncateWB(e.Venting, 80),
				truncateWB(e.Event, 80),
				truncateWB(e.Reaction, 80),
				truncateWB(e.Emotion, 80),
				truncateWB(e.Action, 80),
				truncateWB(e.Thoughts, 80),
			))
		}
		if len(entries) > limit {
			b.WriteString(fmt.Sprintf("\n…и ещё %d записей.", len(entries)-limit))
		}
		return []OutgoingMessage{{Text: b.String(), Buttons: wbMenuButtons()}}, true
	}

	// ── Если нет активного черновика — не обрабатываем ────────────────────
	if draft.State == WBStateIdle {
		return nil, false
	}

	// ── Отмена ────────────────────────────────────────────────────────────
	if norm == normalize("Отменить") || norm == "в меню" || norm == "меню" || norm == "/start" || norm == "start" {
		*draft = WellbeingDraft{State: WBStateIdle}
		return nil, false // пусть основной engine обработает навигацию
	}

	// ── «Пропустить» — переходим к следующему шагу с пустым значением ────
	skip := norm == normalize("Пропустить")

	// ── Шаги ──────────────────────────────────────────────────────────────
	switch draft.State {

	case WBStateVenting:
		if !skip {
			draft.Venting = strings.TrimSpace(text)
		}
		draft.State = WBStateEvent
		return []OutgoingMessage{{
			Text: "Спасибо, что поделился(ась). Я слышу тебя. 💙\n\n" +
				"Теперь давай разберёмся вместе.\n\n" +
				"*Шаг 1 — Событие*\n\n" +
				"Что произошло? Опиши конкретную ситуацию или событие, которое повлияло на твоё самочувствие.",
			Buttons: wbCancelButtons(),
		}}, true

	case WBStateEvent:
		if !skip {
			draft.Event = strings.TrimSpace(text)
		}
		draft.State = WBStateReaction
		return []OutgoingMessage{{
			Text: "✅ Записала.\n\n" +
				"*Шаг 2 — Реакция*\n\n" +
				"Как ты отреагировал(а) на это событие? " +
				"Что произошло в теле или в мыслях в первый момент?",
			Buttons: wbCancelButtons(),
		}}, true

	case WBStateReaction:
		if !skip {
			draft.Reaction = strings.TrimSpace(text)
		}
		draft.State = WBStateEmotion
		return []OutgoingMessage{{
			Text: "✅ Записала.\n\n" +
				"*Шаг 3 — Чувства*\n\n" +
				"Что ты почувствовал(а)? " +
				"Постарайся назвать эмоции — тревога, злость, обида, усталость, пустота, грусть…",
			Buttons: wbCancelButtons(),
		}}, true

	case WBStateEmotion:
		if !skip {
			draft.Emotion = strings.TrimSpace(text)
		}
		draft.State = WBStateAction
		return []OutgoingMessage{{
			Text: "✅ Записала.\n\n" +
				"*Шаг 4 — Действия*\n\n" +
				"Что ты сделал(а) после этого? " +
				"Как ты повёл(а) себя — ушёл(ушла), промолчал(а), позвонил(а), избегал(а)?",
			Buttons: wbCancelButtons(),
		}}, true

	case WBStateAction:
		if !skip {
			draft.Action = strings.TrimSpace(text)
		}
		draft.State = WBStateThoughts
		return []OutgoingMessage{{
			Text: "✅ Записала.\n\n" +
				"*Шаг 5 — Мысли*\n\n" +
				"Когда это событие произошло — какие мысли у тебя возникли? " +
				"Что ты говорил(а) себе внутри в тот момент?",
			Buttons: wbCancelButtons(),
		}}, true

	case WBStateThoughts:
		if !skip {
			draft.Thoughts = strings.TrimSpace(text)
		}
		draft.State = WBStateDone

		// Сохраняем в базу
		entry := WellbeingEntry{
			Platform:  platform,
			ChatID:    chatID,
			CreatedAt: time.Now(),
			Venting:   draft.Venting,
			Event:     draft.Event,
			Reaction:  draft.Reaction,
			Emotion:   draft.Emotion,
			Action:    draft.Action,
			Thoughts:  draft.Thoughts,
		}
		_, _ = store.Save(entry)

		summary := fmt.Sprintf(
			"✅ *Всё записала.*\n\n"+
				"💬 Самочувствие: %s\n"+
				"📌 Событие: %s\n"+
				"💭 Реакция: %s\n"+
				"😔 Эмоции: %s\n"+
				"⚡ Действия: %s\n"+
				"🧠 Мысли: %s\n\n"+
				"Ты проделал(а) важную работу — заметить, что происходит внутри. "+
				"Иногда уже одного этого достаточно, чтобы стало чуть легче. "+
				"Если хочется разобраться глубже — попробуй 📓 Дневник ABC. 💙",
			orDash(draft.Venting),
			orDash(draft.Event),
			orDash(draft.Reaction),
			orDash(draft.Emotion),
			orDash(draft.Action),
			orDash(draft.Thoughts),
		)

		*draft = WellbeingDraft{State: WBStateIdle}
		return []OutgoingMessage{{Text: summary, Buttons: wbMenuButtons()}}, true
	}

	return nil, false
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func truncateWB(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}
