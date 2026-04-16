package usecase

import (
	"context"
	"mirea-qr/internal/entity"
	"mirea-qr/internal/model"
	"mirea-qr/internal/model/converter"
	"mirea-qr/internal/repository"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ReviewUseCase struct {
	DB                      *gorm.DB
	Log                     *logrus.Logger
	Validate                *validator.Validate
	UserRepository          *repository.UserRepository
	TeacherRepository       *repository.TeacherRepository
	TeacherReviewRepository *repository.TeacherReviewRepository
	ReviewLikeRepository    *repository.ReviewLikeRepository
}

func NewReviewUseCase(
	db *gorm.DB,
	log *logrus.Logger,
	validate *validator.Validate,
	userRepo *repository.UserRepository,
	teacherRepo *repository.TeacherRepository,
	reviewRepo *repository.TeacherReviewRepository,
	reviewLikeRepo *repository.ReviewLikeRepository,
) *ReviewUseCase {
	return &ReviewUseCase{
		DB:                      db,
		Log:                     log,
		Validate:                validate,
		UserRepository:          userRepo,
		TeacherRepository:       teacherRepo,
		TeacherReviewRepository: reviewRepo,
		ReviewLikeRepository:    reviewLikeRepo,
	}
}

func (uc *ReviewUseCase) ListAllTeachers(ctx context.Context) ([]model.TeacherResponse, error) {
	var teachers []entity.Teacher
	if err := uc.TeacherRepository.FindAll(uc.DB.WithContext(ctx), &teachers); err != nil {
		uc.Log.Errorf("Failed to list all teachers : %+v", err)
		return nil, fiber.ErrInternalServerError
	}

	result := make([]model.TeacherResponse, 0, len(teachers))
	for i := range teachers {
		count, _ := uc.TeacherRepository.CountReviews(uc.DB.WithContext(ctx), teachers[i].ID)
		avg := uc.TeacherRepository.AvgStars(uc.DB.WithContext(ctx), teachers[i].ID)
		result = append(result, converter.TeacherToResponse(&teachers[i], count, avg))
	}

	return result, nil
}

func (uc *ReviewUseCase) SearchTeachers(ctx context.Context, request *model.SearchTeacherRequest) ([]model.TeacherResponse, error) {
	if err := uc.Validate.Struct(request); err != nil {
		uc.Log.Warnf("Invalid search request : %+v", err)
		return nil, fiber.ErrBadRequest
	}

	var teachers []entity.Teacher
	if err := uc.TeacherRepository.SearchByName(uc.DB.WithContext(ctx), &teachers, request.Query); err != nil {
		uc.Log.Errorf("Failed to search teachers : %+v", err)
		return nil, fiber.ErrInternalServerError
	}

	result := make([]model.TeacherResponse, 0, len(teachers))
	for i := range teachers {
		count, _ := uc.TeacherRepository.CountReviews(uc.DB.WithContext(ctx), teachers[i].ID)
		avg := uc.TeacherRepository.AvgStars(uc.DB.WithContext(ctx), teachers[i].ID)
		result = append(result, converter.TeacherToResponse(&teachers[i], count, avg))
	}

	return result, nil
}

func (uc *ReviewUseCase) AddTeacher(ctx context.Context, request *model.AddTeacherRequest) (*model.TeacherResponse, error) {
	if err := uc.Validate.Struct(request); err != nil {
		uc.Log.Warnf("Invalid add teacher request : %+v", err)
		return nil, fiber.ErrBadRequest
	}

	name := strings.TrimSpace(request.Name)

	var existing entity.Teacher
	if err := uc.TeacherRepository.FindByName(uc.DB.WithContext(ctx), &existing, name); err == nil {
		count, _ := uc.TeacherRepository.CountReviews(uc.DB.WithContext(ctx), existing.ID)
		avg := uc.TeacherRepository.AvgStars(uc.DB.WithContext(ctx), existing.ID)
		resp := converter.TeacherToResponse(&existing, count, avg)
		return &resp, nil
	}

	tx := uc.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	teacher := entity.Teacher{Name: name}
	if err := uc.TeacherRepository.Create(tx, &teacher); err != nil {
		uc.Log.Errorf("Failed to create teacher : %+v", err)
		return nil, fiber.NewError(500, "Ne udalos dobavit prepodavatelya")
	}

	if err := tx.Commit().Error; err != nil {
		uc.Log.Errorf("Failed to commit : %+v", err)
		return nil, fiber.ErrInternalServerError
	}

	resp := converter.TeacherToResponse(&teacher, 0, 0)
	return &resp, nil
}

func (uc *ReviewUseCase) GetReviews(ctx context.Context, request *model.GetReviewsRequest, currentUserID string) (*model.TeacherReviewsResponse, error) {
	if err := uc.Validate.Struct(request); err != nil {
		uc.Log.Warnf("Invalid get reviews request : %+v", err)
		return nil, fiber.ErrBadRequest
	}

	var teacher entity.Teacher
	if err := uc.TeacherRepository.FindById(uc.DB.WithContext(ctx), &teacher, request.TeacherID); err != nil {
		return nil, fiber.NewError(404, "Prepodavatel ne naiden")
	}

	var reviews []entity.TeacherReview
	if err := uc.TeacherReviewRepository.FindByTeacherID(uc.DB.WithContext(ctx), &reviews, request.TeacherID); err != nil {
		uc.Log.Errorf("Failed to get reviews : %+v", err)
		return nil, fiber.ErrInternalServerError
	}

	reviewIDs := make([]uint, 0, len(reviews))
	for _, r := range reviews {
		reviewIDs = append(reviewIDs, r.ID)
	}

	var counts map[uint][2]int64
	var userVotes map[uint]bool
	if len(reviewIDs) > 0 {
		counts = uc.ReviewLikeRepository.CountsForReviews(uc.DB.WithContext(ctx), reviewIDs)
		userVotes = uc.ReviewLikeRepository.VotesForUser(uc.DB.WithContext(ctx), reviewIDs, currentUserID)
	}

	reviewResponses := make([]model.ReviewResponse, 0, len(reviews))
	for i := range reviews {
		var likes, dislikes int64
		myVote := 0
		if counts != nil {
			c := counts[reviews[i].ID]
			likes = c[0]
			dislikes = c[1]
		}
		if userVotes != nil {
			if isLike, voted := userVotes[reviews[i].ID]; voted {
				if isLike {
					myVote = 1
				} else {
					myVote = -1
				}
			}
		}
		reviewResponses = append(reviewResponses, converter.ReviewToResponse(&reviews[i], currentUserID, likes, dislikes, myVote))
	}

	avg := uc.TeacherRepository.AvgStars(uc.DB.WithContext(ctx), request.TeacherID)
	return &model.TeacherReviewsResponse{
		Teacher: converter.TeacherToResponse(&teacher, int64(len(reviews)), avg),
		Reviews: reviewResponses,
	}, nil
}

func (uc *ReviewUseCase) CreateReview(ctx context.Context, request *model.CreateReviewRequest) error {
	if err := uc.Validate.Struct(request); err != nil {
		uc.Log.Warnf("Invalid create review request : %+v", err)
		return fiber.ErrBadRequest
	}

	var teacher entity.Teacher
	if err := uc.TeacherRepository.FindById(uc.DB.WithContext(ctx), &teacher, request.TeacherID); err != nil {
		return fiber.NewError(404, "Prepodavatel ne naiden")
	}

	var existing entity.TeacherReview
	if err := uc.TeacherReviewRepository.FindByTeacherAndUser(uc.DB.WithContext(ctx), &existing, request.TeacherID, request.UserID); err == nil {
		return fiber.NewError(409, "Vy uzhe ostavili otziv na etogo prepodavatelya")
	}

	tx := uc.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	var user entity.User
	course := 0
	if err := uc.UserRepository.FindById(uc.DB.WithContext(ctx), &user, request.UserID); err == nil {
		course = getCourseByGroup(user.Group)
	}

	review := entity.TeacherReview{
		TeacherID: request.TeacherID,
		UserID:    request.UserID,
		Comment:   strings.TrimSpace(request.Comment),
		Course:    course,
		Stars:     request.Stars,
	}
	if err := uc.TeacherReviewRepository.Create(tx, &review); err != nil {
		uc.Log.Errorf("Failed to create review : %+v", err)
		return fiber.NewError(500, "Ne udalos sozdat otziv")
	}

	if err := tx.Commit().Error; err != nil {
		uc.Log.Errorf("Failed to commit : %+v", err)
		return fiber.ErrInternalServerError
	}

	return nil
}

func (uc *ReviewUseCase) DeleteReview(ctx context.Context, request *model.DeleteReviewRequest) error {
	if err := uc.Validate.Struct(request); err != nil {
		uc.Log.Warnf("Invalid delete review request : %+v", err)
		return fiber.ErrBadRequest
	}

	var review entity.TeacherReview
	if err := uc.TeacherReviewRepository.FindByTeacherAndUser(uc.DB.WithContext(ctx), &review, request.TeacherID, request.UserID); err != nil {
		return fiber.NewError(404, "Otziv ne naiden")
	}

	tx := uc.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := uc.TeacherReviewRepository.DeleteByTeacherAndUser(tx, request.TeacherID, request.UserID); err != nil {
		uc.Log.Errorf("Failed to delete review : %+v", err)
		return fiber.ErrInternalServerError
	}

	if err := tx.Commit().Error; err != nil {
		uc.Log.Errorf("Failed to commit : %+v", err)
		return fiber.ErrInternalServerError
	}

	return nil
}

// LikeReview ставит или снимает голос на отзыв (toggle).
// is_like=true — лайк, is_like=false — дизлайк.
// Повторный запрос с тем же значением снимает голос.
func (uc *ReviewUseCase) LikeReview(ctx context.Context, request *model.LikeReviewRequest) error {
	if err := uc.Validate.Struct(request); err != nil {
		uc.Log.Warnf("Invalid like review request : %+v", err)
		return fiber.ErrBadRequest
	}

	var review entity.TeacherReview
	if err := uc.TeacherReviewRepository.FindById(uc.DB.WithContext(ctx), &review, request.ReviewID); err != nil {
		return fiber.NewError(404, "Otziv ne naiden")
	}

	existing, err := uc.ReviewLikeRepository.FindByReviewAndUser(uc.DB.WithContext(ctx), request.ReviewID, request.UserID)

	tx := uc.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err == nil {
		if existing.IsLike == request.IsLike {
			// Toggle off — снимаем голос
			if delErr := uc.ReviewLikeRepository.DeleteByReviewAndUser(tx, request.ReviewID, request.UserID); delErr != nil {
				uc.Log.Errorf("Failed to delete like : %+v", delErr)
				return fiber.ErrInternalServerError
			}
		} else {
			// Меняем голос на противоположный
			like := &entity.ReviewLike{ReviewID: request.ReviewID, UserID: request.UserID, IsLike: request.IsLike}
			if upsertErr := uc.ReviewLikeRepository.Upsert(tx, like); upsertErr != nil {
				uc.Log.Errorf("Failed to upsert like : %+v", upsertErr)
				return fiber.ErrInternalServerError
			}
		}
	} else {
		like := &entity.ReviewLike{ReviewID: request.ReviewID, UserID: request.UserID, IsLike: request.IsLike}
		if upsertErr := uc.ReviewLikeRepository.Upsert(tx, like); upsertErr != nil {
			uc.Log.Errorf("Failed to create like : %+v", upsertErr)
			return fiber.ErrInternalServerError
		}
	}

	if commitErr := tx.Commit().Error; commitErr != nil {
		uc.Log.Errorf("Failed to commit : %+v", commitErr)
		return fiber.ErrInternalServerError
	}

	return nil
}
