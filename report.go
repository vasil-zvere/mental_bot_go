package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jung-kurt/gofpdf"
)

type ReportService struct {
	history   *HistoryStore
	content   *ContentStore
	abc       *ABCStore
	wellbeing *WellbeingStore
}

func NewReportService(history *HistoryStore, content *ContentStore) *ReportService {
	return &ReportService{history: history, content: content}
}

func (r *ReportService) SetABCStore(store *ABCStore)             { r.abc = store }
func (r *ReportService) SetWellbeingStore(store *WellbeingStore) { r.wellbeing = store }

// stripEmoji удаляет символы вне базового диапазона DejaVu (BMP, кроме PUA).
func stripEmoji(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r <= 0xFFFF && !unicode.Is(unicode.Co, r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.TrimSpace(b.String())
}

func (r *ReportService) GenerateUserReport(platform, chatID string) (string, error) {
	summary, messages, err := r.history.BuildSummary(platform, chatID)
	if err != nil {
		return "", err
	}

	fontPath, err := findReportFont()
	if err != nil {
		return "", err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.SetAutoPageBreak(true, 20)
	pdf.AddUTF8Font("dejavu", "", fontPath)

	w := 170.0 // рабочая ширина

	// ── Обложка ───────────────────────────────────────────────────────────
	pdf.AddPage()

	pdf.SetFillColor(50, 90, 160)
	pdf.Rect(0, 0, 210, 40, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("dejavu", "", 18)
	pdf.SetXY(20, 12)
	pdf.CellFormat(w, 10, "Персональный отчёт пользователя", "", 1, "C", false, 0, "")
	pdf.SetFont("dejavu", "", 11)
	pdf.SetXY(20, 25)
	pdf.CellFormat(w, 7, "Психологический ассистент — MentalBot", "", 1, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	pdf.SetXY(20, 50)
	pdf.SetFont("dejavu", "", 11)
	metaLines := []string{
		"Платформа:           " + strings.ToUpper(platform),
		"Идентификатор чата:  " + chatID,
		"Дата формирования:   " + time.Now().Format("02.01.2006 15:04:05"),
	}
	for _, l := range metaLines {
		pdf.MultiCell(w, 7, l, "", "L", false)
	}

	// ── Раздел 1: Тесты ───────────────────────────────────────────────────
	pdf.Ln(4)
	r.bigHeader(pdf, w, "1. ПСИХОЛОГИЧЕСКИЕ ТЕСТЫ")

	r.subHeader(pdf, w, "Общая статистика")
	pdf.SetFont("dejavu", "", 11)

	firstAt, lastAt := "-", "-"
	if !summary.FirstAt.IsZero() {
		firstAt = summary.FirstAt.Format("02.01.2006 15:04")
	}
	if !summary.LastAt.IsZero() {
		lastAt = summary.LastAt.Format("02.01.2006 15:04")
	}

	statRows := [][2]string{
		{"Завершено тестов", fmt.Sprintf("%d", summary.TestsCompleted)},
		{"Повторных прохождений", fmt.Sprintf("%d", summary.RepeatedTests)},
		{"Средняя длительность теста", formatDuration(summary.AvgTestDuration)},
		{"Всего сообщений в диалоге", fmt.Sprintf("%d", summary.TotalMessages)},
		{"Первое обращение", firstAt},
		{"Последнее обращение", lastAt},
	}
	r.table(pdf, w, statRows)

	pdf.Ln(3)
	r.subHeader(pdf, w, "По темам тестов")
	pdf.SetFont("dejavu", "", 11)
	themeKeys := sortedKeys(summary.ThemeCounts)
	if len(themeKeys) == 0 {
		pdf.MultiCell(w, 6, "Тестов ещё не было.", "", "L", false)
	} else {
		var rows [][2]string
		for _, k := range themeKeys {
			rows = append(rows, [2]string{r.themeTitle(k), fmt.Sprintf("%d раз", summary.ThemeCounts[k])})
		}
		r.table(pdf, w, rows)
	}

	pdf.Ln(3)
	r.subHeader(pdf, w, "По уровням результата")
	levelKeys := sortedKeys(summary.ResultLevelCounts)
	if len(levelKeys) == 0 {
		pdf.SetFont("dejavu", "", 11)
		pdf.MultiCell(w, 6, "Данных о результатах нет.", "", "L", false)
	} else {
		var rows [][2]string
		for _, k := range levelKeys {
			rows = append(rows, [2]string{r.levelLabel(k) + " уровень", fmt.Sprintf("%d раз", summary.ResultLevelCounts[k])})
		}
		r.table(pdf, w, rows)
	}

	// ── Раздел 2: Самочувствие ────────────────────────────────────────────
	if r.wellbeing != nil {
		wbEntries, _ := r.wellbeing.LoadAll(platform, chatID)
		pdf.Ln(6)
		r.bigHeader(pdf, w, fmt.Sprintf("2. ЗАПИСИ О САМОЧУВСТВИИ (%d)", len(wbEntries)))
		pdf.SetFont("dejavu", "", 11)
		pdf.MultiCell(w, 6,
			"Здесь собраны ответы из раздела \"Самочувствие\". "+
				"Каждая запись — это разбор одной ситуации: что произошло, "+
				"как вы отреагировали, что почувствовали, что сделали и какие мысли возникли.",
			"", "L", false)

		if len(wbEntries) == 0 {
			pdf.Ln(2)
			pdf.MultiCell(w, 6, "Записей о самочувствии пока нет.", "", "L", false)
		} else {
			for i, e := range wbEntries {
				pdf.Ln(4)
				r.entryHeader(pdf, w,
					fmt.Sprintf("Запись %d  —  %s", i+1, e.CreatedAt.Format("02.01.2006  15:04")))
				rows := [][2]string{
					{"Общее самочувствие", orDash(stripEmoji(e.Venting))},
					{"Что произошло", orDash(stripEmoji(e.Event))},
					{"Реакция на событие", orDash(stripEmoji(e.Reaction))},
					{"Эмоции", orDash(stripEmoji(e.Emotion))},
					{"Что сделал(а)", orDash(stripEmoji(e.Action))},
					{"Мысли в момент события", orDash(stripEmoji(e.Thoughts))},
				}
				r.table(pdf, w, rows)
			}
		}
	}

	// ── Раздел 3: Дневник ABC ─────────────────────────────────────────────
	if r.abc != nil {
		abcEntries, _ := r.abc.LoadAll(platform, chatID)
		pdf.Ln(6)
		r.bigHeader(pdf, w, fmt.Sprintf("3. ДНЕВНИК ABC (%d записей)", len(abcEntries)))
		pdf.SetFont("dejavu", "", 11)
		pdf.MultiCell(w, 6,
			"Дневник ABC — инструмент когнитивно-поведенческой терапии. "+
				"A (Activating event) — активирующее событие. "+
				"B (Beliefs) — мысли и убеждения. "+
				"C (Consequences) — последствия: эмоции и поведение.",
			"", "L", false)

		if len(abcEntries) == 0 {
			pdf.Ln(2)
			pdf.MultiCell(w, 6, "Записей в дневнике ABC пока нет.", "", "L", false)
		} else {
			for i, e := range abcEntries {
				pdf.Ln(4)
				r.entryHeader(pdf, w,
					fmt.Sprintf("Запись %d  —  %s  (когда: %s)",
						i+1,
						e.CreatedAt.Format("02.01.2006  15:04"),
						orDash(e.When)))
				rows := [][2]string{
					{"A — Событие", orDash(stripEmoji(e.Event))},
					{"B — Реакция / мысли", orDash(stripEmoji(e.Reaction))},
					{"C — Эмоции", orDash(stripEmoji(e.Emotion))},
					{"C — Поведение / действия", orDash(stripEmoji(e.Action))},
					{"Мысли в момент события", orDash(stripEmoji(e.Thoughts))},
				}
				r.table(pdf, w, rows)
			}
		}
	}

	// ── Раздел 4: Хронология диалога ─────────────────────────────────────
	pdf.Ln(6)
	r.bigHeader(pdf, w, fmt.Sprintf("4. ХРОНОЛОГИЯ ДИАЛОГА (%d сообщений)", len(messages)))
	pdf.SetFont("dejavu", "", 11)
	pdf.MultiCell(w, 6,
		"Полная история переписки в хронологическом порядке. "+
			"Каждая строка: дата и время | отправитель | текст сообщения.",
		"", "L", false)
	pdf.Ln(2)

	if len(messages) == 0 {
		pdf.MultiCell(w, 6, "История диалога отсутствует.", "", "L", false)
	} else {
		pdf.SetFont("dejavu", "", 9)
		for _, msg := range messages {
			dir := "Пользователь"
			if msg.Direction == "outgoing" {
				dir = "Бот            "
			}
			line := fmt.Sprintf("%s  |  %s  |  %s",
				msg.SentAt.Format("02.01 15:04:05"),
				dir,
				stripEmoji(msg.MessageText),
			)
			pdf.SetFillColor(245, 245, 245)
			fill := msg.Direction == "outgoing"
			pdf.MultiCell(w, 5, line, "B", "L", fill)
		}
	}

	// ── Подвал ────────────────────────────────────────────────────────────
	pdf.Ln(8)
	pdf.SetFont("dejavu", "", 9)
	pdf.SetTextColor(120, 120, 120)
	pdf.MultiCell(w, 5,
		"Данный отчёт сформирован автоматически и носит информационный характер. "+
			"Он не является медицинским документом и не заменяет консультацию специалиста.",
		"", "C", false)

	// ── Сохранение ────────────────────────────────────────────────────────
	fileName := fmt.Sprintf("report_%s_%s_%d.pdf", platform, safeFileName(chatID), time.Now().Unix())
	outputPath := filepath.Join(os.TempDir(), fileName)
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}

// ── Хелперы оформления ────────────────────────────────────────────────────────

// bigHeader — главный заголовок раздела (синяя полоса).
func (r *ReportService) bigHeader(pdf *gofpdf.Fpdf, w float64, title string) {
	pdf.SetFillColor(50, 90, 160)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("dejavu", "", 12)
	pdf.CellFormat(w, 8, "  "+title, "", 1, "L", true, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(2)
}

// subHeader — подзаголовок (светло-серая полоса).
func (r *ReportService) subHeader(pdf *gofpdf.Fpdf, w float64, title string) {
	pdf.SetFillColor(220, 228, 245)
	pdf.SetTextColor(30, 50, 100)
	pdf.SetFont("dejavu", "", 11)
	pdf.CellFormat(w, 7, "  "+title, "", 1, "L", true, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(1)
}

// entryHeader — заголовок отдельной записи (серый фон).
func (r *ReportService) entryHeader(pdf *gofpdf.Fpdf, w float64, title string) {
	pdf.SetFillColor(235, 235, 235)
	pdf.SetTextColor(50, 50, 50)
	pdf.SetFont("dejavu", "", 10)
	pdf.CellFormat(w, 6, "  "+title, "", 1, "L", true, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(1)
}

// table рисует двухколоночную таблицу «Поле | Значение».
func (r *ReportService) table(pdf *gofpdf.Fpdf, w float64, rows [][2]string) {
	colLabel := w * 0.38
	colValue := w * 0.62
	for i, row := range rows {
		if i%2 == 0 {
			pdf.SetFillColor(250, 250, 250)
		} else {
			pdf.SetFillColor(240, 243, 250)
		}
		pdf.SetFont("dejavu", "", 10)

		// Рисуем обе ячейки на одном Y, используя MultiCell только для значения
		x := pdf.GetX()
		y := pdf.GetY()
		pdf.SetXY(x, y)
		pdf.CellFormat(colLabel, 6, "  "+row[0], "1", 0, "L", true, 0, "")
		pdf.MultiCell(colValue, 6, row[1], "1", "L", i%2 != 0)
		// Если MultiCell перенёс строку, CellFormat уже занял высоту, ничего не делаем.
	}
}

func (r *ReportService) themeTitle(key string) string {
	if theme, ok := r.content.Themes[key]; ok {
		return theme.Title
	}
	return key
}

func (r *ReportService) levelLabel(level string) string {
	switch level {
	case "low":
		return "Низкий"
	case "medium":
		return "Умеренный"
	case "high":
		return "Высокий"
	default:
		return level
	}
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "нет данных"
	}
	s := int(d.Seconds())
	return fmt.Sprintf("%d мин. %d сек.", s/60, s%60)
}

func safeFileName(s string) string {
	rep := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_")
	return rep.Replace(s)
}

func findReportFont() (string, error) {
	path := "./DejaVuSans.ttf"
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("не найден шрифт DejaVuSans.ttf в корне проекта")
	}
	if info.Size() == 0 {
		return "", fmt.Errorf("файл DejaVuSans.ttf пустой")
	}
	return path, nil
}
