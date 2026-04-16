Документация бэкенда MIREA QR Tools
1. Общая архитектура
Проект написан на Go с использованием фреймворка Fiber. Архитектура — Clean/Onion (слоистая):

📦 cmd/web/               # Точка входа
📦 internal/             # Внутренний код
 ┣ 📂 app/               # Инициализация и конфиги
 ┣ 📂 entity/            # Модели БД (GORM)
 ┣ 📂 repository/        # Работа с БД
 ┣ 📂 usecase/           # Бизнес-логика (ядро)
 ┣ 📂 handler/           # HTTP-контроллеры
 ┃ ┣ 📂 middleware/      # Прослойки (аутентификация)
 ┃ ┗ 📂 route/           # Роутинг
 ┗ 📂 model/             # DTO (JSON-структуры)
📦 pkg/                   # Внешние интеграции (mirea, crypto)
Связь между слоями:

HTTP → Middleware (auth) → Handler → UseCase → Repository → БД
                                       ↓
                                 Парсинг Pulse.MIREA
                                 
2. Детальный разбор по папкам
2.1 cmd/web/main.go — точка входа
   
func main() {
    cfg := app.NewConfig()           // Читает .env
    logger := app.NewLogger(cfg)      // Логи (debug/prod)
    db := app.NewDatabase(cfg, logger) // PostgreSQL + авто-миграция
    fiber := app.NewFiber(cfg)        // HTTP-сервер
    redis := app.NewRedis(cfg)        // Кэш
    bot := app.NewBot(cfg)            // Telegram-бот
    encryptor := crypto.NewEncryptor() // Шифрование паролей
    
    app.Bootstrap(...)                 // Сборка всех слоёв
    fiber.Listen(":8888")              // Запуск сервера
}
2.2 internal/app/ — инициализация
Файл	Назначение
config.go	Читает .env (БД, Redis, токены бота, ключи шифрования)
database.go	Подключение к PostgreSQL, авто-миграция всех entity
redis.go	Подключение к Redis (кэш, сессии)
bot.go	Telegram-бот: слушает /start, отправляет WebApp кнопку
fiber.go	Настройка HTTP-сервера, обработка ошибок
logrus.go	Логирование (debug/prod)
validator.go	Валидация email на @edu.mirea.ru
bootstrap.go	Сборка проекта: создаёт repository → usecase → handler → middleware → route
2.3 internal/entity/ — модели БД (GORM)
Модель	Хранит
User	Telegram ID, email, пароль (зашифрован), группа, TOTP secret
LinkUser	Связи "студент A подписан на студента B" (для совместного сканирования)
SubjectAttendance	Кэш баллов с Pulse (student_id, group_id, subject, value)
QrScan	Статистика сканирований (для админа)
Teacher / TeacherReview	Отзывы на преподавателей
2.4 internal/repository/ — работа с БД
Каждая модель имеет свой репозиторий.
Ключевой для Pulse: subject_attendance.go

Метод	Что делает
HasForUser	Проверяет, есть ли баллы студента по предмету в БД
GetAvgForSubjectByGroup	Средний балл по группе
UpdateValueForUser	Обновляет баллы после парсинга
GetCountStudents	Количество студентов в группе
2.5 internal/usecase/ — БИЗНЕС-ЛОГИКА (самое важное)
mirea.go — работа с Pulse.MIREA
Это сердце проекта. Все методы используют pkg/mirea для парсинга.

Ключевые функции:

Метод	Что делает	Кэш
GetDisciplines	Получает баллы по всем предметам	Redis + БД (SubjectAttendance)
GetLessons	Расписание на день	Redis (кроме текущего дня)
Attendance	Кто был на паре	Нет
GetDeadlines	Дедлайны	Redis (по группе)
ScanQR	Сканирование QR-кода	Worker с горутинами
Алгоритм GetDisciplines:

Проверить Redis (ключ disciplines_{userID}) — если есть, отдать

Расшифровать пароль пользователя (Encryptor.Decrypt)

Авторизоваться на Pulse через pkg/mirea

Спарсить баллы

Для каждого предмета:

Проверить, есть ли запись в SubjectAttendance

Если нет — создать

Если есть — обновить баллы (UpdateValueForUser)

Посчитать средний балл по группе (из БД)

Сохранить в Redis на 1 час

Очистить кэш у всех студентов группы (чтобы обновились средние)

Алгоритм ScanQR (ключевая функция):

Распарсить URL QR-кода, извлечь token

Получить список привязанных студентов (LinkUser) + самого пользователя

Запустить горутину на каждого студента (WorkerQR.Start)

Каждая горутина:

Проверяет Redis (отметка уже была?)

Авторизуется на Pulse

Вызывает SelfApproveAttendance с токеном

Анализирует ответ:

STATUS_SUCCESS — отмечен ✅

STATUS_NOT_IN_UNIVERSITY — не в вузе

STATUS_NOT_LESSON — не закреплён за парой

и т.д.

Сохраняет результат в общую map

Кэширует статус в Redis (чтобы не дёргать повторно)

Возвращает map[student]status на фронтенд

user.go — управление пользователями
Метод	Что делает
Create	Регистрация: проверяет логин/пароль на Pulse, шифрует пароль, сохраняет
GetUniversityStatus	Статус в вузе (вошёл/вышел) через GetHumanAcsEvents
UpdateTotpSecret	Сохраняет TOTP секрет, проверяет авторизацию
ConnectStudent	Привязывает другого студента (для совместного сканирования)
review.go — отзывы на преподавателей
Работа с таблицами Teacher и TeacherReview.
Простая CRUD-логика.

admin.go — статистика
Собирает метрики: сколько пользователей, сканирований, групп.

2.6 internal/handler/ — HTTP-контроллеры
Просто принимают запрос, вызывают usecase, возвращают JSON.

Контроллер	Методы
mirea.go	Disciplines, GetLessons, Attendance, ScanQR, FindStudent
user.go	Register, Me, ConnectStudent, ChangePassword, GetUniversityStatus
review.go	ListTeachers, CreateReview, DeleteReview
admin.go	GetStats
2.7 internal/handler/middleware/ — аутентификация
Middleware	Что делает
telegram.go	Валидирует данные из Telegram WebApp (проверяет подпись)
register.go	Проверяет, зарегистрирован ли пользователь в БД
admin.go	Проверяет, является ли пользователем админом
debug.go	Логирует запросы
Как проходит запрос:

Фронтенд отправляет JSON с полем miniAppUser (строка от Telegram)

telegram.go проверяет подпись, достаёт telegram_id

register.go ищет пользователя в БД, кладёт в ctx.Locals("user")

Контроллер получает user := middleware.GetUser(ctx)

2.8 internal/handler/route/route.go — все эндпоинты
Полный список API:

Публичные
text
POST /v1/sign-up
Защищённые (требуется регистрация)
text
# Пользователь
POST /v1/me
POST /v1/change-password
POST /v1/university-status
POST /v1/delete-user

# Связи между студентами
POST /v1/connect-student
POST /v1/list-connected-student
POST /v1/disconnect-student

# MIREA Pulse
POST /v1/disciplines        # Баллы
POST /v1/lessons            # Расписание
POST /v1/attendance         # Посещаемость
POST /v1/deadlines          # Дедлайны
POST /v1/scan-qr            # Сканирование QR

# Отзывы
POST /v1/reviews/teachers
POST /v1/reviews/create
Админские
text
POST /v1/admin/stats
2.9 internal/model/ — структуры JSON
Содержит DTO (Data Transfer Objects) — то, что приходит с фронтенда и уходит обратно.

Ключевые:

DisciplinesResponse — баллы

LessonResponse — расписание

ScanQRResponse — результат сканирования

UserResponse — профиль

2.10 pkg/ — внешние интеграции
Пакет	Назначение
mirea/	Клиент для Pulse.MIREA — авторизация, парсинг HTML, вызов API
crypto/	Шифрование/дешифрование паролей пользователей
jwt/	Работа с JWT (не используется, т.к. аутентификация через Telegram)
3. Как работает парсинг Pulse.MIREA (детально)
Авторизация
go
attendance := mirea.NewAttendance(user, redis)
if err := attendance.Authorization(); err != nil {
    // Ошибка: неверный логин/пароль, TOTP, сайт недоступен
}
Получение данных
GetLearnRatingScore() — баллы

GetLessons(year, month, day) — расписание

GetAttendanceStudentForLesson(uuid) — кто был на паре

SelfApproveAttendance(token) — отметка по QR

GetHumanAcsEvents(start, end) — проходы в вуз

Кэширование
Redis: баллы (1 час), расписание (24 часа), дедлайны (24 часа), статусы QR (несколько минут)

БД (SubjectAttendance): долгосрочное хранение баллов (для средних по группе)

4. Связь с фронтендом
Фронтенд открывается внутри Telegram WebApp (по ссылке из бота)

Каждый запрос к бэкенду содержит miniAppUser — строку от Telegram

Бэкенд валидирует её через telegram.go

Ответы всегда в формате:

json
{
  "data": { ... },  // основные данные
  "errors": ""      // ошибка, если есть
}
5. Безопасность
Пароли пользователей хранятся в зашифрованном виде (crypto.Encryptor)

Telegram-данные валидируются через HMAC

TOTP секреты хранятся в открытом виде (нужны для авторизации)

Доступ к админке только по telegram_id из конфига

6. Ключевые бизнес-процессы
Регистрация
Пользователь нажимает "Start" в боте

Открывается WebApp, вводит email/пароль

Бэкенд проверяет их на Pulse, шифрует пароль, сохраняет пользователя

Просмотр баллов
Фронтенд шлёт POST /v1/disciplines

Бэкенд берёт данные из Redis/БД или парсит Pulse

Возвращает JSON с баллами + средние по группе

Сканирование QR
Студент A сканирует QR-код на паре

Фронтенд шлёт POST /v1/scan-qr с URL

Бэкенд получает всех привязанных к A студентов

Запускает горутины на каждого

Каждый студент отмечается (или получает статус ошибки)

Результат возвращается на фронтенд

Если кто-то не в вузе — приходит уведомление в Telegram

7. Технический стек
Компонент	Технология
Язык	Go 1.23
Веб-фреймворк	Fiber v3
БД	PostgreSQL + GORM
Кэш	Redis
Логи	Logrus
Валидация	go-playground/validator
Telegram	telegram-bot-api
Парсинг	Кастомный pkg/mirea (на основе protobuf)
Шифрование	AES (через crypto пакет)
