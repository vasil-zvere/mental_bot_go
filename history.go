package main

import (
	"database/sql"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type HistoryStore struct {
	db *sql.DB
}

type HistoryMessage struct {
	Platform      string
	ChatID        string
	Direction     string // incoming / outgoing
	MessageText   string
	SentAt        time.Time
	ThemeKey      string
	State         string
	QuestionIndex int
	Score         int
	ResultLevel   string
}

type HistorySummary struct {
	TotalMessages     int
	IncomingMessages  int
	OutgoingMessages  int
	TestsCompleted    int
	RepeatedTests     int
	FirstAt           time.Time
	LastAt            time.Time
	AvgTestDuration   time.Duration
	ThemeCounts       map[string]int
	ResultLevelCounts map[string]int
}

func NewHistoryStore(path string) (*HistoryStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &HistoryStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (h *HistoryStore) Close() error {
	return h.db.Close()
}

func (h *HistoryStore) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL,
		chat_id TEXT NOT NULL,
		direction TEXT NOT NULL,
		message_text TEXT NOT NULL,
		sent_at TEXT NOT NULL,
		theme_key TEXT,
		state TEXT,
		question_index INTEGER,
		score INTEGER,
		result_level TEXT
	);
	`
	_, err := h.db.Exec(schema)
	return err
}

func (h *HistoryStore) SaveMessage(m HistoryMessage) error {
	_, err := h.db.Exec(`
		INSERT INTO messages (
			platform, chat_id, direction, message_text, sent_at,
			theme_key, state, question_index, score, result_level
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Platform,
		m.ChatID,
		m.Direction,
		m.MessageText,
		m.SentAt.Format(time.RFC3339Nano),
		m.ThemeKey,
		m.State,
		m.QuestionIndex,
		m.Score,
		m.ResultLevel,
	)
	return err
}

func (h *HistoryStore) LoadMessages(platform, chatID string) ([]HistoryMessage, error) {
	rows, err := h.db.Query(`
		SELECT platform, chat_id, direction, message_text, sent_at,
		       COALESCE(theme_key, ''), COALESCE(state, ''),
		       COALESCE(question_index, 0), COALESCE(score, 0), COALESCE(result_level, '')
		FROM messages
		WHERE platform = ? AND chat_id = ?
		ORDER BY sent_at ASC
	`, platform, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []HistoryMessage
	for rows.Next() {
		var m HistoryMessage
		var sentAt string
		if err := rows.Scan(
			&m.Platform,
			&m.ChatID,
			&m.Direction,
			&m.MessageText,
			&sentAt,
			&m.ThemeKey,
			&m.State,
			&m.QuestionIndex,
			&m.Score,
			&m.ResultLevel,
		); err != nil {
			return nil, err
		}

		t, err := time.Parse(time.RFC3339Nano, sentAt)
		if err != nil {
			t = time.Now()
		}
		m.SentAt = t
		result = append(result, m)
	}

	return result, rows.Err()
}

func (h *HistoryStore) DeleteUserHistory(platform, chatID string) error {
	_, err := h.db.Exec(`DELETE FROM messages WHERE platform = ? AND chat_id = ?`, platform, chatID)
	return err
}

func (h *HistoryStore) BuildSummary(platform, chatID string) (HistorySummary, []HistoryMessage, error) {
	msgs, err := h.LoadMessages(platform, chatID)
	if err != nil {
		return HistorySummary{}, nil, err
	}

	summary := HistorySummary{
		ThemeCounts:       map[string]int{},
		ResultLevelCounts: map[string]int{},
	}

	if len(msgs) == 0 {
		return summary, msgs, nil
	}

	summary.FirstAt = msgs[0].SentAt
	summary.LastAt = msgs[len(msgs)-1].SentAt

	var startTimes []time.Time
	var totalDuration time.Duration

	for _, m := range msgs {
		summary.TotalMessages++

		if m.Direction == "incoming" {
			summary.IncomingMessages++
			if normalize(m.MessageText) == normalize("Да, начать") {
				startTimes = append(startTimes, m.SentAt)
			}
		} else {
			summary.OutgoingMessages++
		}

		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(m.MessageText)), "результат:") {
			summary.TestsCompleted++

			if m.ThemeKey != "" {
				summary.ThemeCounts[m.ThemeKey]++
			}
			if m.ResultLevel != "" {
				summary.ResultLevelCounts[m.ResultLevel]++
			}

			if len(startTimes) > 0 {
				start := startTimes[0]
				startTimes = startTimes[1:]
				totalDuration += m.SentAt.Sub(start)
			}
		}
	}

	if summary.TestsCompleted > 1 {
		summary.RepeatedTests = summary.TestsCompleted - 1
	}

	if summary.TestsCompleted > 0 {
		summary.AvgTestDuration = totalDuration / time.Duration(summary.TestsCompleted)
	}

	return summary, msgs, nil
}

func snapshotFromSession(sess *Session, content *ContentStore) (themeKey, state string, questionIndex, score int, resultLevel string) {
	if sess == nil {
		return "", "", 0, 0, ""
	}

	themeKey = sess.ThemeKey
	state = string(sess.State)
	questionIndex = sess.CurrentQ
	score = sess.Score

	if sess.ThemeKey != "" && sess.State == StateAfterResult {
		if theme, ok := content.Themes[sess.ThemeKey]; ok {
			resultLevel = theme.ThresholdLevel(sess.Score)
		}
	}

	return
}

func isMyReportCommand(text string) bool {
	n := normalize(text)
	return n == "/my_report" || n == "скачать мой отчет"
}

func isDeleteHistoryCommand(text string) bool {
	n := normalize(text)
	return n == "/delete_my_history" || n == "удалить мою историю"
}
