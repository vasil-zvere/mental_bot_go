package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type State string

const (
	StateMainMenu      State = "main_menu"
	StateChoosingTheme State = "choosing_theme"
	StateConfirmTest   State = "confirm_test"
	StateAnswering     State = "answering"
	StateAfterResult   State = "after_result"
)

type Session struct {
	State         State
	ThemeKey      string
	CurrentQ      int
	Score         int
	AwaitingStart bool
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session)}
}

func (s *SessionStore) Get(key string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	if sess, ok := s.sessions[key]; ok {
		return sess
	}

	sess := &Session{State: StateMainMenu}
	s.sessions[key] = sess
	return sess
}

func (s *SessionStore) Reset(key string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess := &Session{State: StateMainMenu}
	s.sessions[key] = sess
	return sess
}

type OutgoingMessage struct {
	Text    string
	Buttons [][]string
}

type Engine struct {
	content  *ContentStore
	sessions *SessionStore
}

func NewEngine(content *ContentStore) *Engine {
	return &Engine{
		content:  content,
		sessions: NewSessionStore(),
	}
}

func normalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "ё", "е")
	return s
}

func mainMenuButtons() [][]string {
	return [][]string{{"Начать тест", "FAQ"}, {"О боте", "Выйти"}}
}

func afterResultButtons() [][]string {
	return [][]string{{"Пройти тест повторно", "Выбрать другую тему"}, {"FAQ", "Выйти"}}
}

func backButtons() [][]string {
	return [][]string{{"В меню"}}
}

func confirmButtons() [][]string {
	return [][]string{{"Да, начать", "Назад"}, {"FAQ", "В меню"}}
}

func exitMessage() OutgoingMessage {
	return OutgoingMessage{
		Text:    "Спасибо за использование бота. Чтобы начать снова, напиши /start или нажми «В меню».",
		Buttons: mainMenuButtons(),
	}
}

func (e *Engine) HandleInput(platform, chatID, rawText string) []OutgoingMessage {
	key := platform + ":" + chatID
	sess := e.sessions.Get(key)
	text := strings.TrimSpace(rawText)
	norm := normalize(text)

	if norm == "/start" || norm == "start" || norm == "в меню" || norm == "меню" {
		sess.State = StateMainMenu
		sess.ThemeKey = ""
		sess.CurrentQ = 0
		sess.Score = 0

		return []OutgoingMessage{{
			Text:    "Привет. Я помогу пройти короткий психологический мини-тест, показать предварительный результат, материалы по теме и подсказать, что можно сделать прямо сейчас.",
			Buttons: mainMenuButtons(),
		}}
	}

	if norm == "выход" {
		e.sessions.Reset(key)
		return []OutgoingMessage{exitMessage()}
	}

	if norm == "faq" {
		return []OutgoingMessage{{Text: e.content.FAQText, Buttons: backButtons()}}
	}

	if norm == normalize("О боте") || norm == normalize("Подробнее о боте") {
		return []OutgoingMessage{{Text: e.content.AboutText, Buttons: backButtons()}}
	}

	if norm == normalize("Начать заново") || norm == normalize("Выбрать другую тему") {
		sess.State = StateChoosingTheme
		sess.ThemeKey = ""
		sess.CurrentQ = 0
		sess.Score = 0
		return []OutgoingMessage{{Text: "Выбери тему теста:", Buttons: e.content.ThemeButtons()}}
	}

	if norm == normalize("Пройти тест повторно") {
		if sess.ThemeKey != "" {
			theme := e.content.Themes[sess.ThemeKey]
			sess.State = StateConfirmTest
			sess.CurrentQ = 0
			sess.Score = 0
			return []OutgoingMessage{{Text: themeIntroText(theme), Buttons: confirmButtons()}}
		}

		sess.State = StateChoosingTheme
		return []OutgoingMessage{{Text: "Выбери тему теста:", Buttons: e.content.ThemeButtons()}}
	}

	switch sess.State {
	case StateMainMenu:
		return e.handleMainMenu(sess, norm)
	case StateChoosingTheme:
		return e.handleChoosingTheme(sess, norm)
	case StateConfirmTest:
		return e.handleConfirmTest(sess, norm)
	case StateAnswering:
		return e.handleAnswer(sess, norm)
	case StateAfterResult:
		return []OutgoingMessage{{Text: "Выбери, что делать дальше:", Buttons: afterResultButtons()}}
	default:
		sess.State = StateMainMenu
		return []OutgoingMessage{{Text: "Вернемся в главное меню.", Buttons: mainMenuButtons()}}
	}
}

func (e *Engine) handleMainMenu(sess *Session, norm string) []OutgoingMessage {
	switch norm {
	case normalize("Начать тест"):
		sess.State = StateChoosingTheme
		return []OutgoingMessage{{Text: "Выбери тему теста:", Buttons: e.content.ThemeButtons()}}
	case normalize("FAQ"):
		return []OutgoingMessage{{Text: e.content.FAQText, Buttons: backButtons()}}
	case normalize("О боте"):
		return []OutgoingMessage{{Text: e.content.AboutText, Buttons: backButtons()}}
	default:
		return []OutgoingMessage{{Text: "Выбери действие с помощью кнопок ниже.", Buttons: mainMenuButtons()}}
	}
}

func (e *Engine) handleChoosingTheme(sess *Session, norm string) []OutgoingMessage {
	theme, ok := e.content.ThemeByInput(norm)
	if !ok {
		return []OutgoingMessage{{
			Text:    "Пожалуйста, выбери одну из доступных тем.",
			Buttons: e.content.ThemeButtons(),
		}}
	}

	sess.ThemeKey = theme.Key
	sess.State = StateConfirmTest
	sess.CurrentQ = 0
	sess.Score = 0

	return []OutgoingMessage{{Text: themeIntroText(theme), Buttons: confirmButtons()}}
}

func themeIntroText(theme Theme) string {
	return fmt.Sprintf(
		"Тема: %s\n\n%s\n\nВопросов: %d\nПримерная длительность: %s\n\nВажно: результат предварительный и не заменяет консультацию специалиста. Готов(а) пройти тест?",
		theme.Title,
		theme.Description,
		len(theme.Questions),
		theme.Duration,
	)
}

func (e *Engine) handleConfirmTest(sess *Session, norm string) []OutgoingMessage {
	switch norm {
	case normalize("Да, начать"):
		sess.State = StateAnswering
		sess.CurrentQ = 0
		sess.Score = 0
		return []OutgoingMessage{e.renderQuestion(sess)}
	case normalize("Назад"):
		sess.State = StateChoosingTheme
		return []OutgoingMessage{{Text: "Хорошо, можно выбрать другую тему.", Buttons: e.content.ThemeButtons()}}
	default:
		theme := e.content.Themes[sess.ThemeKey]
		return []OutgoingMessage{{Text: themeIntroText(theme), Buttons: confirmButtons()}}
	}
}

func (e *Engine) renderQuestion(sess *Session) OutgoingMessage {
	theme := e.content.Themes[sess.ThemeKey]
	q := theme.Questions[sess.CurrentQ]

	buttons := make([][]string, 0, len(q.Options)+1)
	for _, opt := range q.Options {
		buttons = append(buttons, []string{opt.Label})
	}
	buttons = append(buttons, []string{"Начать заново", "В меню"})

	return OutgoingMessage{
		Text:    fmt.Sprintf("%s\n\nВопрос %d из %d\n%s", theme.Title, sess.CurrentQ+1, len(theme.Questions), q.Text),
		Buttons: buttons,
	}
}

func (e *Engine) handleAnswer(sess *Session, norm string) []OutgoingMessage {
	theme := e.content.Themes[sess.ThemeKey]

	if sess.CurrentQ >= len(theme.Questions) {
		sess.State = StateAfterResult
		return []OutgoingMessage{{Text: "Тест уже завершен. Выбери, что делать дальше.", Buttons: afterResultButtons()}}
	}

	q := theme.Questions[sess.CurrentQ]
	for _, opt := range q.Options {
		if normalize(opt.Label) == norm {
			sess.Score += opt.Points
			sess.CurrentQ++

			if sess.CurrentQ < len(theme.Questions) {
				return []OutgoingMessage{e.renderQuestion(sess)}
			}

			sess.State = StateAfterResult
			return e.renderResult(theme, sess.Score)
		}
	}

	return []OutgoingMessage{{
		Text:    "Пожалуйста, выбери один из вариантов ответа кнопкой ниже.",
		Buttons: e.renderQuestion(sess).Buttons,
	}}
}

func bullets(items []string) string {
	var b strings.Builder
	for _, item := range items {
		b.WriteString("• ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func (e *Engine) renderResult(theme Theme, score int) []OutgoingMessage {
	level := theme.ThresholdLevel(score)

	messages := []OutgoingMessage{{
		Text:    theme.ResultText(score),
		Buttons: afterResultButtons(),
	}}

	materialsText := fmt.Sprintf("Материалы по теме «%s»:\n\n%s", theme.Title, bullets(theme.Materials))
	messages = append(messages, OutgoingMessage{
		Text:    materialsText,
		Buttons: afterResultButtons(),
	})

	quickText := "Что сделать прямо сейчас:\n\n" + bullets(theme.QuickActions)
	messages = append(messages, OutgoingMessage{
		Text:    quickText,
		Buttons: afterResultButtons(),
	})

	if level == "high" {
		specialistText := theme.SpecialistPrompt + "\n" + theme.SpecialistURL
		messages = append(messages, OutgoingMessage{
			Text:    specialistText,
			Buttons: afterResultButtons(),
		})
	}

	messages = append(messages, OutgoingMessage{
		Text:    "Что дальше? Можно пройти тест повторно, выбрать другую тему, открыть FAQ или выйти.",
		Buttons: afterResultButtons(),
	})

	return messages
}

func availableThemesList(store *ContentStore) string {
	names := make([]string, 0, len(store.Themes))
	for _, t := range store.Themes {
		names = append(names, t.Title)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
