package handler

import (
	"errors"
	"mirea-qr/internal/entity"
	"mirea-qr/internal/handler/middleware"
	"mirea-qr/internal/model"
	"mirea-qr/internal/model/converter"
	"mirea-qr/internal/usecase"

	"github.com/gofiber/fiber/v3/log"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
)

type UserController struct {
	Log     *logrus.Logger
	UseCase *usecase.UserUseCase
}

func NewUserController(useCase *usecase.UserUseCase, logger *logrus.Logger) *UserController {
	return &UserController{
		Log:     logger,
		UseCase: useCase,
	}
}

func (c *UserController) Register(ctx fiber.Ctx) error {
	request := new(model.RegisterUserRequest)
	log.Debug(*request)
	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}
	request.TelegramId = middleware.GetTelegramId(ctx)
	request.TelegramHash = middleware.GetTelegramHash(ctx)
	request.TelegramUsername = middleware.GetTelegramUsername(ctx)

	response, otpRequired, err := c.UseCase.Create(ctx.UserContext(), request)
	if err != nil {
		c.Log.Warnf("Failed to register user : %+v", err)
		return err
	}
	if otpRequired != nil {
		otpType := otpRequired.OtpType
		if otpType != "max" {
			otpType = "email"
		}
		return ctx.JSON(fiber.Map{
			"data":          nil,
			"otp_required":  otpRequired.OtpRequired,
			"telegram_hash": otpRequired.TelegramHash,
			"otp_type":      otpType,
		})
	}
	return ctx.JSON(model.WebResponse[*model.UserResponse]{Data: response})
}

func (c *UserController) SubmitOtp(ctx fiber.Ctx) error {
	request := new(model.SubmitOtpRequest)
	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse submit-otp body: %+v", err)
		return fiber.ErrBadRequest
	}
	if err := c.UseCase.Validate.Struct(request); err != nil {
		return fiber.ErrBadRequest
	}
	response, err := c.UseCase.SubmitOtp(ctx.UserContext(), request)
	if err != nil {
		c.Log.Warnf("SubmitOtp failed: %+v", err)
		return err
	}
	return ctx.JSON(model.WebResponse[*model.UserResponse]{Data: response})
}

func (c *UserController) Me(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)

	if user == nil {
		return errors.New("user not found")
	}

	if user.Group == "" || string([]rune(user.Group)[:3]) == "ДПЗ" {
		err := c.UseCase.UpdateDataForUser(user)
		if err != nil {
			return err
		}
	}

	// Обновляем telegram_username для существующих пользователей, у которых он пустой
	if tgUsername := middleware.GetTelegramUsername(ctx); tgUsername != "" && user.TelegramUsername == "" {
		user.TelegramUsername = tgUsername
		c.UseCase.DB.Model(user).Update("telegram_username", tgUsername)
	}

	// Проверяем подписку при загрузке профиля (мягко, с использованием кэша)
	isSubscribed, _ := c.UseCase.CheckSubscription(ctx.UserContext(), user, false)

	resp := converter.UserToResponse(user)
	resp.IsSubscribed = isSubscribed

	return ctx.JSON(model.WebResponse[*model.UserResponse]{Data: resp})
}

func (c *UserController) CheckSubscription(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	isSubscribed, err := c.UseCase.CheckSubscription(ctx.UserContext(), user, true) // При нажатии кнопки форсируем проверку в TG API
	if err != nil {
		return err
	}
	return ctx.JSON(model.WebResponse[*model.CheckSubscriptionResponse]{Data: &model.CheckSubscriptionResponse{IsSubscribed: isSubscribed}})
}

func (c *UserController) ConnectStudent(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.ConnectStudentRequest)
	request.TelegramId = user.TelegramId

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	if err := c.UseCase.ConnectStudent(ctx.Context(), request); err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) ListConnectedStudent(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)

	var users []*entity.LinkUser
	if err := c.UseCase.LinkUserRepository.GetConnectedByUser(c.UseCase.DB, &users, user.ID); err != nil {
		return fiber.ErrInternalServerError
	}

	response := []model.ConnectedStudentResponse{}
	for _, user := range users {
		response = append(response, converter.LinkUserToResponse(user))
	}

	return ctx.JSON(model.WebResponse[[]model.ConnectedStudentResponse]{Data: response})
}

func (c *UserController) ListConnectedToUser(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)

	var users []*entity.LinkUser
	if err := c.UseCase.LinkUserRepository.GetConnectedToUser(c.UseCase.DB, &users, user.ID); err != nil {
		return fiber.ErrInternalServerError
	}

	response := []model.ConnectedStudentResponse{}
	for _, linkUser := range users {
		response = append(response, converter.LinkUserToResponseReverse(linkUser))
	}

	return ctx.JSON(model.WebResponse[[]model.ConnectedStudentResponse]{Data: response})
}

func (c *UserController) EnabledConnectedStudent(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.ConnectStudentRequest)
	request.TelegramId = user.TelegramId

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	if err := c.UseCase.ChangeEnabledConnectedStudent(ctx.Context(), request); err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) ChangePassword(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.ChangePasswordRequest)
	request.TelegramId = user.TelegramId

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	if err := c.UseCase.ChangePassword(ctx.Context(), request); err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) GetUniversityStatus(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)

	response, err := c.UseCase.GetUniversityStatus(ctx.Context(), user)
	if err != nil {
		c.Log.Warnf("Failed to get university status : %+v", err)
		return err
	}

	return ctx.JSON(model.WebResponse[*model.UniversityStatusResponse]{Data: response})
}

func (c *UserController) DeleteUser(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)

	if err := c.UseCase.DeleteUser(ctx.Context(), user); err != nil {
		c.Log.Warnf("Failed to delete user : %+v", err)
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) DisconnectStudent(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.ConnectStudentRequest)
	request.TelegramId = user.TelegramId

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	if err := c.UseCase.DisconnectStudent(ctx.Context(), request); err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) DisconnectFromUser(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.ConnectStudentRequest)
	request.TelegramId = user.TelegramId

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	if err := c.UseCase.DisconnectFromUser(ctx.Context(), request); err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) UpdateProxy(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.UpdateProxyRequest)
	request.TelegramId = user.TelegramId

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	if err := c.UseCase.UpdateProxy(ctx.Context(), request); err != nil {
		c.Log.Warnf("Failed to update proxy : %+v", err)
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) CreateInvite(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)

	resp, err := c.UseCase.CreateInvite(ctx.Context(), user)
	if err != nil {
		c.Log.Warnf("Failed to create invite : %+v", err)
		return err
	}

	return ctx.JSON(model.WebResponse[*model.CreateInviteResponse]{Data: resp})
}

func (c *UserController) GetInviteInfo(ctx fiber.Ctx) error {
	request := new(model.AcceptInviteRequest)
	if err := ctx.Bind().JSON(request); err != nil {
		return fiber.ErrBadRequest
	}

	resp, err := c.UseCase.GetInviteInfo(ctx.Context(), request.Token)
	if err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[*model.InviteInfoResponse]{Data: resp})
}

func (c *UserController) ListPendingConnections(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)

	response, err := c.UseCase.GetPendingConnections(ctx.Context(), user)
	if err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[[]model.ConnectedStudentResponse]{Data: response})
}

func (c *UserController) AcceptConnection(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.ConnectionActionRequest)
	request.TelegramId = user.TelegramId

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	if err := c.UseCase.AcceptConnection(ctx.Context(), request.FromUserID, user); err != nil {
		c.Log.Warnf("Failed to accept connection : %+v", err)
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) DeclineConnection(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.ConnectionActionRequest)
	request.TelegramId = user.TelegramId

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	if err := c.UseCase.DeclineConnection(ctx.Context(), request.FromUserID, user); err != nil {
		c.Log.Warnf("Failed to decline connection : %+v", err)
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) AcceptInvite(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.AcceptInviteRequest)
	if err := ctx.Bind().JSON(request); err != nil {
		return fiber.ErrBadRequest
	}

	if err := c.UseCase.AcceptInvite(ctx.Context(), request.Token, user); err != nil {
		c.Log.Warnf("Failed to accept invite : %+v", err)
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) SyncSession(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	tgHash := middleware.GetTelegramHash(ctx)

	req := new(model.SyncSessionRequest)
	_ = ctx.Bind().JSON(req)

	otpReq, err := c.UseCase.SyncSession(ctx.UserContext(), user, tgHash, req)
	if err != nil {
		return err
	}
	if otpReq != nil {
		return ctx.JSON(fiber.Map{
			"data":          nil,
			"otp_required":  otpReq.OtpRequired,
			"telegram_hash": otpReq.TelegramHash,
			"otp_type":      otpReq.OtpType,
		})
	}
	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *UserController) GetGroupmates(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)

	response, err := c.UseCase.GetGroupmates(ctx.Context(), user)
	if err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[[]*model.UserResponse]{Data: response})
}
