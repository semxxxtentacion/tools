package handler

import (
	"mirea-qr/internal/handler/middleware"
	"mirea-qr/internal/model"
	"mirea-qr/internal/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
)

type ReviewController struct {
	Log     *logrus.Logger
	UseCase *usecase.ReviewUseCase
}

func NewReviewController(useCase *usecase.ReviewUseCase, logger *logrus.Logger) *ReviewController {
	return &ReviewController{
		Log:     logger,
		UseCase: useCase,
	}
}

func (c *ReviewController) ListTeachers(ctx fiber.Ctx) error {
	response, err := c.UseCase.ListAllTeachers(ctx.UserContext())
	if err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[[]model.TeacherResponse]{Data: response})
}

func (c *ReviewController) SearchTeacher(ctx fiber.Ctx) error {
	request := new(model.SearchTeacherRequest)

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	response, err := c.UseCase.SearchTeachers(ctx.UserContext(), request)
	if err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[[]model.TeacherResponse]{Data: response})
}

func (c *ReviewController) AddTeacher(ctx fiber.Ctx) error {
	request := new(model.AddTeacherRequest)

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	response, err := c.UseCase.AddTeacher(ctx.UserContext(), request)
	if err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[*model.TeacherResponse]{Data: response})
}

func (c *ReviewController) GetReviews(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.GetReviewsRequest)

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	response, err := c.UseCase.GetReviews(ctx.UserContext(), request, user.ID)
	if err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[*model.TeacherReviewsResponse]{Data: response})
}

func (c *ReviewController) CreateReview(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.CreateReviewRequest)

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	request.UserID = user.ID

	if err := c.UseCase.CreateReview(ctx.UserContext(), request); err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *ReviewController) DeleteReview(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.DeleteReviewRequest)

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	request.UserID = user.ID

	if err := c.UseCase.DeleteReview(ctx.UserContext(), request); err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

func (c *ReviewController) LikeReview(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	request := new(model.LikeReviewRequest)

	if err := ctx.Bind().JSON(request); err != nil {
		c.Log.Warnf("Failed to parse request body : %+v", err)
		return fiber.ErrBadRequest
	}

	request.UserID = user.ID

	if err := c.UseCase.LikeReview(ctx.UserContext(), request); err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}
