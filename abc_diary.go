package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ─── ABC diary states ────────────────────────────────────────────────────────

type ABCState string

const (
	ABCStateIdle     ABCState = ""
	ABCStateEvent    ABCState = "abc_event"    // A – Активирующее событие
	ABCStateReaction ABCState = "abc_reaction" // B – Реакция / мысли
	ABCStateEmotion  ABCState = "abc_emotion"  // C – Эмоции
	ABCStateAction   ABCState = "abc_action"   // C – Поведение / что сделал
	ABCStateThoughts ABCState = "abc_thoughts" // доп. – мысли в момент события
	ABCStateWhen     ABCState = "abc_when"     // когда произошло
	ABCStateDone     ABCState = "abc_done"     // запись сохранена
)

// ABCEntry — одна запись дневника
type ABCEntry struct {
	ID        int64
	Platform  string
	ChatID    string
	CreatedAt time.Time
	When      string // когда произошло (свободный текст)
	Event     string // A
	Reaction  string // B
	Emotion   string // C (эмоции)
	Action    string // C (поведение)
	Thoughts  string // мысли в момент события
}

// ─── Store ────────────────────────────────────────────────────────────────────

type ABCStore struct {
	db *sql.DB
}

func NewABCStore(db *sql.DB) (*ABCStore, error) {
	schema := `
	CREATE TABLE IF NOT EXISTS abc_entries (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		platform    TEXT NOT NULL,
		chat_id     TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		event_when  TEXT,
		event       TEXT,
		reaction    TEXT,
		emotion     TEXT,
		action      TEXT,
		thoughts    TEXT
	);`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &ABCStore{db: db}, nil
}

func (s *ABCStore) Save(e ABCEntry) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO abc_entries
			(platform, chat_id, created_at, event_when, event, reaction, emotion, action, thoughts)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Platform,
		e.ChatID,
		e.CreatedAt.Format(time.RFC3339),
		e.When,
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

func (s *ABCStore) LoadAll(platform, chatID string) ([]ABCEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, platform, chat_id, created_at, event_when, event, reaction, emotion, action, thoughts
		FROM abc_entries
		WHERE platform = ? AND chat_id = ?
		ORDER BY created_at DESC`,
		platform, chatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ABCEntry
	for rows.Next() {
		var e ABCEntry
		var createdAt string
		if err := rows.Scan(
			&e.ID, &e.Platform, &e.ChatID, &createdAt,
			&e.When, &e.Event, &e.Reaction, &e.Emotion, &e.Action, &e.Thoughts,
		); err != nil {
			return nil, err
		}
		t, _ := time.Parse(time.RFC3339, createdAt)
		e.CreatedAt = t
		result = append(result, e)
	}
	return result, rows.Err()
}

// ─── In-progress draft (in-memory) ───────────────────────────────────────────

type ABCDraft struct {
	State    ABCState
	When     string
	Event    string
	Reaction string
	Emotion  string
	Action   string
	Thoughts string
}

// ─── ABC flow helpers ─────────────────────────────────────────────────────────

func abcMenuButtons() [][]string {
	return [][]string{
		{"📓 Новая запись в дневнике"},
		{"📋 Мои записи"},
		{"В меню"},
	}
}

func abcCancelButtons() [][]string {
	return [][]string{{"Отменить запись", "В меню"}}
}

// HandleABC обрабатывает входящий текст, когда пользователь находится в ABC-флоу.
// Возвращает (messages, handled): если handled=false — вызывающий должен обработать сам.
func HandleABC(
	platform, chatID, text string,
	sess *Session,
	draft *ABCDraft,
	store *ABCStore,
) ([]OutgoingMessage, bool) {

	norm := normalize(text)

	// ── Вход в ABC-дневник из любого состояния ──────────────────────────────
	if norm == normalize("📓 Новая запись в дневнике") || norm == normalize("дневник abc") || norm == normalize("abc дневник") {
		draft.State = ABCStateEvent
		draft.Event = ""
		draft.Reaction = ""
		draft.Emotion = ""
		draft.Action = ""
		draft.Thoughts = ""
		draft.When = ""

		return []OutgoingMessage{{
			Text: "📓 *Дневник ABC* — помогает разобраться, как события влияют на наши мысли и чувства.\n\n" +
				"*Шаг 1 из 6 — Событие*\n\n" +
				"Что произошло? Опиши событие, которое тебя взволновало — в нескольких словах или подробно, как тебе удобно.",
			Buttons: abcCancelButtons(),
		}}, true
	}

	// ── Просмотр записей ────────────────────────────────────────────────────
	if norm == normalize("📋 Мои записи") {
		entries, err := store.LoadAll(platform, chatID)
		if err != nil || len(entries) == 0 {
			return []OutgoingMessage{{
				Text:    "Записей пока нет. Начни с «📓 Новая запись в дневнике».",
				Buttons: abcMenuButtons(),
			}}, true
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("📋 Твои записи (%d):\n", len(entries)))
		limit := 5
		if len(entries) < limit {
			limit = len(entries)
		}
		for i := 0; i < limit; i++ {
			e := entries[i]
			b.WriteString(fmt.Sprintf(
				"\n─────────────\n🗓 %s\n📌 Событие: %s\n💭 Реакция: %s\n😔 Эмоции: %s\n⚡ Действие: %s\n🧠 Мысли: %s\n",
				e.CreatedAt.Format("02.01.2006 15:04"),
				truncate(e.Event, 80),
				truncate(e.Reaction, 80),
				truncate(e.Emotion, 80),
				truncate(e.Action, 80),
				truncate(e.Thoughts, 80),
			))
		}
		if len(entries) > limit {
			b.WriteString(fmt.Sprintf("\n…и ещё %d записей.", len(entries)-limit))
		}
		return []OutgoingMessage{{Text: b.String(), Buttons: abcMenuButtons()}}, true
	}

	// ── Если нет активного черновика — не обрабатываем ───────────────────────
	if draft.State == ABCStateIdle {
		return nil, false
	}

	// ── Отмена ───────────────────────────────────────────────────────────────
	if norm == normalize("Отменить запись") || norm == "в меню" || norm == "меню" || norm == "/start" || norm == "start" {
		draft.State = ABCStateIdle
		return nil, false // пусть основной engine обработает навигацию
	}

	// ── Шаги заполнения ──────────────────────────────────────────────────────
	switch draft.State {

	case ABCStateEvent:
		draft.Event = strings.TrimSpace(text)
		draft.State = ABCStateReaction
		return []OutgoingMessage{{
			Text: "✅ Записала событие.\n\n" +
				"*Шаг 2 из 6 — Реакция*\n\n" +
				"Как ты отреагировал(а) на это событие? Что первым делом пришло в голову или в тело?",
			Buttons: abcCancelButtons(),
		}}, true

	case ABCStateReaction:
		draft.Reaction = strings.TrimSpace(text)
		draft.State = ABCStateEmotion
		return []OutgoingMessage{{
			Text: "✅ Записала реакцию.\n\n" +
				"*Шаг 3 из 6 — Эмоции*\n\n" +
				"Что ты почувствовал(а)? Постарайся назвать конкретные эмоции — тревога, злость, обида, стыд, грусть, растерянность…",
			Buttons: abcCancelButtons(),
		}}, true

	case ABCStateEmotion:
		draft.Emotion = strings.TrimSpace(text)
		draft.State = ABCStateAction
		return []OutgoingMessage{{
			Text: "✅ Записала эмоции.\n\n" +
				"*Шаг 4 из 6 — Действия*\n\n" +
				"Что ты сделал(а)? Как ты повёл(а) себя после этого события?",
			Buttons: abcCancelButtons(),
		}}, true

	case ABCStateAction:
		draft.Action = strings.TrimSpace(text)
		draft.State = ABCStateThoughts
		return []OutgoingMessage{{
			Text: "✅ Записала действия.\n\n" +
				"*Шаг 5 из 6 — Мысли в момент события*\n\n" +
				"Когда это событие произошло — какие мысли у тебя возникли? " +
				"Что ты говорил(а) себе внутри?",
			Buttons: abcCancelButtons(),
		}}, true

	case ABCStateThoughts:
		draft.Thoughts = strings.TrimSpace(text)
		draft.State = ABCStateWhen
		return []OutgoingMessage{{
			Text: "✅ Записала мысли.\n\n" +
				"*Шаг 6 из 6 — Время события*\n\n" +
				"Когда это произошло? Напиши дату и время в любом удобном формате, например: «вчера вечером», «2 июня, около 14:00» или «недавно».",
			Buttons: abcCancelButtons(),
		}}, true

	case ABCStateWhen:
		draft.When = strings.TrimSpace(text)
		draft.State = ABCStateDone

		// Сохраняем в базу
		entry := ABCEntry{
			Platform:  platform,
			ChatID:    chatID,
			CreatedAt: time.Now(),
			When:      draft.When,
			Event:     draft.Event,
			Reaction:  draft.Reaction,
			Emotion:   draft.Emotion,
			Action:    draft.Action,
			Thoughts:  draft.Thoughts,
		}
		_, _ = store.Save(entry)

		summary := fmt.Sprintf(
			"✅ *Запись сохранена!*\n\n"+
				"🗓 Когда: %s\n"+
				"📌 Событие: %s\n"+
				"💭 Реакция: %s\n"+
				"😔 Эмоции: %s\n"+
				"⚡ Действия: %s\n"+
				"🧠 Мысли: %s\n\n"+
				"Дневник помогает замечать связь между событиями, мыслями и чувствами. "+
				"Со временем ты сможешь видеть паттерны и реагировать осознаннее. 💙",
			draft.When,
			draft.Event,
			draft.Reaction,
			draft.Emotion,
			draft.Action,
			draft.Thoughts,
		)

		draft.State = ABCStateIdle
		return []OutgoingMessage{{Text: summary, Buttons: abcMenuButtons()}}, true
	}

	return nil, false
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}
