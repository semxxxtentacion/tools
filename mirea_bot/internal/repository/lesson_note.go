package repository

import (
	"mirea-qr/internal/entity"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type LessonNoteRepository struct {
	Log *logrus.Logger
}

func NewLessonNoteRepository(log *logrus.Logger) *LessonNoteRepository {
	return &LessonNoteRepository{Log: log}
}

// FindByUserAndLesson возвращает заметку пользователя для конкретной пары.
func (r *LessonNoteRepository) FindByUserAndLesson(db *gorm.DB, userID, lessonUUID string) (*entity.LessonNote, error) {
	var note entity.LessonNote
	err := db.Where("user_id = ? AND lesson_uuid = ?", userID, lessonUUID).Take(&note).Error
	if err != nil {
		return nil, err
	}
	return &note, nil
}

// FindByUserAndLessons возвращает все заметки пользователя для списка UUID пар.
func (r *LessonNoteRepository) FindByUserAndLessons(db *gorm.DB, userID string, lessonUUIDs []string) ([]entity.LessonNote, error) {
	var notes []entity.LessonNote
	err := db.Where("user_id = ? AND lesson_uuid IN ?", userID, lessonUUIDs).Find(&notes).Error
	return notes, err
}

// Upsert создаёт или обновляет заметку.
func (r *LessonNoteRepository) Upsert(db *gorm.DB, note *entity.LessonNote) error {
	var existing entity.LessonNote
	err := db.Where("user_id = ? AND lesson_uuid = ?", note.UserID, note.LessonUUID).Take(&existing).Error
	if err != nil {
		// Не найдена — создаём
		return db.Create(note).Error
	}
	// Найдена — обновляем
	return db.Model(&existing).Updates(map[string]interface{}{
		"lesson_title":     note.LessonTitle,
		"lesson_start":     note.LessonStart,
		"note_text":        note.NoteText,
		"reminder_enabled": note.ReminderEnabled,
		"reminder_at":      note.ReminderAt,
		"is_sent":          false,
	}).Error
}

// Delete удаляет заметку пользователя для конкретной пары.
func (r *LessonNoteRepository) Delete(db *gorm.DB, userID, lessonUUID string) error {
	return db.Where("user_id = ? AND lesson_uuid = ?", userID, lessonUUID).
		Delete(&entity.LessonNote{}).Error
}

// FindPendingReminders возвращает все заметки с включёнными напоминаниями,
// время которых наступило и которые ещё не отправлены.
func (r *LessonNoteRepository) FindPendingReminders(db *gorm.DB, nowUnix int64) ([]entity.LessonNote, error) {
	var notes []entity.LessonNote
	err := db.Where("reminder_enabled = true AND is_sent = false AND reminder_at > 0 AND reminder_at <= ?", nowUnix).
		Find(&notes).Error
	return notes, err
}

// MarkSent помечает напоминание как отправленное.
func (r *LessonNoteRepository) MarkSent(db *gorm.DB, id uint) error {
	return db.Model(&entity.LessonNote{}).Where("id = ?", id).Update("is_sent", true).Error
}
