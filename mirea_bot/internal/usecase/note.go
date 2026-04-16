package usecase

import (
	"context"
	"fmt"
	"mirea-qr/internal/entity"
	"mirea-qr/internal/model"
	"mirea-qr/internal/repository"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type NoteUseCase struct {
	DB             *gorm.DB
	Log            *logrus.Logger
	Validate       *validator.Validate
	NoteRepository *repository.LessonNoteRepository
	UserRepository *repository.UserRepository
	Bot            *tgbotapi.BotAPI
}

func NewNoteUseCase(
	db *gorm.DB,
	log *logrus.Logger,
	validate *validator.Validate,
	noteRepo *repository.LessonNoteRepository,
	userRepo *repository.UserRepository,
	bot *tgbotapi.BotAPI,
) *NoteUseCase {
	return &NoteUseCase{
		DB:             db,
		Log:            log,
		Validate:       validate,
		NoteRepository: noteRepo,
		UserRepository: userRepo,
		Bot:            bot,
	}
}

func (uc *NoteUseCase) UpsertNote(ctx context.Context, req *model.UpsertNoteRequest) (*model.NoteResponse, error) {
	if err := uc.Validate.Struct(req); err != nil {
		uc.Log.Warnf("Invalid upsert note request: %+v", err)
		return nil, fiber.ErrBadRequest
	}

	note := &entity.LessonNote{
		UserID:          req.UserID,
		LessonUUID:      req.LessonUUID,
		LessonTitle:     req.LessonTitle,
		LessonStart:     req.LessonStart,
		NoteText:        req.NoteText,
		ReminderEnabled: req.ReminderEnabled,
		ReminderAt:      req.ReminderAt,
	}

	if err := uc.NoteRepository.Upsert(uc.DB.WithContext(ctx), note); err != nil {
		uc.Log.Errorf("Failed to upsert note: %+v", err)
		return nil, fiber.ErrInternalServerError
	}

	saved, err := uc.NoteRepository.FindByUserAndLesson(uc.DB.WithContext(ctx), req.UserID, req.LessonUUID)
	if err != nil {
		return nil, fiber.ErrInternalServerError
	}

	return noteToResponse(saved), nil
}

func (uc *NoteUseCase) DeleteNote(ctx context.Context, req *model.DeleteNoteRequest) error {
	if err := uc.Validate.Struct(req); err != nil {
		uc.Log.Warnf("Invalid delete note request: %+v", err)
		return fiber.ErrBadRequest
	}

	if err := uc.NoteRepository.Delete(uc.DB.WithContext(ctx), req.UserID, req.LessonUUID); err != nil {
		uc.Log.Errorf("Failed to delete note: %+v", err)
		return fiber.ErrInternalServerError
	}
	return nil
}

func (uc *NoteUseCase) GetNotes(ctx context.Context, req *model.GetNotesRequest) ([]model.NoteResponse, error) {
	if err := uc.Validate.Struct(req); err != nil {
		uc.Log.Warnf("Invalid get notes request: %+v", err)
		return nil, fiber.ErrBadRequest
	}

	notes, err := uc.NoteRepository.FindByUserAndLessons(uc.DB.WithContext(ctx), req.UserID, req.LessonUUIDs)
	if err != nil {
		uc.Log.Errorf("Failed to get notes: %+v", err)
		return nil, fiber.ErrInternalServerError
	}

	result := make([]model.NoteResponse, 0, len(notes))
	for i := range notes {
		result = append(result, *noteToResponse(&notes[i]))
	}
	return result, nil
}

// StartReminderScheduler запускает фоновый горутин, который каждые 30 секунд
// проверяет напоминания и отправляет сообщения в Telegram.
func (uc *NoteUseCase) StartReminderScheduler() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			uc.processReminders()
		}
	}()
	uc.Log.Info("Reminder scheduler started")
}

func (uc *NoteUseCase) processReminders() {
	nowUnix := time.Now().Unix()
	notes, err := uc.NoteRepository.FindPendingReminders(uc.DB, nowUnix)
	if err != nil {
		uc.Log.Errorf("Failed to fetch pending reminders: %+v", err)
		return
	}

	for _, note := range notes {
		uc.sendReminder(note)
	}
}

func (uc *NoteUseCase) sendReminder(note entity.LessonNote) {
	var user entity.User
	if err := uc.UserRepository.FindById(uc.DB, &user, note.UserID); err != nil {
		uc.Log.Warnf("Reminder: user not found %s: %v", note.UserID, err)
		_ = uc.NoteRepository.MarkSent(uc.DB, note.ID)
		return
	}

	lessonInfo := ""
	if note.LessonTitle != "" {
		lessonInfo = fmt.Sprintf("\n📚 *%s*", note.LessonTitle)
		if note.LessonStart > 0 {
			t := time.Unix(note.LessonStart, 0)
			lessonInfo += fmt.Sprintf(" — %s", t.Format("15:04"))
		}
		lessonInfo += "\n"
	}
	text := fmt.Sprintf("📝 *Напоминание о паре*%s\n%s", lessonInfo, note.NoteText)
	msg := tgbotapi.NewMessage(user.TelegramId, text)
	msg.ParseMode = "Markdown"

	if _, err := uc.Bot.Send(msg); err != nil {
		uc.Log.Warnf("Failed to send reminder to %d: %v", user.TelegramId, err)
	}

	if err := uc.NoteRepository.MarkSent(uc.DB, note.ID); err != nil {
		uc.Log.Errorf("Failed to mark reminder sent %d: %v", note.ID, err)
	}
}

func noteToResponse(n *entity.LessonNote) *model.NoteResponse {
	return &model.NoteResponse{
		LessonUUID:      n.LessonUUID,
		NoteText:        n.NoteText,
		ReminderEnabled: n.ReminderEnabled,
		ReminderAt:      n.ReminderAt,
		IsSent:          n.IsSent,
	}
}
