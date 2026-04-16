package entity

// LessonNote — заметка пользователя к паре.
type LessonNote struct {
	ID              uint   `gorm:"column:id;primaryKey;autoIncrement"`
	UserID          string `gorm:"column:user_id;size:255;not null;index"`
	LessonUUID      string `gorm:"column:lesson_uuid;size:255;not null;index"`
	LessonTitle     string `gorm:"column:lesson_title;size:500;default:''"`
	LessonStart     int64  `gorm:"column:lesson_start;default:0"`
	NoteText        string `gorm:"column:note_text;size:1000;not null"`
	ReminderEnabled bool   `gorm:"column:reminder_enabled;default:false"`
	ReminderAt      int64  `gorm:"column:reminder_at;default:0"`
	IsSent          bool   `gorm:"column:is_sent;default:false"`
	CreatedAt       int64  `gorm:"column:created_at;autoCreateTime:milli"`
	UpdatedAt       int64  `gorm:"column:updated_at;autoCreateTime:milli;autoUpdateTime:milli"`
}
