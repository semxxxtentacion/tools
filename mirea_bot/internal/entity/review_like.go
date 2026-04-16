package entity

// ReviewLike — лайк или дизлайк пользователя на отзыв.
// Один пользователь может поставить не более одного голоса на отзыв.
type ReviewLike struct {
	ID       uint   `gorm:"column:id;primaryKey;autoIncrement"`
	ReviewID uint   `gorm:"column:review_id;not null;uniqueIndex:idx_review_user_like"`
	UserID   string `gorm:"column:user_id;size:255;not null;uniqueIndex:idx_review_user_like"`
	IsLike   bool   `gorm:"column:is_like;not null"`
}
