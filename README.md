# mental_bot_go

## RU

Это мой учебный проект на Go.  
В этом проекте я реализовал диалоговый интерфейс для Telegram и VK. Он помогает пользователю пройти короткий психологический мини-тест, получить предварительный результат, материалы по теме и рекомендации о том, что можно сделать прямо сейчас. Если результат тревожный, интерфейс предлагает обратиться к специалистам по ссылке.

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
- единая логика сценария для Telegram и VK.

### Какие технологии я использовал

- Go
- Telegram Bot API через HTTP
- VK Bots Long Poll API через HTTP
- JSON для хранения контента тестов

### Структура проекта

- `main.go` — точка входа и запуск Telegram/VK
- `app.go` — основная логика сценария и состояний
- `content.go` — загрузка данных из `content.json`
- `content.json` — темы, вопросы, результаты, FAQ и тексты
- `platform_telegram.go` — работа с Telegram
- `platform_vk.go` — работа с VK
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

### Переменные окружения

#### Telegram

```powershell
$env:TG_BOT_TOKEN="ВАШ_ТОКЕН"
```

#### VK

```powershell
$env:VK_GROUP_TOKEN="ТОКЕН_СООБЩЕСТВА"
$env:VK_GROUP_ID="123456789"
$env:VK_API_VERSION="5.199"
```

#### Если запускать Telegram и VK вместе

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
3. Записывал токен в `TG_BOT_TOKEN`.
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
5. Указывал `VK_GROUP_TOKEN`, `VK_GROUP_ID` и `VK_API_VERSION`.
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
2. Выбирает действие: начать тест, FAQ или информация.
3. Выбирает тему теста.
4. Читает описание теста и подтверждает старт.
5. Проходит вопросы по одному.
6. Получает результат.
7. Получает материалы по теме.
8. Получает блок «что сделать прямо сейчас».
9. При тревожном результате получает ссылку на специалистов.
10. Может пройти тест повторно или выбрать другую тему.

### Что важно знать

- состояние пользователя хранится только в памяти;
- после перезапуска приложения прогресс сбрасывается;
- чтобы проект работал без ПК, его нужно развернуть на сервере.

### Проблемы с которыми я столкнулся

#### Бот не видит токен в PowerShell

Я использовал такой вариант:

```powershell
$env:TG_BOT_TOKEN="ВАШ_ТОКЕН"
```

или для VK:

```powershell
$env:VK_GROUP_TOKEN="ВАШ_VK_ТОКЕН"
$env:VK_GROUP_ID="123456789"
```

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

### Что можно доработать

- полностью вынести кнопки и меню в JSON;
- добавить постоянное хранение состояния;
- подключить webhook вместо long polling;
- добавить новые темы тестов;
- оформить более подробное логирование.

---

## EN

This is my study project in Go. In this project, I implemented a dialog interface for Telegram and VK. It helps the user to take a short psychological mini-test, get a preliminary result, materials on the topic, and recommendations on what can be done right now. If the result is alarming, the interface suggests contacting specialists via a link.

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
- have a unified script logic for Telegram and VK.

### What technologies I used

- Go
- Telegram Bot API via HTTP
- VK Bots Long Poll API via HTTP
- JSON for storing test content

### Project structure

- `main.go` — entry point and Telegram/VK launch
- `app.go` — main script logic and states
- `content.go` — loading data from `content.json`
- `content.json` — topics, questions, results, FAQ, and texts
- `platform_telegram.go` — working with Telegram
- `platform_vk.go` — working with VK
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

### Environment variables

#### Telegram

```powershell
$env:TG_BOT_TOKEN="YOUR_TOKEN"
```

#### VK

```powershell
$env:VK_GROUP_TOKEN="COMMUNITY_TOKEN"
$env:VK_GROUP_ID="123456789"
$env:VK_API_VERSION="5.199"
```

#### If you run Telegram and VK together

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
3. Write the token in `TG_BOT_TOKEN`.
4. Run the project:

```powershell
go run .
```

5. After that, open the bot in Telegram and click `/start`.

### How I tested VK

1. Created a VK community.
2. Enabled community messages.
3. Enabled Long Poll API.
4. Received a community token.
5. Specify `VK_GROUP_TOKEN`, `VK_GROUP_ID` and `VK_API_VERSION`.
6. Run the project.
7. Send a message to the community.

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
2. Selects an action: start the test, FAQ, or information.
3. Selects the test topic.
4. Reads the test description and confirms the start.
5. Passes the questions one by one.
6. Gets the result.
7. Gets the materials on the topic.
8. Gets the block "what to do right now."
9. If the result is alarming, gets a link to specialists.
10. Can retake the test or select a different topic.

### What is important to know

- the user's state is only stored in memory;
- after restarting the application, the progress is reset;
- to make the project work without a PC, it needs to be deployed on a server.

### Problems I encountered

#### The bot doesn't see the token in PowerShell

I used this option:

```powershell
$env:TG_BOT_TOKEN="YOUR_TOKEN"
```

or for VK:

```powershell
$env:VK_GROUP_TOKEN="YOUR_VK_TOKEN"
$env:VK_GROUP_ID="123456789"
```

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

### What can be improved

- completely remove the buttons and menu in JSON;
- add persistent state storage;
- connect a webhook instead of long polling;
- add new test topics;
- create more detailed logging.