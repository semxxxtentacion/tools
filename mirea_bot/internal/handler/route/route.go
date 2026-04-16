package route

import (
	"mirea-qr/internal/handler"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

type RouteConfig struct {
	App                *fiber.App
	UserController     *handler.UserController
	MireaController    *handler.MireaController
	AdminController    *handler.AdminController
	ReviewController   *handler.ReviewController
	NoteController     *handler.NoteController
	DebugMiddleware    fiber.Handler
	TelegramMiddleware fiber.Handler
	RegisterMiddleware fiber.Handler
	AdminMiddleware    fiber.Handler
}

func corsMiddleware() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"*"},
		AllowHeaders: []string{"*"},
	})
}

func (c *RouteConfig) Setup() {
	c.App.Use(corsMiddleware())
	c.App.Use(c.TelegramMiddleware)
	//c.App.Use(c.DebugMiddleware)

	api := c.App.Group("")
	c.SetupGuestRoute(api)
	c.SetupAuthRoute(api)
	c.SetupAdminRoute(api)
}

func (c *RouteConfig) SetupGuestRoute(api fiber.Router) {
	api.Post("/v1/sign-up", c.UserController.Register)
	api.Post("/v1/submit-otp", c.UserController.SubmitOtp)
}

func (c *RouteConfig) SetupAuthRoute(api fiber.Router) {
	// Группа с RegisterMiddleware только для маршрутов, требующих авторизации.
	// /v1/sign-up остаётся без этого middleware, иначе новый пользователь получит 403.
	auth := api.Group("/v1", c.RegisterMiddleware)

	auth.Post("/me", c.UserController.Me)
	auth.Post("/check-subscription", c.UserController.CheckSubscription)
	auth.Post("/change-password", c.UserController.ChangePassword)
	auth.Post("/update-proxy", c.UserController.UpdateProxy)
	auth.Post("/delete-user", c.UserController.DeleteUser)
	auth.Post("/university-status", c.UserController.GetUniversityStatus)

	auth.Post("/disciplines", c.MireaController.Disciplines)
	auth.Post("/find-student", c.MireaController.FindStudent)
	auth.Post("/find-student-by-tg", c.MireaController.FindStudentByTg)

	auth.Post("/create-invite", c.UserController.CreateInvite)
	auth.Post("/invite-info", c.UserController.GetInviteInfo)
	auth.Post("/accept-invite", c.UserController.AcceptInvite)

	auth.Post("/connect-student", c.UserController.ConnectStudent)
	auth.Post("/list-connected-student", c.UserController.ListConnectedStudent)
	auth.Post("/list-connected-to-user", c.UserController.ListConnectedToUser)
	auth.Post("/list-pending-connections", c.UserController.ListPendingConnections)
	auth.Post("/accept-connection", c.UserController.AcceptConnection)
	auth.Post("/decline-connection", c.UserController.DeclineConnection)
	auth.Post("/enabled-connected-student", c.UserController.EnabledConnectedStudent)
	auth.Post("/disconnect-student", c.UserController.DisconnectStudent)
	auth.Post("/disconnect-from-user", c.UserController.DisconnectFromUser)
	auth.Post("/groupmates", c.UserController.GetGroupmates)
	auth.Post("/sync-session", c.UserController.SyncSession)

	auth.Post("/scan-qr", c.MireaController.ScanQR)

	auth.Post("/status-of-bypass", c.MireaController.CheckStatusBypass)

	auth.Post("/lessons", c.MireaController.GetLessons)
	auth.Post("/attendance", c.MireaController.Attendance)
	auth.Post("/deadlines", c.MireaController.Deadlines)

	// Отзывы на преподавателей
	auth.Post("/reviews/teachers", c.ReviewController.ListTeachers)
	auth.Post("/reviews/search-teacher", c.ReviewController.SearchTeacher)
	auth.Post("/reviews/add-teacher", c.ReviewController.AddTeacher)
	auth.Post("/reviews/list", c.ReviewController.GetReviews)
	auth.Post("/reviews/create", c.ReviewController.CreateReview)
	auth.Post("/reviews/delete", c.ReviewController.DeleteReview)
	auth.Post("/reviews/like", c.ReviewController.LikeReview)

	// Заметки к парам
	auth.Post("/notes/upsert", c.NoteController.UpsertNote)
	auth.Post("/notes/delete", c.NoteController.DeleteNote)
	auth.Post("/notes/list", c.NoteController.GetNotes)
}

func (c *RouteConfig) SetupAdminRoute(api fiber.Router) {
	admin := api.Group("/v1/admin", c.RegisterMiddleware, c.AdminMiddleware)
	admin.Post("/stats", c.AdminController.GetStats)
}
