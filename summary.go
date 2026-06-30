package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
)

// ── Сводка дневника ABC ───────────────────────────────────────────────────────

// FormatABCSummary возвращает текст с записями дневника ABC за последние 7 дней.
func FormatABCSummary(entries []ABCEntry) string {
	if len(entries) == 0 {
		return "За последние 7 дней записей в дневнике ABC нет.\n\n" +
			"Нажми «Дневник ABC», чтобы сделать первую запись."
	}

	// Фильтруем записи за 7 дней
	week := time.Now().Add(-7 * 24 * time.Hour)
	var recent []ABCEntry
	for _, e := range entries {
		if e.CreatedAt.After(week) {
			recent = append(recent, e)
		}
	}

	if len(recent) == 0 {
		return "За последние 7 дней записей в дневнике ABC нет.\n\n" +
			"Нажми «Дневник ABC», чтобы сделать первую запись."
	}

	// Сортируем от новых к старым
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].CreatedAt.After(recent[j].CreatedAt)
	})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Дневник ABC — последние 7 дней (%d записей)\n", len(recent)))

	for _, e := range recent {
		b.WriteString("\n")
		b.WriteString(strings.Repeat("-", 24) + "\n")
		b.WriteString(fmt.Sprintf("Дата: %s\n", e.CreatedAt.Format("02 января, 15:04")))
		if strings.TrimSpace(e.When) != "" {
			b.WriteString(fmt.Sprintf("Когда: %s\n", e.When))
		}
		if strings.TrimSpace(e.Event) != "" {
			b.WriteString(fmt.Sprintf("A — Событие: %s\n", e.Event))
		}
		if strings.TrimSpace(e.Reaction) != "" {
			b.WriteString(fmt.Sprintf("B — Реакция: %s\n", e.Reaction))
		}
		if strings.TrimSpace(e.Emotion) != "" {
			b.WriteString(fmt.Sprintf("C — Эмоции: %s\n", e.Emotion))
		}
		if strings.TrimSpace(e.Action) != "" {
			b.WriteString(fmt.Sprintf("C — Действия: %s\n", e.Action))
		}
		if strings.TrimSpace(e.Thoughts) != "" {
			b.WriteString(fmt.Sprintf("Мысли: %s\n", e.Thoughts))
		}
	}

	b.WriteString("\n" + strings.Repeat("-", 24) + "\n")
	b.WriteString("Чтобы добавить новую запись — нажми «Дневник ABC».")
	return b.String()
}

// ── Статистика самочувствия ───────────────────────────────────────────────────

// FormatWellbeingStat возвращает текст со статистикой самочувствия за 7 дней.
func FormatWellbeingStat(entries []WellbeingEntry) string {
	if len(entries) == 0 {
		return "За последние 7 дней записей о самочувствии нет.\n\n" +
			"Нажми «Самочувствие», чтобы рассказать, как ты себя чувствуешь."
	}

	week := time.Now().Add(-7 * 24 * time.Hour)
	var recent []WellbeingEntry
	for _, e := range entries {
		if e.CreatedAt.After(week) {
			recent = append(recent, e)
		}
	}

	if len(recent) == 0 {
		return "За последние 7 дней записей о самочувствии нет.\n\n" +
			"Нажми «Самочувствие», чтобы рассказать, как ты себя чувствуешь."
	}

	// Подсчёт упоминаний эмоций
	emotionCount := map[string]int{}
	for _, e := range recent {
		for _, word := range splitWords(e.Emotion) {
			if len([]rune(word)) >= 4 {
				emotionCount[strings.ToLower(word)]++
			}
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Статистика самочувствия за 7 дней\n\n"))
	b.WriteString(fmt.Sprintf("Всего записей: %d\n", len(recent)))

	// Топ-3 эмоции
	if len(emotionCount) > 0 {
		type kv struct {
			word  string
			count int
		}
		var sorted []kv
		for w, c := range emotionCount {
			sorted = append(sorted, kv{w, c})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].count > sorted[j].count
		})
		b.WriteString("\nЧаще всего упоминаемые эмоции:\n")
		limit := 3
		if len(sorted) < limit {
			limit = len(sorted)
		}
		for i := 0; i < limit; i++ {
			b.WriteString(fmt.Sprintf("  %d. %s", i+1, capitalize(sorted[i].word)))
			if sorted[i].count > 1 {
				b.WriteString(fmt.Sprintf(" (%d раз)", sorted[i].count))
			}
			b.WriteString("\n")
		}
	}

	// Последние 3 записи кратко
	b.WriteString("\nПоследние записи:\n")
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].CreatedAt.After(recent[j].CreatedAt)
	})
	limit := 3
	if len(recent) < limit {
		limit = len(recent)
	}
	for i := 0; i < limit; i++ {
		e := recent[i]
		b.WriteString(fmt.Sprintf("\n%s\n", e.CreatedAt.Format("02 января, 15:04")))
		if v := orDash(e.Venting); v != "—" {
			b.WriteString(fmt.Sprintf("  Самочувствие: %s\n", truncateStr(v, 60)))
		}
		if v := orDash(e.Emotion); v != "—" {
			b.WriteString(fmt.Sprintf("  Эмоции: %s\n", truncateStr(v, 60)))
		}
	}

	b.WriteString("\nЧтобы добавить запись — нажми «Самочувствие».")
	return b.String()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func splitWords(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func capitalize(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	return string(unicode.ToUpper(runes[0])) + string(runes[1:])
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
