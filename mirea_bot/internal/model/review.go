package model

// SearchTeacherRequest — запрос на поиск преподавателя по имени (подстрока).
type SearchTeacherRequest struct {
	Query string `json:"query" validate:"required,min=2,max=255"`
}

// AddTeacherRequest — запрос на добавление нового преподавателя.
type AddTeacherRequest struct {
	Name string `json:"name" validate:"required,min=2,max=255"`
}

// CreateReviewRequest — запрос на создание отзыва.
type CreateReviewRequest struct {
	UserID    string `json:"-"`
	TeacherID uint   `json:"teacher_id" validate:"required"`
	Comment   string `json:"comment" validate:"required,min=1,max=255"`
	Stars     int    `json:"stars" validate:"required,min=1,max=5"`
}

// DeleteReviewRequest — запрос на удаление своего отзыва.
type DeleteReviewRequest struct {
	UserID    string `json:"-"`
	TeacherID uint   `json:"teacher_id" validate:"required"`
}

// GetReviewsRequest — запрос на получение отзывов по преподавателю.
type GetReviewsRequest struct {
	TeacherID uint `json:"teacher_id" validate:"required"`
}

// LikeReviewRequest — запрос на лайк/дизлайк отзыва.
// IsLike: true = лайк, false = дизлайк. Повторный запрос с тем же значением — снимает голос.
type LikeReviewRequest struct {
	UserID   string `json:"-"`
	ReviewID uint   `json:"review_id" validate:"required"`
	IsLike   bool   `json:"is_like"`
}

// TeacherResponse — ответ с данными преподавателя.
type TeacherResponse struct {
	ID          uint    `json:"id"`
	Name        string  `json:"name"`
	ReviewCount int64   `json:"review_count"`
	AvgStars    float64 `json:"avg_stars"`
}

// ReviewResponse — ответ с данными отзыва (анонимный, без user_id).
type ReviewResponse struct {
	ID        uint   `json:"id"`
	Comment   string `json:"comment"`
	Course    int    `json:"course"`
	Stars     int    `json:"stars"`
	CreatedAt int64  `json:"created_at"`
	IsMine    bool   `json:"is_mine"`
	Likes     int64  `json:"likes"`
	Dislikes  int64  `json:"dislikes"`
	// MyVote: 0 — нет голоса, 1 — лайк, -1 — дизлайк
	MyVote int `json:"my_vote"`
}

// TeacherReviewsResponse — преподаватель + список отзывов.
type TeacherReviewsResponse struct {
	Teacher TeacherResponse  `json:"teacher"`
	Reviews []ReviewResponse `json:"reviews"`
}
