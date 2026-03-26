package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type AnswerOption struct {
	Label  string `json:"label"`
	Points int    `json:"points"`
}

type Question struct {
	Text    string         `json:"text"`
	Options []AnswerOption `json:"options"`
}

type Theme struct {
	Key              string     `json:"key"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Duration         string     `json:"duration"`
	Questions        []Question `json:"questions"`
	LowLabel         string     `json:"low_label"`
	MediumLabel      string     `json:"medium_label"`
	HighLabel        string     `json:"high_label"`
	LowResult        string     `json:"low_result"`
	MediumResult     string     `json:"medium_result"`
	HighResult       string     `json:"high_result"`
	Materials        []string   `json:"materials"`
	QuickActions     []string   `json:"quick_actions"`
	SpecialistURL    string     `json:"specialist_url"`
	SpecialistPrompt string     `json:"specialist_prompt"`
}

type rawContent struct {
	Themes    []Theme `json:"themes"`
	FAQText   string  `json:"faq_text"`
	AboutText string  `json:"about_text"`
}

type ContentStore struct {
	Themes    map[string]Theme
	FAQText   string
	AboutText string
}

func LoadContentStore(path string) (*ContentStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать %s: %w", path, err)
	}

	var raw rawContent
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("ошибка JSON в %s: %w", path, err)
	}

	store := &ContentStore{
		Themes:    make(map[string]Theme),
		FAQText:   raw.FAQText,
		AboutText: raw.AboutText,
	}

	for _, theme := range raw.Themes {
		store.Themes[theme.Key] = theme
	}

	return store, nil
}

func (c *ContentStore) ThemeButtons() [][]string {
	return [][]string{
		{c.themeTitle("anxiety"), c.themeTitle("stress")},
		{c.themeTitle("burnout"), c.themeTitle("selfesteem")},
		{"В меню"},
	}
}

func (c *ContentStore) ThemeByInput(input string) (Theme, bool) {
	norm := strings.ToLower(strings.TrimSpace(input))

	for _, theme := range c.Themes {
		if strings.ToLower(strings.TrimSpace(theme.Title)) == norm {
			return theme, true
		}
	}

	switch norm {
	case "тревожность":
		theme, ok := c.Themes["anxiety"]
		return theme, ok
	case "стресс":
		theme, ok := c.Themes["stress"]
		return theme, ok
	case "выгорание":
		theme, ok := c.Themes["burnout"]
		return theme, ok
	case "самооценка":
		theme, ok := c.Themes["selfesteem"]
		return theme, ok
	default:
		return Theme{}, false
	}
}

func (c *ContentStore) themeTitle(key string) string {
	if theme, ok := c.Themes[key]; ok && strings.TrimSpace(theme.Title) != "" {
		return theme.Title
	}

	switch key {
	case "anxiety":
		return "Тревожность"
	case "stress":
		return "Стресс"
	case "burnout":
		return "Выгорание"
	case "selfesteem":
		return "Самооценка"
	default:
		return key
	}
}

func (t Theme) ThresholdLevel(score int) string {
	switch {
	case score <= 4:
		return "low"
	case score <= 9:
		return "medium"
	default:
		return "high"
	}
}

func (t Theme) ResultText(score int) string {
	switch t.ThresholdLevel(score) {
	case "low":
		return fmt.Sprintf("Результат: %s.\n\n%s", t.LowLabel, t.LowResult)
	case "medium":
		return fmt.Sprintf("Результат: %s.\n\n%s", t.MediumLabel, t.MediumResult)
	default:
		return fmt.Sprintf("Результат: %s.\n\n%s", t.HighLabel, t.HighResult)
	}
}
