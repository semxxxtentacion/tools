package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mirea-qr/internal/config"
	entity "mirea-qr/internal/entity"
	"mirea-qr/internal/model"
	"mirea-qr/internal/model/converter"
	"mirea-qr/internal/repository"
	"mirea-qr/pkg/crypto"
	"mirea-qr/pkg/customerrors"
	"mirea-qr/pkg/mirea"
	"sort"
	"strconv"
	"strings"
	"time"

	browser "github.com/EDDYCJY/fake-useragent"
	"github.com/go-playground/validator/v10"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type UserUseCase struct {
	Config             *config.Config
	DB                 *gorm.DB
	Log                *logrus.Logger
	Validate           *validator.Validate
	UserRepository     *repository.UserRepository
	LinkUserRepository *repository.LinkUserRepository
	Redis              *redis.Client
	Bot                *tgbotapi.BotAPI
	Encryptor          *crypto.Encryptor
}

func NewUserUseCase(cfg *config.Config, db *gorm.DB, logger *logrus.Logger, validate *validator.Validate, userRepository *repository.UserRepository, linkUserRepository *repository.LinkUserRepository, redis *redis.Client, bot *tgbotapi.BotAPI, encryptor *crypto.Encryptor) *UserUseCase {
	return &UserUseCase{
		Config:             cfg,
		DB:                 db,
		Log:                logger,
		Validate:           validate,
		UserRepository:     userRepository,
		LinkUserRepository: linkUserRepository,
		Redis:              redis,
		Bot:                bot,
		Encryptor:          encryptor,
	}
}

// createUserWithDecryptedPassword creates a user with decrypted password for API authorization
func (c *UserUseCase) createUserWithDecryptedPassword(user entity.User) (entity.User, error) {
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

const otpPendingTTL = 20 * time.Minute
const otpPendingKeyPrefix = "otp_pending:"

const inviteTTL = 48 * time.Hour
const inviteKeyPrefix = "invite:"

func otpPendingKey(telegramHash string, telegramId int64) string {
	if telegramHash != "" {
		return otpPendingKeyPrefix + telegramHash
	}
	return otpPendingKeyPrefix + "id:" + strconv.FormatInt(telegramId, 10)
}

func (c *UserUseCase) Create(ctx context.Context, request *model.RegisterUserRequest) (*model.UserResponse, *model.OtpRequiredResponse, error) {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	err := c.Validate.Struct(request)
	if err != nil {
		c.Log.Warnf("Invalid request body : %+v", err)
		return nil, nil, fiber.ErrBadRequest
	}

	// Проверяем, не зарегистрирован ли уже пользователь с таким Telegram ID
	user := new(entity.User)
	if err := c.UserRepository.FindByEmail(tx, user, request.Email); err == nil {
		c.Log.Warnf("User already exists with email %s : %+v", request.Email, err)
		//return nil, fiber.NewError(409, "В боте можно зарегистрироваться только с одного аккаунта Telegram")
	}

	attendance := mirea.NewAttendance(c.Config, entity.User{
		Email:    request.Email,
		Password: request.Password,
	}, c.Redis)

	encryptedPassword, err := c.Encryptor.Encrypt(request.Password)
	if err != nil {
		c.Log.Errorf("Failed to encrypt password: %+v", err)
		return nil, nil, fiber.ErrInternalServerError
	}
	attendance.SetUseCase(false)
	if err := attendance.Authorization(); err != nil {
		c.Log.Warnf("Authorization error : %+v", err)

		var authErr *customerrors.AuthError
		if errors.As(err, &authErr) {
			switch authErr.Type {
			case "invalid_credentials":
				return nil, nil, fiber.NewError(403, "Неверный логин или пароль от MIREA")
			case "network_error":
				return nil, nil, fiber.NewError(503, "Сайт MIREA не отвечает")
			case "site_unavailable":
				return nil, nil, fiber.NewError(503, "Сайт MIREA недоступен")
			case "totp_secret_required":
				return nil, nil, fiber.NewError(400, authErr.Message)
			case "otp_is_required":
				loginActionURL := authErr.GetLoginActionUrl()
				if loginActionURL == "" {
					return nil, nil, fiber.NewError(500, "Ошибка: ссылка авторизации не получена")
				}

				otpType := authErr.GetOtpType()
				key := otpPendingKey(request.TelegramHash, request.TelegramId)
				pending := model.OtpPendingData{
					TelegramId:       request.TelegramId,
					TelegramUsername: request.TelegramUsername,
					Email:            request.Email,
					Password:         encryptedPassword,
					LoginActionURL:   loginActionURL,
					OtpType:          otpType,
				}
				if err := c.savePendingOtp(ctx, key, &pending); err != nil {
					c.Log.Errorf("Failed to save otp pending to Redis: %+v", err)
					return nil, nil, fiber.ErrInternalServerError
				}
				hashForFront := request.TelegramHash
				if hashForFront == "" {
					hashForFront = strconv.FormatInt(request.TelegramId, 10)
				}
				return nil, &model.OtpRequiredResponse{OtpRequired: true, TelegramHash: hashForFront, OtpType: otpType}, nil
			default:
				return nil, nil, fiber.NewError(500, "Ошибка авторизации в системе MIREA")
			}
		}
		return nil, nil, fiber.NewError(500, "Ошибка авторизации в системе MIREA")
	}

	student, err := attendance.GetMeInfo()
	if err != nil {
		c.Log.Errorf("Failed get me info : %+v", err)
		return nil, nil, fiber.ErrInternalServerError
	}

	group, err := attendance.GetAvailableGroup()
	if err != nil {
		c.Log.Errorf("Failed get group : %+v", err)
		return nil, nil, fiber.ErrInternalServerError
	}

	fullname := strings.Join([]string{student.GetFullname(), student.GetName(), student.GetMiddlename().GetValue()}, " ")
	if user.ID != "" {
		user.TelegramId = request.TelegramId
		user.TelegramUsername = request.TelegramUsername
		user.Group = group.Title
		user.GroupID = group.UUID
		user.Password = encryptedPassword
		if err := c.UserRepository.Update(tx, user); err != nil {
			c.Log.Warnf("Failed update user to database : %+v", err)
			return nil, nil, fiber.NewError(500, err.Error())
		}
	} else {
		user = &entity.User{
			ID:               student.Id,
			TelegramId:       request.TelegramId,
			TelegramUsername: request.TelegramUsername,
			Email:            strings.ToLower(request.Email),
			Password:         encryptedPassword,
			Fullname:         fullname,
			Group:            group.Title,
			GroupID:          group.UUID,
			UserAgent:        browser.Mobile(),
		}

		if err := c.UserRepository.Create(tx, user); err != nil {
			c.Log.Warnf("Failed create user to database : %+v", err)
			return nil, nil, fiber.NewError(500, err.Error())
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Warnf("Failed commit transaction : %+v", err)
		return nil, nil, fiber.ErrInternalServerError
	}

	return converter.UserToResponse(user), nil, nil
}

func (c *UserUseCase) savePendingOtp(ctx context.Context, key string, pending *model.OtpPendingData) error {
	data, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	return c.Redis.Set(ctx, key, data, otpPendingTTL).Err()
}

// SubmitOtp завершает регистрацию после ввода OTP: находит данные в Redis, отправляет код по ссылке, создаёт пользователя.
func (c *UserUseCase) SubmitOtp(ctx context.Context, request *model.SubmitOtpRequest) (*model.UserResponse, error) {
	if request.TelegramHash == "" {
		return nil, fiber.NewError(400, "telegram_hash обязателен")
	}
	var key string
	allDigits := true
	for _, r := range request.TelegramHash {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		key = otpPendingKeyPrefix + "id:" + request.TelegramHash
	} else {
		key = otpPendingKeyPrefix + request.TelegramHash
	}
	data, err := c.Redis.Get(ctx, key).Bytes()
	if err != nil {
		c.Log.Warnf("Otp pending not found or expired: %+v", err)
		return nil, fiber.NewError(404, "Сессия истекла или не найдена. Повторите вход.")
	}
	var pending model.OtpPendingData
	if err := json.Unmarshal(data, &pending); err != nil {
		c.Log.Warnf("Invalid otp pending data: %+v", err)
		return nil, fiber.ErrInternalServerError
	}

	attendance := mirea.NewAttendance(c.Config, entity.User{
		Email: pending.Email,
	}, c.Redis)
	attendance.SetUseCase(false)
	otpType := pending.OtpType
	if otpType == "" {
		otpType = "email"
	}

	if err := attendance.HandleTwoFactorAuth(pending.LoginActionURL, request.OtpCode, otpType); err != nil {
		var authErr *customerrors.AuthError
		if errors.As(err, &authErr) && authErr.Type == "otp_code_is_wrong" {
			loginActionURL := authErr.GetLoginActionUrl()
			otpTypeFromErr := authErr.GetOtpType()
			if loginActionURL != "" {
				pending.LoginActionURL = loginActionURL
				pending.OtpType = otpTypeFromErr
				_ = c.savePendingOtp(ctx, key, &pending)
			}
			return nil, fiber.NewError(400, "Неверный код OTP")
		}
		c.Log.Warnf("HandleTwoFactorAuth error: %+v", err)
		return nil, fiber.NewError(400, err.Error())
	}

	student, err := attendance.GetMeInfo()
	if err != nil {
		c.Log.Errorf("Failed get me info after OTP: %+v", err)
		return nil, fiber.ErrInternalServerError
	}
	group, err := attendance.GetAvailableGroup()
	if err != nil {
		c.Log.Errorf("Failed get group after OTP: %+v", err)
		return nil, fiber.ErrInternalServerError
	}

	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	fullname := strings.Join([]string{student.GetFullname(), student.GetName(), student.GetMiddlename().GetValue()}, " ")

	existingUser := new(entity.User)
	if err := c.UserRepository.FindByEmail(tx, existingUser, pending.Email); err == nil {
		existingUser.TelegramId = pending.TelegramId
		existingUser.TelegramUsername = pending.TelegramUsername
		existingUser.Group = group.Title
		existingUser.GroupID = group.UUID
		if err := c.UserRepository.Update(tx, existingUser); err != nil {
			c.Log.Warnf("Failed update user: %+v", err)
			return nil, fiber.ErrInternalServerError
		}
		if err := tx.Commit().Error; err != nil {
			return nil, fiber.ErrInternalServerError
		}
		_ = c.Redis.Del(ctx, key)
		return converter.UserToResponse(existingUser), nil
	}

	newUser := &entity.User{
		ID:               student.Id,
		TelegramId:       pending.TelegramId,
		TelegramUsername: pending.TelegramUsername,
		Email:            strings.ToLower(pending.Email),
		Password:         pending.Password,
		Fullname:         fullname,
		Group:            group.Title,
		GroupID:          group.UUID,
		UserAgent:        browser.Mobile(),
	}
	if err := c.UserRepository.Create(tx, newUser); err != nil {
		c.Log.Warnf("Failed create user: %+v", err)
		return nil, fiber.ErrInternalServerError
	}
	if err := tx.Commit().Error; err != nil {
		return nil, fiber.ErrInternalServerError
	}
	_ = c.Redis.Del(ctx, key)
	return converter.UserToResponse(newUser), nil
}

func (c *UserUseCase) UpdateDataForUser(user *entity.User) error {
	tx := c.DB.WithContext(context.Background()).Begin()
	defer tx.Rollback()

	userWithDecryptedPassword, err := c.createUserWithDecryptedPassword(*user)
	if err != nil {
		return errors.New("failed to decrypt password")
	}
	userWithDecryptedPassword.Password = "" // Предотвращаем спам кодами

	attendance := mirea.NewAttendance(c.Config, userWithDecryptedPassword, c.Redis)
	if err := attendance.Authorization(); err != nil {
		c.Log.Warnf("Wrong login or password from mirea : %+v", err)
		return errors.New("wrong login or password")
	}

	student, err := attendance.GetMeInfo()
	if err != nil {
		return errors.New("failed get me info")
	}

	group, err := attendance.GetAvailableGroup()
	if err != nil {
		return errors.New("failed get group")
	}

	var middlename string
	if student.GetMiddlename() != nil {
		middlename = student.GetMiddlename().GetValue()
	}
	user.Fullname = strings.Join([]string{student.GetFullname(), student.GetName(), middlename}, " ")
	user.Group = group.Title
	user.GroupID = group.UUID

	if err := c.UserRepository.Update(tx, user); err != nil {
		return errors.New("failed update user")
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Warnf("Failed commit transaction : %+v", err)
		return errors.New("failed commit")
	}

	return nil
}

func (c *UserUseCase) ConnectStudent(ctx context.Context, request *model.ConnectStudentRequest) error {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	err := c.Validate.Struct(request)
	if err != nil {
		c.Log.Warnf("Invalid request body : %+v", err)
		return fiber.ErrBadRequest
	}

	var fromUser entity.User
	if err := c.UserRepository.FindByTelegramID(tx, &fromUser, request.TelegramId); err != nil {
		return fiber.ErrNotFound
	}

	// Determine lookup input: prefer Input field, fall back to Email
	input := request.Input
	if input == "" {
		input = request.Email
	}
	if input == "" {
		return fiber.NewError(400, "Укажите email или Telegram username")
	}

	var toUser entity.User
	if strings.HasPrefix(input, "@") {
		username := strings.TrimPrefix(input, "@")
		if err := c.UserRepository.FindByTelegramUsername(tx, &toUser, username); err != nil {
			return fiber.NewError(404, "Пользователь с таким Telegram username не найден")
		}
	} else {
		if err := c.UserRepository.FindByEmail(tx, &toUser, input); err != nil {
			return fiber.ErrNotFound
		}
	}

	if fromUser.ID == toUser.ID {
		return fiber.NewError(400, "Вы не можете подключить самого себя")
	}

	var linkUserAlready entity.LinkUser
	if err := c.LinkUserRepository.FindLinkUser(tx, &linkUserAlready, fromUser.ID, toUser.ID); err == nil && linkUserAlready.FromUserID != "" {
		c.Log.Warnf("Link user already exists : %+v", linkUserAlready)
		return fiber.NewError(400, "Запрос уже отправлен или пользователь уже подключен")
	}

	linkUser := entity.LinkUser{
		FromUserID: fromUser.ID,
		ToUserID:   toUser.ID,
		Enabled:    true,
		Status:     "pending",
	}
	if err := c.LinkUserRepository.Create(tx, &linkUser); err != nil {
		return fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Warnf("Failed commit transaction : %+v", err)
		return fiber.ErrInternalServerError
	}

	if c.Bot != nil {
		c.Bot.Send(tgbotapi.NewMessage(toUser.TelegramId, fmt.Sprintf("%s хочет добавить вас в список отслеживания посещаемости", fromUser.Fullname)))
	}

	return nil
}

func (c *UserUseCase) AcceptConnection(ctx context.Context, fromUserID string, toUser *entity.User) error {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	var pending entity.LinkUser
	if err := c.LinkUserRepository.FindLinkUser(tx, &pending, fromUserID, toUser.ID); err != nil || pending.FromUserID == "" {
		return fiber.NewError(404, "Запрос на подключение не найден")
	}

	if err := c.LinkUserRepository.UpdateLinkUserStatus(tx, fromUserID, toUser.ID, "accepted"); err != nil {
		return fiber.ErrInternalServerError
	}

	var reverseExisting entity.LinkUser
	if err := c.LinkUserRepository.FindLinkUser(tx, &reverseExisting, toUser.ID, fromUserID); err != nil || reverseExisting.FromUserID == "" {
		reverseLink := entity.LinkUser{
			FromUserID: toUser.ID,
			ToUserID:   fromUserID,
			Enabled:    true,
			Status:     "accepted",
		}
		if err := c.LinkUserRepository.Create(tx, &reverseLink); err != nil {
			return fiber.ErrInternalServerError
		}
	} else {
		if err := c.LinkUserRepository.UpdateLinkUserStatus(tx, toUser.ID, fromUserID, "accepted"); err != nil {
			return fiber.ErrInternalServerError
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fiber.ErrInternalServerError
	}

	var fromUser entity.User
	if err := c.UserRepository.FindById(c.DB, &fromUser, fromUserID); err == nil && c.Bot != nil {
		c.Bot.Send(tgbotapi.NewMessage(fromUser.TelegramId, fmt.Sprintf("%s принял(а) ваш запрос на подключение", toUser.Fullname)))
	}

	return nil
}

func (c *UserUseCase) DeclineConnection(ctx context.Context, fromUserID string, toUser *entity.User) error {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := c.LinkUserRepository.DeleteLinkUser(tx, fromUserID, toUser.ID); err != nil {
		return fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		return fiber.ErrInternalServerError
	}

	return nil
}

func (c *UserUseCase) GetPendingConnections(ctx context.Context, user *entity.User) ([]model.ConnectedStudentResponse, error) {
	var links []*entity.LinkUser
	if err := c.LinkUserRepository.FindPendingByToUser(c.DB.WithContext(ctx), &links, user.ID); err != nil {
		return nil, fiber.ErrInternalServerError
	}

	response := make([]model.ConnectedStudentResponse, 0, len(links))
	for _, link := range links {
		response = append(response, converter.LinkUserToResponseReverse(link))
	}
	return response, nil
}

func (c *UserUseCase) ChangeEnabledConnectedStudent(ctx context.Context, request *model.ConnectStudentRequest) error {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	err := c.Validate.Struct(request)
	if err != nil {
		c.Log.Warnf("Invalid request body : %+v", err)
		return fiber.ErrBadRequest
	}

	var fromUser entity.User
	if err := c.UserRepository.FindByTelegramID(tx, &fromUser, request.TelegramId); err != nil {
		return fiber.ErrNotFound
	}

	var toUser entity.User
	if err := c.UserRepository.FindByEmail(tx, &toUser, request.Email); err != nil {
		return fiber.ErrNotFound
	}

	var linkUser entity.LinkUser
	if err := c.LinkUserRepository.FindLinkUser(tx, &linkUser, fromUser.ID, toUser.ID); err != nil {
		return fiber.ErrNotFound
	}

	c.Log.Warnf("Link user : %+v", linkUser)
	linkUser.Enabled = !linkUser.Enabled

	if err := c.LinkUserRepository.UpdateLinkUser(tx, &linkUser); err != nil {
		c.Log.Warnf("Failed update link user : %+v", err)
		return fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Warnf("Failed commit transaction : %+v", err)
		return fiber.ErrInternalServerError
	}

	return nil
}

func (c *UserUseCase) DisconnectStudent(ctx context.Context, request *model.ConnectStudentRequest) error {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	err := c.Validate.Struct(request)
	if err != nil {
		c.Log.Warnf("Invalid request body : %+v", err)
		return fiber.ErrBadRequest
	}

	var fromUser entity.User
	if err := c.UserRepository.FindByTelegramID(tx, &fromUser, request.TelegramId); err != nil {
		return fiber.ErrNotFound
	}

	var toUser entity.User
	if err := c.UserRepository.FindByEmail(tx, &toUser, request.Email); err != nil {
		return fiber.ErrNotFound
	}

	if err := c.LinkUserRepository.DeleteLinkUser(tx, fromUser.ID, toUser.ID); err != nil {
		c.Log.Warnf("Failed to delete link user : %+v", err)
		return fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Warnf("Failed commit transaction : %+v", err)
		return fiber.ErrInternalServerError
	}

	return nil
}

func (c *UserUseCase) DisconnectFromUser(ctx context.Context, request *model.ConnectStudentRequest) error {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	err := c.Validate.Struct(request)
	if err != nil {
		c.Log.Warnf("Invalid request body : %+v", err)
		return fiber.ErrBadRequest
	}

	var toUser entity.User
	if err := c.UserRepository.FindByTelegramID(tx, &toUser, request.TelegramId); err != nil {
		return fiber.ErrNotFound
	}

	var fromUser entity.User
	if err := c.UserRepository.FindByEmail(tx, &fromUser, request.Email); err != nil {
		return fiber.ErrNotFound
	}

	if err := c.LinkUserRepository.DeleteLinkUser(tx, fromUser.ID, toUser.ID); err != nil {
		c.Log.Warnf("Failed to delete link user : %+v", err)
		return fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Warnf("Failed commit transaction : %+v", err)
		return fiber.ErrInternalServerError
	}

	return nil
}

func (c *UserUseCase) ChangePassword(ctx context.Context, request *model.ChangePasswordRequest) error {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	err := c.Validate.Struct(request)
	if err != nil {
		c.Log.Warnf("Invalid request body : %+v", err)
		return fiber.ErrBadRequest
	}

	var user entity.User
	if err := c.UserRepository.FindByTelegramID(tx, &user, request.TelegramId); err != nil {
		return fiber.ErrNotFound
	}

	// Проверяем новый пароль
	user.Password = request.NewPassword

	newAttendance := mirea.NewAttendance(c.Config, user, c.Redis)
	newAttendance.SetUseCase(false)
	if err := newAttendance.Authorization(); err != nil {
		c.Log.Warnf("Wrong new password : %+v", err)
		return fiber.NewError(403, "Неверный новый пароль")
	}

	// Encrypt new password before saving
	encryptedPassword, err := c.Encryptor.Encrypt(request.NewPassword)
	if err != nil {
		c.Log.Errorf("Failed to encrypt new password: %+v", err)
		return fiber.ErrInternalServerError
	}

	// Обновляем пароль в базе данных
	user.Password = encryptedPassword
	if err := c.UserRepository.Update(tx, &user); err != nil {
		c.Log.Warnf("Failed update user password : %+v", err)
		return fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Warnf("Failed commit transaction : %+v", err)
		return fiber.ErrInternalServerError
	}

	return nil
}

func getCourseByGroup(code string) int {
	parts := strings.Split(code, "-")
	if len(parts) < 3 {
		return 0
	}

	// последние две цифры
	lastPart := parts[len(parts)-1]

	yearSuffix, err := strconv.Atoi(lastPart)
	if err != nil {
		return 0
	}

	// формула
	value := (25 - yearSuffix) + 1

	return value
}

func getMoscowDayBounds() (int64, int64) {
	moscowLoc, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(moscowLoc)

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, moscowLoc)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, moscowLoc)

	return startOfDay.Unix(), endOfDay.Unix()
}

// getBuildingName преобразует код корпуса в читаемое название
func getBuildingName(locationTitle string) string {
	if locationTitle == "Неконтролируемая территория" {
		return "Улица"
	}

	// Берем только первое слово (код корпуса)
	parts := strings.Fields(locationTitle)
	if len(parts) == 0 {
		return locationTitle
	}

	code := parts[0]

	// Преобразуем известные коды корпусов
	switch code {
	case "С20":
		return "Стромынка"
	case "П1":
		return "Пироговка"
	case "В78":
		return "Вернадка"
	default:
		return code
	}
}

func (c *UserUseCase) GetUniversityStatus(ctx context.Context, user *entity.User) (*model.UniversityStatusResponse, error) {
	// Get user with decrypted password for authorization
	userWithDecryptedPassword, err := c.createUserWithDecryptedPassword(*user)
	if err != nil {
		c.Log.Errorf("Failed to decrypt password for user %s: %+v", user.Email, err)
		return nil, fiber.NewError(500, "Не удалось расшифровать пароль")
	}
	userWithDecryptedPassword.Password = "" // Предотвращаем спам кодами при проверке статуса в вузе

	attendance := mirea.NewAttendance(c.Config, userWithDecryptedPassword, c.Redis)

	if err := attendance.Authorization(); err != nil {
		c.Log.Warnf("Failed to authorize attendance : %+v", err)
		return nil, fiber.NewError(500, "Не удалось авторизоваться в системе посещаемости")
	}

	startTime, endTime := getMoscowDayBounds()

	events, err := attendance.GetHumanAcsEvents(startTime, endTime)
	if err != nil {
		c.Log.Errorf("Failed to get ACS events : %+v", err)
		if err != nil && err.Error() != "wrong base64" {
			return nil, fiber.NewError(500, "Не удалось получить данные о проходах")
		}
	}

	response := &model.UniversityStatusResponse{
		IsInUniversity: false,
		EntryTime:      0,
		ExitTime:       0,
		Events:         []model.UniversityEventDetail{},
	}

	if len(events) > 0 {
		sort.Slice(events, func(i, j int) bool {
			return events[i].GetTime().GetValue() < events[j].GetTime().GetValue()
		})

		response.EntryTime = events[0].GetTime().GetValue()

		// Создаем детальную информацию о событиях
		for i, event := range events {
			entryLocation := ""
			exitLocation := ""

			if event.GetEntryLocation() != nil {
				entryLocation = getBuildingName(event.GetEntryLocation().GetTitle())
			}
			if event.GetExitLocation() != nil {
				exitLocation = getBuildingName(event.GetExitLocation().GetTitle())
			}

			// Определяем, является ли событие входом или выходом
			// Если это первое событие или предыдущее событие было выходом, то это вход
			isEntry := i == 0
			if i > 0 {
				prevEvent := events[i-1]
				prevIsExit := false
				if prevEvent.GetExitLocation() != nil && prevEvent.GetExitLocation().GetTitle() != "Неконтролируемая территория" {
					prevIsExit = true
				}
				isEntry = prevIsExit
			}

			response.Events = append(response.Events, model.UniversityEventDetail{
				Time:          event.GetTime().GetValue(),
				EntryLocation: entryLocation,
				ExitLocation:  exitLocation,
				IsEntry:       isEntry,
			})
		}

		// Определяем, находится ли студент в университете
		// Возможные значение GetTitle()
		// С20 1эт ЦентральныйВход
		// С20 КПП2
		// П1 А 1эт ЦентральныйВход ДополнительныйВход
		// и т.д
		if events[len(events)-1].GetExitLocation().GetTitle() != "Неконтролируемая территория" {
			response.IsInUniversity = false
			response.ExitTime = events[len(events)-1].GetTime().GetValue()
		} else {
			response.IsInUniversity = true
		}
	}

	return response, nil
}

func (c *UserUseCase) DeleteUser(ctx context.Context, user *entity.User) error {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := c.LinkUserRepository.DeleteByFromUser(tx, user.ID); err != nil {
		c.Log.Warnf("Failed to delete user connections as from_user : %+v", err)
		return fiber.ErrInternalServerError
	}

	if err := c.LinkUserRepository.DeleteByToUser(tx, user.ID); err != nil {
		c.Log.Warnf("Failed to delete user connections as to_user : %+v", err)
		return fiber.ErrInternalServerError
	}

	if err := c.UserRepository.Delete(tx, user); err != nil {
		c.Log.Warnf("Failed to delete user : %+v", err)
		return fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Warnf("Failed commit transaction : %+v", err)
		return fiber.ErrInternalServerError
	}

	return nil
}

func (c *UserUseCase) UpdateProxy(ctx context.Context, request *model.UpdateProxyRequest) error {
	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	err := c.Validate.Struct(request)
	if err != nil {
		c.Log.Warnf("Invalid request body : %+v", err)
		return fiber.ErrBadRequest
	}

	var user entity.User
	if err := c.UserRepository.FindByTelegramID(tx, &user, request.TelegramId); err != nil {
		return fiber.ErrNotFound
	}

	user.CustomProxy = request.Proxy
	if err := c.UserRepository.Update(tx, &user); err != nil {
		c.Log.Warnf("Failed update user proxy : %+v", err)
		return fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		c.Log.Warnf("Failed commit transaction : %+v", err)
		return fiber.ErrInternalServerError
	}

	return nil
}

// CreateInvite создаёт одноразовую ссылку-приглашение.
// Семантика: fromUser создаёт ссылку → отправляет другу → друг принимает → fromUser будет отмечать друга.
func (c *UserUseCase) CreateInvite(ctx context.Context, fromUser *entity.User) (*model.CreateInviteResponse, error) {
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fiber.ErrInternalServerError
	}
	token := "inv_" + hex.EncodeToString(tokenBytes)

	payload, err := json.Marshal(map[string]string{
		"from_user_id": fromUser.ID,
		"from_email":   fromUser.Email,
	})
	if err != nil {
		return nil, fiber.ErrInternalServerError
	}

	expiresAt := time.Now().Add(inviteTTL).Unix()
	if err := c.Redis.Set(ctx, inviteKeyPrefix+token, payload, inviteTTL).Err(); err != nil {
		return nil, fiber.ErrInternalServerError
	}

	return &model.CreateInviteResponse{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

// GetInviteInfo возвращает информацию об авторе приглашения (без принятия).
func (c *UserUseCase) GetInviteInfo(ctx context.Context, token string) (*model.InviteInfoResponse, error) {
	data, err := c.Redis.Get(ctx, inviteKeyPrefix+token).Bytes()
	if err != nil {
		return nil, fiber.NewError(404, "Приглашение не найдено или устарело")
	}
	var payload map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fiber.ErrInternalServerError
	}
	var fromUser entity.User
	if err := c.UserRepository.FindByEmail(c.DB, &fromUser, payload["from_email"]); err != nil {
		return nil, fiber.NewError(404, "Пользователь не найден")
	}
	return &model.InviteInfoResponse{
		Fullname: fromUser.Fullname,
		Group:    fromUser.Group,
	}, nil
}

// AcceptInvite принимает ссылку-приглашение.
// toUser — текущий пользователь (тот, кого будут отмечать).
// Создаётся LinkUser{FromUserID: fromUser, ToUserID: toUser}.
func (c *UserUseCase) AcceptInvite(ctx context.Context, token string, toUser *entity.User) error {
	data, err := c.Redis.Get(ctx, inviteKeyPrefix+token).Bytes()
	if err != nil {
		return fiber.NewError(404, "Приглашение не найдено или устарело")
	}
	var payload map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		return fiber.ErrInternalServerError
	}

	fromEmail := payload["from_email"]
	if fromEmail == toUser.Email {
		return fiber.NewError(400, "Нельзя принять собственное приглашение")
	}

	var fromUser entity.User
	if err := c.UserRepository.FindByEmail(c.DB, &fromUser, fromEmail); err != nil {
		return fiber.NewError(404, "Пользователь-инициатор не найден")
	}

	tx := c.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	var existing entity.LinkUser
	if err := c.LinkUserRepository.FindLinkUser(tx, &existing, fromUser.ID, toUser.ID); err == nil && existing.FromUserID != "" {
		return fiber.NewError(409, "Уже подключён")
	}

	link1 := entity.LinkUser{FromUserID: fromUser.ID, ToUserID: toUser.ID, Enabled: true, Status: "accepted"}
	if err := c.LinkUserRepository.Create(tx, &link1); err != nil {
		return fiber.ErrInternalServerError
	}

	var reverseExisting entity.LinkUser
	if err := c.LinkUserRepository.FindLinkUser(tx, &reverseExisting, toUser.ID, fromUser.ID); err != nil || reverseExisting.FromUserID == "" {
		link2 := entity.LinkUser{FromUserID: toUser.ID, ToUserID: fromUser.ID, Enabled: true, Status: "accepted"}
		if err := c.LinkUserRepository.Create(tx, &link2); err != nil {
			return fiber.ErrInternalServerError
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fiber.ErrInternalServerError
	}

	if c.Bot != nil {
		c.Bot.Send(tgbotapi.NewMessage(fromUser.TelegramId,
			fmt.Sprintf("%s принял(а) ваше приглашение и добавлен(а) в список отслеживания", toUser.Fullname)))
	}

	return nil
}

func (c *UserUseCase) CheckSubscription(ctx context.Context, user *entity.User, forceCheck bool) (bool, error) {
	if c.Bot == nil {
		return false, errors.New("bot is not initialized")
	}

	cacheKey := fmt.Sprintf("sub_status_%d", user.TelegramId)

	// Если не форсируем, пытаемся отдать из кэша (чтобы при частых обновлениях страницы не спамить Telegram API)
	if !forceCheck {
		if val, err := c.Redis.Get(ctx, cacheKey).Result(); err == nil {
			return val == "1", nil
		}
	}

	member, err := c.Bot.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			SuperGroupUsername: "@mirea_tools",
			UserID:             user.TelegramId,
		},
	})
	if err != nil {
		c.Log.Warnf("Failed to check subscription for user %d: %+v", user.TelegramId, err)
		return user.IsSubscribed, nil // Если Telegram API недоступен, полагаемся на последнее значение из БД
	}

	isSubscribed := member.Status == "member" || member.Status == "administrator" || member.Status == "creator"

	// Обновляем флаг в БД, только если статус изменился
	if user.IsSubscribed != isSubscribed {
		user.IsSubscribed = isSubscribed
		_ = c.UserRepository.Update(c.DB, user)
	}

	statusStr := "0"
	if isSubscribed {
		statusStr = "1"
	}
	c.Redis.Set(ctx, cacheKey, statusStr, 5*time.Minute) // Кэшируем результат на 5 минут

	return isSubscribed, nil
}

func (c *UserUseCase) SyncSession(ctx context.Context, user *entity.User, telegramHash string, req *model.SyncSessionRequest) (*model.OtpRequiredResponse, error) {
	key := otpPendingKey(telegramHash, user.TelegramId)

	forceNewCode := false
	if req != nil {
		forceNewCode = req.ForceNewCode
	}

	if !forceNewCode {
		if data, err := c.Redis.Get(ctx, key).Bytes(); err == nil {
			var pending model.OtpPendingData
			if err := json.Unmarshal(data, &pending); err == nil {
				hashForFront := telegramHash
				if hashForFront == "" {
					hashForFront = strconv.FormatInt(user.TelegramId, 10)
				}
				return &model.OtpRequiredResponse{OtpRequired: true, TelegramHash: hashForFront, OtpType: pending.OtpType}, nil
			}
		}

		// Проверяем наличие сессии в кэше, чтобы зря не дёргать ПУЛЬС, если её там нет
		sessionKey := "sess_" + user.Email
		if c.Redis.Exists(ctx, sessionKey).Val() == 0 {
			return &model.OtpRequiredResponse{OtpRequired: true, TelegramHash: "", OtpType: ""}, nil
		}

		// Проверяем реальную работоспособность сессии
		userWithDecryptedPassword, err := c.createUserWithDecryptedPassword(*user)
		if err != nil {
			return nil, fiber.ErrInternalServerError
		}
		userWithDecryptedPassword.Password = "" // Не отправляем код

		attendance := mirea.NewAttendance(c.Config, userWithDecryptedPassword, c.Redis)
		attendance.SetUseCase(true) // Используем кэш

		if err := attendance.Authorization(); err != nil {
			var authErr *customerrors.AuthError
			if errors.As(err, &authErr) && (authErr.Type == "network_error" || authErr.Type == "site_unavailable" || authErr.Type == "internal_error") {
				// ПУЛЬС лагает, не сбрасываем живую сессию
				return nil, nil
			}

			c.Redis.Del(ctx, sessionKey)
			return &model.OtpRequiredResponse{OtpRequired: true, TelegramHash: "", OtpType: ""}, nil
		}

		// Сессия действительно активна и работает
		return nil, nil
	} else {
		c.Redis.Del(ctx, key)
		c.Redis.Del(ctx, "sess_"+user.Email) // Очищаем старую сессию для принудительного запроса нового кода
	}

	userWithDecryptedPassword, err := c.createUserWithDecryptedPassword(*user)
	if err != nil {
		return nil, fiber.ErrInternalServerError
	}

	attendance := mirea.NewAttendance(c.Config, userWithDecryptedPassword, c.Redis)
	attendance.SetUseCase(false) // Принудительно логинимся, чтобы ПУЛЬС отправил код

	if err := attendance.Authorization(); err != nil {
		var authErr *customerrors.AuthError
		if errors.As(err, &authErr) && authErr.Type == "otp_is_required" {
			loginActionURL := authErr.GetLoginActionUrl()
			otpType := authErr.GetOtpType()
			pending := model.OtpPendingData{
				TelegramId:       user.TelegramId,
				TelegramUsername: user.TelegramUsername,
				Email:            user.Email,
				Password:         user.Password,
				LoginActionURL:   loginActionURL,
				OtpType:          otpType,
			}
			_ = c.savePendingOtp(ctx, key, &pending)
			notifiedKey := fmt.Sprintf("session_notified_%d", user.TelegramId)
			if c.Redis.Get(ctx, notifiedKey).Err() == redis.Nil {
				if c.Bot != nil {
					c.Bot.Send(tgbotapi.NewMessage(user.TelegramId, "⚠️ Ваша сессия в ПУЛЬС истекла. Зайдите в бота и введите новый код авторизации (MAX/Почта) для продолжения работы."))
				}
				c.Redis.Set(ctx, notifiedKey, true, 24*time.Hour)
			}
			hashForFront := telegramHash
			if hashForFront == "" {
				hashForFront = strconv.FormatInt(user.TelegramId, 10)
			}
			return &model.OtpRequiredResponse{OtpRequired: true, TelegramHash: hashForFront, OtpType: otpType}, nil
		}
		return nil, err
	}

	c.Redis.Del(ctx, fmt.Sprintf("session_notified_%d", user.TelegramId))
	return nil, nil
}

// GetGroupmates возвращает список одногруппников, с которыми пользователь еще не взаимодействовал
func (c *UserUseCase) GetGroupmates(ctx context.Context, user *entity.User) ([]*model.UserResponse, error) {
	var groupmates []entity.User

	if user.Group == "" {
		return make([]*model.UserResponse, 0), nil
	}

	cacheKey := "group_users:" + user.Group
	// 1. Пытаемся получить студентов этой группы из Redis (очень быстро)
	if cached, err := c.Redis.Get(ctx, cacheKey).Bytes(); err == nil {
		_ = json.Unmarshal(cached, &groupmates)
	}

	// 2. Если в кэше пусто, делаем запрос в БД и кэшируем (чтобы не делать Full Scan таблицы users каждый раз)
	if len(groupmates) == 0 {
		if err := c.DB.WithContext(ctx).Where("\"group\" = ?", user.Group).Find(&groupmates).Error; err != nil {
			c.Log.Warnf("Failed to find groupmates: %+v", err)
			return nil, fiber.ErrInternalServerError
		}
		if data, err := json.Marshal(groupmates); err == nil {
			c.Redis.Set(ctx, cacheKey, data, 15*time.Minute)
		}
	}

	var links []entity.LinkUser
	// Находим все связи текущего пользователя (отправленные и полученные)
	if err := c.DB.WithContext(ctx).Where("from_user_id = ? OR to_user_id = ?", user.ID, user.ID).Find(&links).Error; err != nil {
		c.Log.Warnf("Failed to find user links: %+v", err)
		return nil, fiber.ErrInternalServerError
	}

	connectedMap := make(map[string]bool)
	for _, link := range links {
		if link.FromUserID == user.ID {
			connectedMap[link.ToUserID] = true
		} else {
			connectedMap[link.FromUserID] = true
		}
	}

	result := make([]*model.UserResponse, 0)
	for _, mate := range groupmates {
		// Исключаем самого себя и тех, с кем уже есть связь
		if mate.ID != user.ID && !connectedMap[mate.ID] {
			mateCopy := mate
			result = append(result, converter.UserToResponse(&mateCopy))
		}
	}

	return result, nil
}
