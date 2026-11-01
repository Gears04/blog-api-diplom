# REST API для блога на Go

Дипломная работа по программе «Go-разработчик с нуля». Структура проекта основана на [шаблоне диплома](https://github.com/netology-code/gopr-temp-ex).

## О проекте

API для регистрации пользователей, публикации постов и комментариев. Данные хранятся в PostgreSQL, пароли — в виде bcrypt-хешей. Для авторизации используется JWT. События создания постов и комментариев записываются в файл через канал и отдельную горутину.

Технологии: Go 1.21+, `net/http`, PostgreSQL, JWT, bcrypt, godotenv, Docker Compose.

## Структура проекта

```text
blog-api/
├── main.go
├── cmd/api/main.go
├── internal/
│   ├── app/          # запуск сервера
│   ├── config/       # настройки
│   ├── database/     # подключение к БД и миграции
│   ├── handler/      # HTTP-обработчики
│   ├── middleware/   # авторизация и логи запросов
│   ├── model/        # модели данных
│   ├── repository/   # SQL-запросы
│   ├── service/      # валидация и бизнес-логика
│   └── logger/       # запись событий в файл
├── pkg/auth/         # JWT и bcrypt
├── migrations/
├── .env.example
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

## Запуск

Все команды выполняются из папки `blog-api`. Для запуска в контейнерах нужен Docker с Compose; для локального запуска приложения — также Go 1.21+.

### Настройки

При первом запуске создайте `.env` из примера. Уже настроенный файл не перезаписывайте:

```powershell
Copy-Item .env.example .env
```

В `.env` укажите `JWT_SECRET` — случайную строку длиной не менее 32 символов. В примере это поле специально оставлено пустым. Для запуска переменная должна быть задана в `.env` или в окружении терминала. Секреты храните только в `.env`, не в `.env.example`.

В примере API использует порт `8080`, база — `15434` на компьютере и `5432` внутри Docker. Если меняете параметры `POSTGRES_*`, согласуйте их с `DATABASE_URL` для локального запуска. Переменные терминала имеют приоритет над `.env`.

### Через Docker

```powershell
docker compose up -d --build --wait
docker compose ps
Invoke-RestMethod http://localhost:8080/health
```

Оба сервиса должны иметь статус `healthy`, ответ API — `{"status":"ok"}`.

### Локальный запуск Go

Остановите контейнер API, если он уже запущен, чтобы освободить порт 8080:

```powershell
docker compose stop api
docker compose up -d --wait postgres
go mod download
go run main.go
```

Оставьте терминал открытым. Для остановки приложения нажмите Ctrl+C. Миграции выполняются автоматически при запуске.

Контейнеры останавливаются командой `docker compose down`. Данные и логи сохраняются в томах. Не добавляйте `-v`, если хотите их сохранить. Файл `.env` не загружайте в репозиторий.

## API

Базовый адрес: `http://localhost:8080`. Тело запросов и ответов — JSON, ошибки имеют вид `{"error":"сообщение"}`. Ответ 204 не содержит тела.

| Метод | Адрес | Доступ | Успешный код |
|---|---|---|---|
| POST | /register | всем | 201 |
| POST | /login | всем | 200 |
| GET | /posts | всем | 200 |
| POST | /posts | с токеном | 201 |
| GET | /posts/{id} | всем | 200 |
| PUT | /posts/{id} | автору поста | 200 |
| DELETE | /posts/{id} | автору поста | 204 |
| GET | /posts/{id}/comments | всем | 200 |
| POST | /posts/{id}/comments | с токеном | 201 |
| GET | /health | всем | 200 |

Регистрация принимает `username`, `email`, `password`; вход — `email` и `password`. В обоих ответах есть `token`, `expires_at` и `user`. Для защищённых запросов нужен заголовок `Authorization: Bearer <token>`.

У поста есть `id`, `author_id`, `title`, `content`, `created_at`; у комментария — `id`, `post_id`, `author_id`, `text`, `created_at`. Также возвращаются `updated_at` и публичные данные автора: `author` содержит только `id` и `username`, без email.

Списки поддерживают `limit` (1–100, по умолчанию 10) и `offset` (от 0). Формат ответа: `{"posts":[],"total":0,"limit":10,"offset":0}`; для комментариев ключ `posts` заменяется на `comments`.

Валидация: username — 3–50 символов, корректный email, пароль — от 6 символов до 72 байт. Заголовок — до 200 символов, содержимое поста — до 50000, комментарий — до 1000. Пустые значения отклоняются. Username уникален с учётом регистра, email — без учёта регистра.

## Примеры запросов

Команды для Bash или Git Bash:

```bash
curl -i -X POST http://localhost:8080/register -H "Content-Type: application/json" -d '{"username":"student","email":"student@example.com","password":"strongpass123"}'
curl -i -X POST http://localhost:8080/login -H "Content-Type: application/json" -d '{"email":"student@example.com","password":"strongpass123"}'

# Вставьте token из ответа регистрации или входа.
TOKEN="полученный_JWT"
curl -i -X POST http://localhost:8080/posts -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"title":"Первый пост","content":"Текст поста"}'

# Замените 7 на id созданного поста.
POST_ID=7
curl -i "http://localhost:8080/posts?limit=10&offset=0"
curl -i "http://localhost:8080/posts/$POST_ID"
curl -i -X POST "http://localhost:8080/posts/$POST_ID/comments" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"text":"Мой комментарий"}'
curl -i "http://localhost:8080/posts/$POST_ID/comments"
curl -i http://localhost:8080/health
```

Повторная регистрация с занятым email или username возвращает 409. Создание без токена — 401, неверные данные — 400, чужой пост при изменении — 403, отсутствующий ресурс — 404, неподдерживаемый метод — 405.

## Логи и проверки

Запросы выводятся в консоль и записываются в файл. События создания постов и комментариев передаются через канал; горутина записывает их с задержкой около секунды. При штатной остановке очередь дописывается.

Локально файл называется `log.txt`. В Docker посмотреть его можно так:

```powershell
docker compose exec api tail -n 20 /app/logs/log.txt
```

Тесты не требуют базы данных:

```powershell
go test ./...
go vet ./...
```
