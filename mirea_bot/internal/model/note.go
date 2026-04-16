package model

// UpsertNoteRequest — создание или обновление заметки к паре.
type UpsertNoteRequest struct {
	UserID          string `json:"-"`
	LessonUUID      string `json:"lesson_uuid" validate:"required"`
	LessonTitle     string `json:"lesson_title"`
	LessonStart     int64  `json:"lesson_start"`
	NoteText        string `json:"note_text" validate:"required,min=1,max=1000"`
	ReminderEnabled bool   `json:"reminder_enabled"`
	// ReminderAt — Unix timestamp (секунды) времени напоминания. 0 если не задано.
	ReminderAt int64 `json:"reminder_at"`
}

// DeleteNoteRequest — удаление заметки к паре.
type DeleteNoteRequest struct {
	UserID     string `json:"-"`
	LessonUUID string `json:"lesson_uuid" validate:"required"`
}

// GetNotesRequest — получение заметок для списка UUID пар.
type GetNotesRequest struct {
	UserID      string   `json:"-"`
	LessonUUIDs []string `json:"lesson_uuids" validate:"required,min=1"`
}

// NoteResponse — ответ с данными заметки.
type NoteResponse struct {
	LessonUUID      string `json:"lesson_uuid"`
	NoteText        string `json:"note_text"`
	ReminderEnabled bool   `json:"reminder_enabled"`
	ReminderAt      int64  `json:"reminder_at"`
	IsSent          bool   `json:"is_sent"`
}
