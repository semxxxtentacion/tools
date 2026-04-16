package repository

import (
	"mirea-qr/internal/entity"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReviewLikeRepository struct {
	Repository[entity.ReviewLike]
	Log *logrus.Logger
}

func NewReviewLikeRepository(log *logrus.Logger) *ReviewLikeRepository {
	return &ReviewLikeRepository{Log: log}
}

// CountByReview возвращает количество лайков и дизлайков для отзыва.
func (r *ReviewLikeRepository) CountByReview(db *gorm.DB, reviewID uint) (likes int64, dislikes int64) {
	db.Model(&entity.ReviewLike{}).Where("review_id = ? AND is_like = true", reviewID).Count(&likes)
	db.Model(&entity.ReviewLike{}).Where("review_id = ? AND is_like = false", reviewID).Count(&dislikes)
	return
}

// FindByReviewAndUser возвращает голос пользователя для отзыва (если есть).
func (r *ReviewLikeRepository) FindByReviewAndUser(db *gorm.DB, reviewID uint, userID string) (*entity.ReviewLike, error) {
	var like entity.ReviewLike
	err := db.Where("review_id = ? AND user_id = ?", reviewID, userID).Take(&like).Error
	if err != nil {
		return nil, err
	}
	return &like, nil
}

// Upsert создаёт или обновляет голос пользователя.
func (r *ReviewLikeRepository) Upsert(db *gorm.DB, like *entity.ReviewLike) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "review_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"is_like"}),
	}).Create(like).Error
}

// DeleteByReviewAndUser удаляет голос пользователя для отзыва.
func (r *ReviewLikeRepository) DeleteByReviewAndUser(db *gorm.DB, reviewID uint, userID string) error {
	return db.Where("review_id = ? AND user_id = ?", reviewID, userID).Delete(&entity.ReviewLike{}).Error
}

// CountsForReviews возвращает карту reviewID => [likes, dislikes] для списка отзывов.
func (r *ReviewLikeRepository) CountsForReviews(db *gorm.DB, reviewIDs []uint) map[uint][2]int64 {
	type row struct {
		ReviewID uint
		IsLike   bool
		Count    int64
	}
	var rows []row
	db.Model(&entity.ReviewLike{}).
		Select("review_id, is_like, COUNT(*) as count").
		Where("review_id IN ?", reviewIDs).
		Group("review_id, is_like").
		Scan(&rows)

	result := make(map[uint][2]int64)
	for _, row := range rows {
		counts := result[row.ReviewID]
		if row.IsLike {
			counts[0] = row.Count
		} else {
			counts[1] = row.Count
		}
		result[row.ReviewID] = counts
	}
	return result
}

// VotesForUser возвращает карту reviewID => is_like для голосов пользователя.
func (r *ReviewLikeRepository) VotesForUser(db *gorm.DB, reviewIDs []uint, userID string) map[uint]bool {
	var likes []entity.ReviewLike
	db.Where("review_id IN ? AND user_id = ?", reviewIDs, userID).Find(&likes)

	result := make(map[uint]bool)
	for _, l := range likes {
		result[l.ReviewID] = l.IsLike
	}
	return result
}
