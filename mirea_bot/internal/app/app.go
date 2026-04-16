package app

import (
	"mirea-qr/internal/config"
	"mirea-qr/internal/handler"
	"mirea-qr/internal/handler/middleware"
	"mirea-qr/internal/handler/route"
	"mirea-qr/internal/repository"
	"mirea-qr/internal/usecase"
	"mirea-qr/pkg/crypto"

	"github.com/go-playground/validator/v10"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type BootstrapConfig struct {
	DB        *gorm.DB
	App       *fiber.App
	Log       *logrus.Logger
	Cfg       config.Config
	Redis     *redis.Client
	Validator *validator.Validate
	Bot       *tgbotapi.BotAPI
	Encryptor *crypto.Encryptor
}

func Bootstrap(boot BootstrapConfig) route.RouteConfig {
	// setup repositories
	userRepository := repository.NewUserRepository(boot.Log)
	linkUserRepository := repository.NewLinkUserRepository(boot.Log)
	subjectAttendanceRepository := repository.NewSubjectAttendanceRepository(boot.Log)
	qrScanRepository := repository.NewQrScanRepository(boot.DB)
	teacherRepository := repository.NewTeacherRepository(boot.Log)
	teacherReviewRepository := repository.NewTeacherReviewRepository(boot.Log)
	reviewLikeRepository := repository.NewReviewLikeRepository(boot.Log)
	lessonNoteRepository := repository.NewLessonNoteRepository(boot.Log)

	// setup use cases
	userUseCase := usecase.NewUserUseCase(&boot.Cfg, boot.DB, boot.Log, boot.Validator, userRepository, linkUserRepository, boot.Redis, boot.Bot, boot.Encryptor)
	mireaUseCase := usecase.NewMireaUseCase(&boot.Cfg, boot.DB, boot.Log, boot.Validator, userRepository, linkUserRepository, subjectAttendanceRepository, boot.Redis, boot.Bot, boot.Encryptor)
	adminUseCase := usecase.NewAdminUseCase(boot.DB, boot.Log, qrScanRepository, boot.Redis)
	reviewUseCase := usecase.NewReviewUseCase(boot.DB, boot.Log, boot.Validator, userRepository, teacherRepository, teacherReviewRepository, reviewLikeRepository)
	noteUseCase := usecase.NewNoteUseCase(boot.DB, boot.Log, boot.Validator, lessonNoteRepository, userRepository, boot.Bot)
	noteUseCase.StartReminderScheduler()

	// setup controller
	userController := handler.NewUserController(userUseCase, boot.Log)
	mireaController := handler.NewMireaController(mireaUseCase, adminUseCase, boot.Log)
	adminController := handler.NewAdminController(adminUseCase, boot.Log)
	reviewController := handler.NewReviewController(reviewUseCase, boot.Log)
	noteController := handler.NewNoteController(noteUseCase, boot.Log)

	// setup middleware
	telegramMiddleware := middleware.NewTelegram(boot.Cfg.TelegramBotToken, userUseCase)
	registerMiddleware := middleware.NewRegister(userUseCase)
	debugMiddleware := middleware.NewDebug()
	adminMiddleware := middleware.NewAdminMiddleware(&middleware.AdminConfig{
		AdminUserID: boot.Cfg.AdminUserID,
	})

	routeConfig := route.RouteConfig{
		App:                boot.App,
		UserController:     userController,
		MireaController:    mireaController,
		AdminController:    adminController,
		ReviewController:   reviewController,
		NoteController:     noteController,
		TelegramMiddleware: telegramMiddleware,
		RegisterMiddleware: registerMiddleware,
		DebugMiddleware:    debugMiddleware,
		AdminMiddleware:    adminMiddleware,
	}
	routeConfig.Setup()

	return routeConfig
}
