# mental_bot_go

## RU

Это мой учебный проект на Go.  
В этом проекте я реализовал диалоговый интерфейс для Telegram и VK — бота психологической самопомощи **MentalBot**. Он помогает пользователю пройти короткий психологический мини-тест, получить предварительный результат, материалы по теме и рекомендации о том, что можно сделать прямо сейчас. Если результат тревожный, интерфейс предлагает обратиться к специалистам по ссылке. Помимо тестов бот умеет вести дневник ABC, разбирать жалобы на самочувствие, напоминать о себе неактивным пользователям и формировать персональный PDF-отчёт.

### Что умеет проект

- выбор темы теста;
- описание теста перед началом;
- FAQ;
- прохождение теста по шагам;
- несколько уровней результата;
- материалы по теме результата;
- блок «что сделать прямо сейчас»;
- ссылка на специалистов при тревожном результате;
- возможность начать заново;
- повторное прохождение;
- единая логика сценария для Telegram и VK;
- раздел «Самочувствие» — пошаговый разбор ситуации (что произошло, реакция, эмоции, действия, мысли);
- дневник ABC по методике когнитивно-поведенческой терапии;
- сводка дневника ABC за последние 7 дней прямо в чате;
- статистика самочувствия за 7 дней с топ-3 эмоций;
- автоматические напоминания неактивным пользователям;
- персональный PDF-отчёт со всей историей, дневником ABC и записями самочувствия;
- команда удаления всех своих данных (`/delete_my_data`);
- ротация лог-файлов;
- конфигурация через YAML-файл с приоритетом переменных окружения.

### Какие технологии я использовал

- Go
- Telegram Bot API через HTTP
- VK Bots Long Poll API через HTTP
- JSON для хранения контента тестов
- SQLite (`modernc.org/sqlite`) для хранения истории, дневника ABC, самочувствия и уведомлений
- `gofpdf` для генерации PDF-отчётов
- `gopkg.in/yaml.v3` для конфигурации

### Структура проекта

- `main.go` — точка входа: загрузка конфигурации, логирования, запуск Telegram/VK и планировщика уведомлений
- `app.go` — основная логика сценария, состояний и маршрутизации сообщений
- `content.go` — загрузка данных из `content.json`
- `content.json` — темы, вопросы, результаты, FAQ и тексты
- `platform_telegram.go` — работа с Telegram
- `platform_vk.go` — работа с VK
- `history.go` — хранилище истории переписки (SQLite)
- `report.go` — генерация персонального PDF-отчёта
- `wellbeing.go` — флоу обработки жалоб на самочувствие
- `abc_diary.go` — дневник ABC (когнитивно-поведенческая терапия)
- `summary.go` — текстовые сводки дневника ABC и статистики самочувствия за 7 дней
- `notifier.go` — планировщик автоматических напоминаний неактивным пользователям
- `notifier_senders.go` — адаптеры отправки напоминаний для Telegram и VK
- `data_eraser.go` — удаление всех данных пользователя (`/delete_my_data`)
- `logging.go` — логирование с ротацией файлов
- `config.go` — загрузка и валидация конфигурации
- `config.yaml` — файл конфигурации (токены, путь к БД, логирование, уведомления)
- `go.mod` — модуль Go

### Что нужно для запуска

- Go 1.22 или новее
- Visual Studio Code
- расширение Go для VS Code
- токен Telegram-бота и/или токен VK-сообщества

### Как я запускал проект в Visual Studio Code

1. Открывал папку проекта в VS Code.
2. Открывал терминал.
3. Выполнял команды:

```powershell
go mod tidy
go run .
```

### Конфигурация

Основной способ настройки — файл `config.yaml` в корне проекта:

```yaml
telegram:
  token: "ВАШ_TELEGRAM_ТОКЕН"

vk:
  token: ""
  group_id: 0
  api_version: "5.199"

database:
  path: "bot_history.db"

logging:
  dir: "./logs"
  base_name: "mentalbot"

notifier:
  inactive_days: 3
```

При старте конфигурация проверяется: если не указана ни одна платформа или данные неполные, бот сообщит об этом и не запустится.

#### Переменные окружения (необязательно)

Переменные окружения переопределяют значения из `config.yaml` — это удобно при запуске в контейнере или CI, и сохраняет совместимость с прежним способом запуска.

##### Telegram

```powershell
$env:TG_BOT_TOKEN="ВАШ_ТОКЕН"
```

##### VK

```powershell
$env:VK_GROUP_TOKEN="ТОКЕН_СООБЩЕСТВА"
$env:VK_GROUP_ID="123456789"
$env:VK_API_VERSION="5.199"
```

##### Если запускать Telegram и VK вместе

```powershell
$env:TG_BOT_TOKEN="ВАШ_ТЕЛЕГРАМ_ТОКЕН"
$env:VK_GROUP_TOKEN="ВАШ_VK_ТОКЕН"
$env:VK_GROUP_ID="123456789"
$env:VK_API_VERSION="5.199"
go run .
```

### Как я проверял Telegram

1. Создавал бота через `@BotFather`.
2. Получал токен.
3. Записывал токен в `config.yaml` (или в `TG_BOT_TOKEN`).
4. Запускал проект:

```powershell
go run .
```

5. После этого открывал бота в Telegram и нажимал `/start`.

### Как я проверял VK

1. Создавал сообщество VK.
2. Включал сообщения сообщества.
3. Включал Long Poll API.
4. Получал токен сообщества.
5. Указывал `vk.token`, `vk.group_id` и `vk.api_version` в `config.yaml` (или соответствующие переменные окружения).
6. Запускал проект.
7. Отправлял сообщение сообществу.

### Как устроен контент

Контент я вынес в `content.json`.

Там можно менять:
- темы тестов;
- описания тестов;
- вопросы;
- варианты ответов и баллы;
- уровни результата;
- материалы;
- быстрые действия;
- ссылки на специалистов;
- FAQ;
- текст «О боте».

После изменения `content.json` нужно просто заново запустить проект.

### Как работает логика

1. Пользователь запускает интерфейс.
2. Выбирает действие: начать тест, FAQ, информация, самочувствие, дневник ABC, статистика, сводка дневника или удаление данных.
3. **Тест:** выбирает тему, читает описание, проходит вопросы по одному, получает результат, материалы и блок «что сделать прямо сейчас», при тревожном результате — ссылку на специалистов. Может пройти повторно.
4. **Самочувствие:** проходит пятишаговый разбор ситуации — что произошло, реакция, эмоции, действия, мысли. Каждый шаг можно пропустить.
5. **Дневник ABC:** заполняет запись по методике КПТ (событие, реакция, эмоции, действия, мысли, дата события), может посмотреть последние записи.
6. **Сводка дневника / статистика:** получает текстовую сводку записей дневника ABC или самочувствия за последние 7 дней прямо в чате.
7. **Напоминания:** если пользователь не писал боту дольше заданного срока, бот сам присылает мягкое напоминание.
8. **Отчёт:** по команде «Скачать мой отчёт» бот формирует PDF со статистикой тестов, записями самочувствия, дневником ABC и хронологией переписки.
9. **Удаление данных:** по команде `/delete_my_data` с подтверждением бот безвозвратно удаляет всю историю, дневник ABC, записи самочувствия и историю уведомлений пользователя.

### Что важно знать

- история переписки, дневник ABC, записи самочувствия и уведомления хранятся в SQLite (`bot_history.db`) — это персистентное хранилище, прогресс не сбрасывается при перезапуске;
- состояние текущего диалога (на каком шаге теста или флоу находится пользователь) хранится в памяти и сбрасывается при перезапуске приложения;
- логи пишутся в файл с ротацией (`./logs/mentalbot.log` по умолчанию) и одновременно выводятся в консоль;
- чтобы проект работал без ПК, его нужно развернуть на сервере.

### Проблемы с которыми я столкнулся

#### Бот не видит токен

Если используешь переменные окружения вместо `config.yaml`:

```powershell
$env:TG_BOT_TOKEN="ВАШ_ТОКЕН"
```

или для VK:

```powershell
$env:VK_GROUP_TOKEN="ВАШ_VK_ТОКЕН"
$env:VK_GROUP_ID="123456789"
```

Переменные окружения действуют только в текущем окне PowerShell — при перезапуске терминала их нужно задать заново, либо использовать `config.yaml`.

#### Ошибка TLS handshake timeout

Обычно это проблема сети. В таком случае стоит попробовать:
- другую Wi‑Fi сеть;
- интернет с телефона;
- VPN;
- проверить, открывается ли `https://api.telegram.org`.

#### Бот перестает отвечать после выключения ПК

Это нормально для локального запуска. Проект работает только пока:
- компьютер включен;
- есть интернет;
- процесс `go run .` не остановлен.

#### VK возвращает HTTP 405 при загрузке отчёта (kittenx)

Эта ошибка возникала из-за отсутствия обязательного параметра `type=doc` в запросе `docs.getMessagesUploadServer` — без него VK мог отдавать upload-сервер, не предназначенный для документов. Исправлено добавлением параметра и повторной попыткой загрузки со свежим `upload_url`, если первая попытка не удалась.

### Что можно доработать

- подключить webhook вместо long polling;
- добавить новые темы тестов;
- вынести состояние текущего диалога тоже в персистентное хранилище, чтобы не терять прогресс на середине теста при перезапуске;
- добавить экспорт дневника в другие форматы.

---

## EN

This is my study project in Go. In this project, I implemented a dialog interface for Telegram and VK — a psychological self-help bot called **MentalBot**. It helps the user to take a short psychological mini-test, get a preliminary result, materials on the topic, and recommendations on what can be done right now. If the result is alarming, the interface suggests contacting specialists via a link. Besides tests, the bot can keep an ABC diary, help process complaints about wellbeing, remind inactive users about itself, and generate a personal PDF report.

### What the project can do

- select a test topic;
- describe the test before starting it;
- FAQ;
- take the test in steps;
- have multiple result levels;
- have materials on the result topic;
- have a "what to do right now" block;
- have a link to specialists if the result is alarming;
- have the ability to start over;
- have the ability to retake the test;
- have a unified script logic for Telegram and VK;
- "Wellbeing" section — a step-by-step breakdown of a situation (what happened, reaction, emotions, actions, thoughts);
- ABC diary based on cognitive behavioral therapy;
- ABC diary summary for the last 7 days right in the chat;
- wellbeing statistics for the last 7 days with top-3 emotions;
- automatic reminders for inactive users;
- personal PDF report with full history, ABC diary, and wellbeing entries;
- command to delete all personal data (`/delete_my_data`);
- rotating log files;
- configuration via a YAML file with environment variable override priority.

### What technologies I used

- Go
- Telegram Bot API via HTTP
- VK Bots Long Poll API via HTTP
- JSON for storing test content
- SQLite (`modernc.org/sqlite`) for storing history, ABC diary, wellbeing, and notifications
- `gofpdf` for PDF report generation
- `gopkg.in/yaml.v3` for configuration

### Project structure

- `main.go` — entry point: loads configuration and logging, starts Telegram/VK and the notification scheduler
- `app.go` — main script logic, states, and message routing
- `content.go` — loading data from `content.json`
- `content.json` — topics, questions, results, FAQ, and texts
- `platform_telegram.go` — working with Telegram
- `platform_vk.go` — working with VK
- `history.go` — conversation history storage (SQLite)
- `report.go` — personal PDF report generation
- `wellbeing.go` — wellbeing complaint handling flow
- `abc_diary.go` — ABC diary (cognitive behavioral therapy)
- `summary.go` — text summaries of the ABC diary and wellbeing statistics for the last 7 days
- `notifier.go` — scheduler for automatic reminders to inactive users
- `notifier_senders.go` — reminder-sending adapters for Telegram and VK
- `data_eraser.go` — deletes all user data (`/delete_my_data`)
- `logging.go` — logging with file rotation
- `config.go` — configuration loading and validation
- `config.yaml` — configuration file (tokens, database path, logging, notifications)
- `go.mod` — Go module

### What you need to run it

- Go 1.22 or newer
- Visual Studio Code
- Go extension for VS Code
- Telegram bot token and/or VK community token

### How I ran the project in Visual Studio Code

1. Opened the project folder in VS Code.
2. Opened the terminal.
3. Ran the commands:

```powershell
go mod tidy
go run .
```

### Configuration

The main way to configure the bot is the `config.yaml` file in the project root:

```yaml
telegram:
  token: "YOUR_TELEGRAM_TOKEN"

vk:
  token: ""
  group_id: 0
  api_version: "5.199"

database:
  path: "bot_history.db"

logging:
  dir: "./logs"
  base_name: "mentalbot"

notifier:
  inactive_days: 3
```

On startup, the configuration is validated: if no platform is specified or the data is incomplete, the bot reports this and does not start.

#### Environment variables (optional)

Environment variables override the values from `config.yaml` — this is convenient when running in a container or CI, and keeps compatibility with the previous launch method.

##### Telegram

```powershell
$env:TG_BOT_TOKEN="YOUR_TOKEN"
```

##### VK

```powershell
$env:VK_GROUP_TOKEN="COMMUNITY_TOKEN"
$env:VK_GROUP_ID="123456789"
$env:VK_API_VERSION="5.199"
```

##### If you run Telegram and VK together

```powershell
$env:TG_BOT_TOKEN="YOUR_TELEGRAM_TOKEN"
$env:VK_GROUP_TOKEN="YOUR_VK_TOKEN"
$env:VK_GROUP_ID="123456789"
$env:VK_API_VERSION="5.199"
go run .
```

### How I tested Telegram

1. Created a bot through `@BotFather`.
2. Received a token.
3. Wrote the token into `config.yaml` (or into `TG_BOT_TOKEN`).
4. Ran the project:

```powershell
go run .
```

5. After that, opened the bot in Telegram and clicked `/start`.

### How I tested VK

1. Created a VK community.
2. Enabled community messages.
3. Enabled Long Poll API.
4. Received a community token.
5. Specified `vk.token`, `vk.group_id`, and `vk.api_version` in `config.yaml` (or the matching environment variables).
6. Ran the project.
7. Sent a message to the community.

### How the content is organized

I moved the content to `content.json`.

There you can change:
- test topics;
- test descriptions;
- questions;
- answer options and scores;
- result levels;
- materials;
- quick actions;
- links to specialists;
- FAQ;
- text "About the bot".

After changing `content.json`, you just need to restart the project.

### How the logic works

1. The user launches the interface.
2. Selects an action: start the test, FAQ, information, wellbeing, ABC diary, statistics, diary summary, or data deletion.
3. **Test:** selects a topic, reads the description, passes the questions one by one, gets the result, materials, and the "what to do right now" block, and if the result is alarming — a link to specialists. Can retake the test.
4. **Wellbeing:** goes through a five-step breakdown of a situation — what happened, reaction, emotions, actions, thoughts. Each step can be skipped.
5. **ABC diary:** fills in an entry based on the CBT method (event, reaction, emotions, actions, thoughts, date of the event), can view recent entries.
6. **Diary summary / statistics:** receives a text summary of ABC diary or wellbeing entries for the last 7 days right in the chat.
7. **Reminders:** if the user hasn't written to the bot for longer than the configured period, the bot sends a gentle reminder on its own.
8. **Report:** using the "Download my report" command, the bot generates a PDF with test statistics, wellbeing entries, the ABC diary, and the conversation history.
9. **Data deletion:** using the `/delete_my_data` command with confirmation, the bot permanently deletes all of the user's history, ABC diary, wellbeing entries, and notification history.

### What is important to know

- conversation history, the ABC diary, wellbeing entries, and notifications are stored in SQLite (`bot_history.db`) — this is persistent storage, progress is not reset on restart;
- the current dialog state (which step of a test or flow the user is on) is stored in memory and is reset when the application restarts;
- logs are written to a rotating file (`./logs/mentalbot.log` by default) and printed to the console at the same time;
- to make the project work without a PC, it needs to be deployed on a server.

### Problems I encountered

#### The bot doesn't see the token

If using environment variables instead of `config.yaml`:

```powershell
$env:TG_BOT_TOKEN="YOUR_TOKEN"
```

or for VK:

```powershell
$env:VK_GROUP_TOKEN="YOUR_VK_TOKEN"
$env:VK_GROUP_ID="123456789"
```

Environment variables only apply within the current PowerShell window — they need to be set again after restarting the terminal, or use `config.yaml` instead.

#### TLS handshake timeout error

This is usually a network issue. In this case, you should try:
- a different Wi‑Fi network;
- internet from a phone;
- VPN;
- check if `https://api.telegram.org` opens.

#### The bot stops responding after the PC is turned off

This is normal for local launch. The project only works while:
- the computer is turned on;
- there is an internet connection;
- the process `go run .` is not stopped.

#### VK returns HTTP 405 when uploading a report (kittenx)

This error occurred due to a missing required `type=doc` parameter in the `docs.getMessagesUploadServer` request — without it, VK could return an upload server not intended for documents. Fixed by adding the parameter and retrying the upload with a fresh `upload_url` if the first attempt failed.

### What can be improved

- connect a webhook instead of long polling;
- add new test topics;
- also move the current dialog state into persistent storage so progress isn't lost mid-test on restart;
- add diary export to other formats.