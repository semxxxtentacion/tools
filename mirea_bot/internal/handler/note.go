package handler

import (
	"mirea-qr/internal/handler/middleware"
	"mirea-qr/internal/model"
	"mirea-qr/internal/usecase"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
)

type NoteController struct {
	Log     *logrus.Logger
	UseCase *usecase.NoteUseCase
}

func NewNoteController(useCase *usecase.NoteUseCase, logger *logrus.Logger) *NoteController {
	return &NoteController{
		Log:     logger,
		UseCase: useCase,
	}
}

// UpsertNote создаёт или обновляет заметку к паре.
func (c *NoteController) UpsertNote(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	req := new(model.UpsertNoteRequest)

	if err := ctx.Bind().JSON(req); err != nil {
		c.Log.Warnf("Failed to parse upsert note request: %+v", err)
		return fiber.ErrBadRequest
	}

	req.UserID = user.ID

	resp, err := c.UseCase.UpsertNote(ctx.UserContext(), req)
	if err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[*model.NoteResponse]{Data: resp})
}

// DeleteNote удаляет заметку к паре.
func (c *NoteController) DeleteNote(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	req := new(model.DeleteNoteRequest)

	if err := ctx.Bind().JSON(req); err != nil {
		c.Log.Warnf("Failed to parse delete note request: %+v", err)
		return fiber.ErrBadRequest
	}

	req.UserID = user.ID

	if err := c.UseCase.DeleteNote(ctx.UserContext(), req); err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[string]{Data: "success"})
}

// GetNotes возвращает заметки для списка UUID пар.
func (c *NoteController) GetNotes(ctx fiber.Ctx) error {
	user := middleware.GetUser(ctx)
	req := new(model.GetNotesRequest)

	if err := ctx.Bind().JSON(req); err != nil {
		c.Log.Warnf("Failed to parse get notes request: %+v", err)
		return fiber.ErrBadRequest
	}

	req.UserID = user.ID

	resp, err := c.UseCase.GetNotes(ctx.UserContext(), req)
	if err != nil {
		return err
	}

	return ctx.JSON(model.WebResponse[[]model.NoteResponse]{Data: resp})
}
