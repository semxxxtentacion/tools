package converter

import (
	"mirea-qr/internal/entity"
	"mirea-qr/internal/model"
)

func TeacherToResponse(teacher *entity.Teacher, reviewCount int64, avgStars float64) model.TeacherResponse {
	return model.TeacherResponse{
		ID:          teacher.ID,
		Name:        teacher.Name,
		ReviewCount: reviewCount,
		AvgStars:    avgStars,
	}
}

func ReviewToResponse(review *entity.TeacherReview, currentUserID string, likes, dislikes int64, myVote int) model.ReviewResponse {
	return model.ReviewResponse{
		ID:        review.ID,
		Comment:   review.Comment,
		Course:    review.Course,
		Stars:     review.Stars,
		CreatedAt: review.CreatedAt,
		IsMine:    review.UserID == currentUserID,
		Likes:     likes,
		Dislikes:  dislikes,
		MyVote:    myVote,
	}
}
