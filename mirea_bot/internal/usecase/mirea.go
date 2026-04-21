package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"mirea-qr/internal/config"
	"mirea-qr/internal/entity"
	"mirea-qr/internal/model"
	"mirea-qr/internal/repository"
	"mirea-qr/pkg/crypto"
	"mirea-qr/pkg/customerrors"
	"mirea-qr/pkg/mirea"

	"github.com/go-playground/validator/v10"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type MireaUseCase struct {
	DB        *gorm.DB
	Log       *logrus.Logger
	Validate  *validator.Validate
	Redis     *redis.Client
	Bot       *tgbotapi.BotAPI
	Encryptor *crypto.Encryptor
	Config    *config.Config

	UserRepository              *repository.UserRepository
	LinkUserRepository          *repository.LinkUserRepository
	SubjectAttendanceRepository *repository.SubjectAttendanceRepository
}

type WorkerQR struct {
	config    *config.Config
	users     []entity.User
	mutex     sync.Mutex
	wg        sync.WaitGroup
	redis     *redis.Client
	response  *model.ScanQRResponse
	uuid      string
	subject   string
	bot       *tgbotapi.BotAPI
	encryptor *crypto.Encryptor
}

const (
	STATUS_NOT_AUTHORIZED = iota
	STATUS_NOT_APPROVED
	STATUS_UNKNOWN
	STATUS_ERROR
	STATUS_SUCCESS
	STATUS_NOT_IN_UNIVERSITY
	STATUS_NOT_LESSON
)

func NewMireaUseCase(Cfg *config.Config, db *gorm.DB, logger *logrus.Logger, validate *validator.Validate, userRepository *repository.UserRepository, linkUserRepository *repository.LinkUserRepository, subjectAttendanceRepository *repository.SubjectAttendanceRepository, redis *redis.Client, bot *tgbotapi.BotAPI, encryptor *crypto.Encryptor) *MireaUseCase {
	return &MireaUseCase{
		Config:                      Cfg,
		DB:                          db,
		Log:                         logger,
		Validate:                    validate,
		UserRepository:              userRepository,
		LinkUserRepository:          linkUserRepository,
		SubjectAttendanceRepository: subjectAttendanceRepository,
		Redis:                       redis,
		Bot:                         bot,
		Encryptor:                   encryptor,
	}
}

// createUserWithDecryptedPassword creates a user with decrypted password for API authorization
func (c *MireaUseCase) createUserWithDecryptedPassword(user entity.User) (entity.User, error) {
	// Decrypt password
	decryptedPassword, err := c.Encryptor.Decrypt(user.Password)
	if err != nil {
		c.Log.Errorf("Failed to decrypt password for user %s: %+v", user.Email, err)
		return user, fiber.NewError(500, "Failed to decrypt password")
	}

	// Create user with decrypted password
	userWithDecryptedPassword := user
	userWithDecryptedPassword.Password = decryptedPassword

	return userWithDecryptedPassword, nil
}

func (c *MireaUseCase) GetDisciplines(user entity.User) (*model.DisciplinesResponse, error) {
	tx := c.DB.WithContext(context.Background()).Begin()
	defer tx.Rollback()
	data, err := c.Redis.Get(context.Background(), "disciplines_v2_"+user.ID).Bytes()

	if err == nil {
		var response *model.DisciplinesResponse
		if err := json.Unmarshal(data, &response); err == nil {
			return response, nil
		}
	}

	userWithDecryptedPassword, err := c.createUserWithDecryptedPassword(user)
	if err != nil {
		return nil, err
	}
	userWithDecryptedPassword.Password = "" // Очищаем пароль, чтобы предотвратить автоматическую отправку OTP

	attendance := mirea.NewAttendance(c.Config, userWithDecryptedPassword, c.Redis)
	if err := attendance.Authorization(); err != nil {
		return nil, fiber.NewError(403, "failed authorization")
	}

	rating, err := attendance.GetLearnRatingScore()
	if err != nil {
		return nil, fiber.NewError(500, fmt.Sprintf("failed get disciplines : %+v", err))
	}

	// Fetch potential attendance scores (best-effort, non-fatal if it fails)
	potentialScores := map[string]float64{}
	if potentials, err := attendance.GetIdealAttendancePotentials(); err != nil {
		c.Log.Warnf("[ideal] GetIdealAttendancePotentials failed: %+v", err)
	} else {
		c.Log.Infof("[ideal] potentialScores: %v", potentials)
		potentialScores = potentials
	}

	valueDataRating := map[string]model.DisciplineScoreData{}
	for _, valueData := range rating.GetValueData() {
		for _, block := range valueData.GetBlock() {
			valueDataRating[block.GetUuid()] = model.DisciplineScoreData{
				Title: block.GetTitle(),
				Now:   0,
				Max:   float64(block.GetMax()),
			}
		}
	}

	var disciplinesResponse []model.DisciplineResponse
	for _, discipline := range rating.GetDiscipline() {
		title := discipline.GetTitle().GetValue()
		avg := c.SubjectAttendanceRepository.GetAvgForSubjectByGroup(tx, user.GroupID, title)

		disciplineScoreData := []model.DisciplineScoreData{}
		for _, scoreData := range discipline.GetScoreData() {
			disciplineScoreData = append(disciplineScoreData, model.DisciplineScoreData{
				Title: valueDataRating[scoreData.GetUuid()].Title,
				Now:   scoreData.GetNow(),
				Max:   valueDataRating[scoreData.GetUuid()].Max,
			})
		}

		disciplinesResponse = append(disciplinesResponse, model.DisciplineResponse{
			Title:          title,
			Total:          discipline.GetTotal(),
			AvgGroup:       avg,
			ScoreData:      disciplineScoreData,
			PotentialScore: potentialScores[title],
		})

		if c.SubjectAttendanceRepository.HasForUser(tx, user.ID, user.GroupID, title) == false {
			subjectAttendance := &entity.SubjectAttendance{
				StudentId: user.ID,
				GroupId:   user.GroupID,
				Subject:   title,
				Value:     discipline.GetTotal(),
			}

			if err := c.SubjectAttendanceRepository.Create(tx, subjectAttendance); err != nil {
				c.Log.Warnf("Failed insert subject attendance to database : %+v", err)
			}
		} else {
			// Обновляем баллы студента, если запись уже существует
			if err := c.SubjectAttendanceRepository.UpdateValueForUser(tx, user.ID, user.GroupID, title, discipline.GetTotal()); err != nil {
				c.Log.Warnf("Failed update subject attendance for user : %+v", err)
			}
		}
	}

	response := &model.DisciplinesResponse{
		CountStudents: c.SubjectAttendanceRepository.GetCountStudents(tx, user.GroupID),
		Disciplines:   disciplinesResponse,
	}

	tx.Commit()

	// Очищаем кеш для всех студентов группы, чтобы обновить средний балл группы
	// Получаем всех студентов группы и очищаем их кеш
	var groupUsers []entity.User
	if err := c.DB.Where("group_id = ?", user.GroupID).Find(&groupUsers).Error; err == nil {
		for _, groupUser := range groupUsers {
			c.Redis.Del(context.Background(), "disciplines_"+groupUser.ID)
		}
	}

	redisData, err := json.Marshal(response)
	if err == nil {
		c.Redis.Set(context.Background(), "disciplines_v2_"+user.ID, redisData, time.Hour)
	}

	return response, nil
}

func (c *MireaUseCase) GetLessons(user entity.User, request model.GetLessons) ([]model.LessonResponse, error) {
	tx := c.DB.WithContext(context.Background()).Begin()
	defer tx.Rollback()

	today := time.Now()
	// Условие чтобы не кешировался текущий день, для обновления отметок на паре
	if int32(today.Year()) != request.Year || int32(today.Month()) != request.Month || int32(today.Day()) != request.Day {
		data, err := c.Redis.Get(context.Background(), fmt.Sprintf("lessons_%s_%d%d%d", user.GroupID, request.Year, request.Month, request.Day)).Bytes()

		if err == nil {
			var response []model.LessonResponse
			if err := json.Unmarshal(data, &response); err == nil {
				return response, nil
			}
		}
	}

	userWithDecryptedPassword, err := c.createUserWithDecryptedPassword(user)
	if err != nil {
		return nil, err
	}
	userWithDecryptedPassword.Password = "" // Очищаем пароль, чтобы предотвратить автоматическую отправку OTP

	attendance := mirea.NewAttendance(c.Config, userWithDecryptedPassword, c.Redis)
	if err := attendance.Authorization(); err != nil {
		return nil, fiber.NewError(403, "failed authorization")
	}

	lessons, err := attendance.GetLessons(request.Year, request.Month, request.Day)
	if err != nil {
		if err.Error() == "empty response" {
			// мб воскресенье, мб у школьников каникулы
			return []model.LessonResponse{}, nil
		}

		return nil, fiber.NewError(500, fmt.Sprintf("failed get lessons : %+v", err))
	}

	lessonsResponse := []model.LessonResponse{}
	for _, lesson := range lessons {
		data := lesson.GetLesson()
		teacherData := data.GetTeacher()
		var teacher string
		if teacherData != nil {
			var middleName string
			if teacherData.GetMiddleName() != nil {
				middleName = teacherData.GetMiddleName().GetValue()
			}
			teacher = teacherData.GetFamilyName() + " " + teacherData.GetName() + " " + middleName
		}

		lessonsResponse = append(lessonsResponse, model.LessonResponse{
			Uuid:       data.GetUuid(),
			Auditorium: data.GetRoom().GetNumber(),
			Campus:     data.GetRoom().GetCampus(),
			Title:      data.GetSubject().GetValue(),
			Type:       data.GetType().GetValue(),
			Start:      int32(data.GetStart().GetValue()),
			End:        int32(data.GetEnd().GetValue()),
			Teacher:    teacher,
			Groups:     []string{},
			Attended:   0,
			Total:      0,
			Status:     lesson.GetAttended(),
		})
	}

	// small naser for update group id
	currentUser := attendance.GetCurrentUser()
	c.Log.Printf("Old group id : %s, current group id : %s", user.GroupID, currentUser.GroupID)
	if user.GroupID != currentUser.GroupID || user.Group != currentUser.Group {
		user.GroupID = currentUser.GroupID
		user.Group = currentUser.Group

		err := c.UserRepository.Update(tx, &user)
		if err != nil {
			c.Log.Warnf("Failed update group id : %+v", err)
		}
	}

	tx.Commit()

	redisData, err := json.Marshal(lessonsResponse)
	if err == nil {
		c.Redis.Set(context.Background(), fmt.Sprintf("lessons_%s_%d%d%d", user.GroupID, request.Year, request.Month, request.Day), redisData, time.Hour*24)
	}

	return lessonsResponse, nil
}

func (c *MireaUseCase) Attendance(user entity.User, request model.AttendanceRequest) ([]model.AttendanceStudentResponse, error) {
	tx := c.DB.WithContext(context.Background()).Begin()
	defer tx.Rollback()

	userWithDecryptedPassword, err := c.createUserWithDecryptedPassword(user)
	if err != nil {
		return nil, err
	}
	userWithDecryptedPassword.Password = "" // Очищаем пароль, чтобы предотвратить автоматическую отправку OTP

	attendance := mirea.NewAttendance(c.Config, userWithDecryptedPassword, c.Redis)
	if err := attendance.Authorization(); err != nil {
		return nil, fiber.NewError(403, "failed authorization")
	}

	r, err := attendance.GetAttendanceStudentForLesson(request.LessonUUID)
	if err != nil {
		return nil, fiber.NewError(500, fmt.Sprintf("failed get attendance for lesson %s : %+v", request.LessonUUID, err))
	}

	response := []model.AttendanceStudentResponse{}
	for _, student := range r {
		fullname := strings.Join([]string{student.GetLastName(), student.GetFirstName(), student.GetMiddleName().GetValue()}, " ")

		response = append(response, model.AttendanceStudentResponse{
			Fullname: fullname,
			Status:   student.GetGroup().GetNumber() * -1,
			IsElder:  student.GetHuesos() != -1,
		})
	}

	return response, nil
}

func (c *MireaUseCase) GetDeadlines(user entity.User) ([]model.DeadlineResponse, error) {
	data, err := c.Redis.Get(context.Background(), fmt.Sprintf("deadlines_group_%s", user.GroupID)).Bytes()

	if err == nil {
		var response []model.DeadlineResponse
		if err := json.Unmarshal(data, &response); err == nil {
			return response, nil
		}
	}

	userWithDecryptedPassword, err := c.createUserWithDecryptedPassword(user)
	if err != nil {
		return nil, err
	}
	userWithDecryptedPassword.Password = "" // Очищаем пароль, чтобы предотвратить автоматическую отправку OTP

	online := mirea.NewOnlineEdu(userWithDecryptedPassword, c.Redis)
	if _, err := online.Authorization(); err != nil {
		return nil, fiber.NewError(403, "failed authorization")
	}

	deadlines, err := online.GetDeadlines()
	if err != nil {
		return nil, fiber.NewError(500, fmt.Sprintf("failed get deadlines : %+v", err))
	}

	out := make([]model.DeadlineResponse, 0, len(deadlines))
	for _, d := range deadlines {
		out = append(out, model.DeadlineResponse{
			Title:     d.Title,
			Timestamp: d.Timestamp,
			Subject:   d.Subject,
		})
	}

	redisData, err := json.Marshal(out)
	if err == nil {
		c.Redis.Set(context.Background(), fmt.Sprintf("deadlines_group_%s", user.GroupID), redisData, time.Hour*24)
	}

	return out, nil
}

func (c *MireaUseCase) ScanQR(ctx context.Context, request *model.ScanQRRequest, fromUser *entity.User) (*model.ScanQRResponse, error) {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	worker := WorkerQR{
		config: c.Config,
		users:  []entity.User{},
		mutex:  sync.Mutex{},
		wg:     sync.WaitGroup{},
		redis:  c.Redis,
		response: &model.ScanQRResponse{
			Students: map[string]uint{},
		},
		bot:       c.Bot,
		encryptor: c.Encryptor,
	}

	// 🚨 БЕЗОПАСНЫЙ ПАРСИНГ ССЫЛКИ 🚨
	parsedURL, err := url.Parse(request.URL)
	if err != nil {
		c.Log.Warnf("Failed to parse URL: %s", request.URL)
		return worker.response, fiber.NewError(400, "Неверный формат ссылки")
	}

	// Извлекаем токен из query-параметров.
	// Пытаемся найти параметры "token", "code", "id", или "uuid" (МИРЭА могли изменить название)
	token := parsedURL.Query().Get("token")
	if token == "" {
		token = parsedURL.Query().Get("code")
	}
	if token == "" {
		token = parsedURL.Query().Get("id")
	}
	if token == "" {
		token = parsedURL.Query().Get("uuid")
	}

	// Если токен так и не найден, или ссылка ведет не на МИРЭА
	if token == "" || (!strings.Contains(parsedURL.Host, "mirea.ru")) {
		return worker.response, fiber.NewError(400, "QR код не относится к МИРЭА или не содержит токена")
	}

	// Очищаем токен от лишнего мусора (на всякий случай)
	token = strings.Split(token, "&")[0]
	token = strings.Split(token, "#")[0]
	worker.uuid = token

	var links []*entity.LinkUser
	if err := c.LinkUserRepository.GetConnectedByUser(tx, &links, request.FromUserId); err != nil {
		//return worker.response, fiber.NewError(500, "Не удалось получить студентов")
	}

	for _, link := range links {
		if !link.Enabled {
			continue
		}
		worker.users = append(worker.users, link.ToUser)
	}

	worker.users = append(worker.users, *fromUser)
	worker.wg.Add(len(worker.users))
	worker.Start()
	worker.wg.Wait()

	return worker.response, nil
}

func (w *WorkerQR) Start() {
	for i, user := range w.users {
		// Добавляем искусственную задержку, чтобы размазать запросы 
		// и избежать блокировки 403 Forbidden от DDoS-Guard МИРЭА
		delay := time.Duration(i*400) * time.Millisecond

		go func(u entity.User, d time.Duration) {
			defer w.wg.Done()
			time.Sleep(d)

			telegramAlreadyNotificationErr := w.redis.Get(context.Background(), fmt.Sprintf("telegram_notification_%d", u.TelegramId)).Err()
			telegramSendNotification := errors.Is(telegramAlreadyNotificationErr, redis.Nil)

			familyName := strings.Split(u.Fullname, " ")[0]

			attendanceStatusKey := fmt.Sprintf("attendance_status_%s", u.ID)
			cachedStatus := w.redis.Get(context.Background(), attendanceStatusKey)
			if cachedStatus.Err() == nil {
				w.mutex.Lock()
				s, _ := cachedStatus.Int()
				w.response.Students[familyName] = uint(s)
				w.mutex.Unlock()
				return
			}

			_, err := w.encryptor.Decrypt(u.Password)
			if err != nil {
				w.mutex.Lock()
				w.response.Students[familyName] = STATUS_ERROR
				w.mutex.Unlock()
				return
			}

			userWithDecryptedPassword := u
			userWithDecryptedPassword.Password = "" 

			attendance := mirea.NewAttendance(w.config, userWithDecryptedPassword, w.redis)
			attendance.SetUseCase(true)
			if err := attendance.Authorization(); err != nil {
				w.mutex.Lock()
				var authErr *customerrors.AuthError
				if errors.As(err, &authErr) && (authErr.Type == "network_error" || authErr.Type == "site_unavailable" || authErr.Type == "internal_error") {
					w.response.Students[familyName] = STATUS_ERROR 
				} else {
					w.response.Students[familyName] = STATUS_NOT_AUTHORIZED
				}
				w.mutex.Unlock()
				fmt.Printf("%s - failed authorization for scan: %v\n", familyName, err)
				return
			}
			
			msg, err := attendance.SelfApproveAttendance(w.uuid)
			
			w.mutex.Lock()
			if err != nil {
				// ТЕПЕРЬ МЫ ВЫВОДИМ ОШИБКУ, ЕСЛИ ОТМЕТКА УПАЛА
				fmt.Printf("[SCAN ERROR] %s - gRPC request failed during SelfApproveAttendance: %v\n", familyName, err)
				w.response.Students[familyName] = STATUS_UNKNOWN
			} else {
				if msg != nil {
					if msg.GetSuccess() != nil {
						w.response.Students[familyName] = STATUS_SUCCESS
						w.redis.Set(context.Background(), attendanceStatusKey, STATUS_SUCCESS, time.Minute*15)
						w.response.Subject = msg.GetSuccess().GetSubject()
					} else {
						if msg.GetFailed() != nil {
							errStatus := msg.GetFailed().GetError()
							switch errStatus {
							case 1:
								w.response.Students[familyName] = STATUS_NOT_APPROVED
								break
							case 3:
								if telegramSendNotification {
									notificationMessage := "🏫 Вас пытались отметить на паре, к которой вы не закреплены"
									w.bot.Send(tgbotapi.NewMessage(u.TelegramId, notificationMessage))
								}
								w.redis.Set(context.Background(), attendanceStatusKey, STATUS_NOT_LESSON, time.Minute*1)
								w.response.Students[familyName] = STATUS_NOT_LESSON
								break
							case 5:
								w.redis.Set(context.Background(), "check_student_univ", true, time.Hour*48)
								w.response.Students[familyName] = STATUS_NOT_IN_UNIVERSITY
								if telegramSendNotification {
									notificationMessage := "🏫 Не удалось отметить вас на паре, вы не находитесь в университете."
									w.bot.Send(tgbotapi.NewMessage(u.TelegramId, notificationMessage))
								}
								w.redis.Set(context.Background(), attendanceStatusKey, STATUS_NOT_IN_UNIVERSITY, time.Minute*2)
								break
							default:
								w.response.Students[familyName] = STATUS_NOT_APPROVED
								break
							}

						} else {
							w.response.Students[familyName] = STATUS_NOT_APPROVED
						}
					}
				} else {
					w.response.Students[familyName] = STATUS_ERROR
				}
			}

			w.redis.Set(context.Background(), fmt.Sprintf("telegram_notification_%d", u.TelegramId), true, time.Minute*10)
			w.mutex.Unlock()
		}(user, delay)
	}
}
