package worker

import (
	"math/rand"
	"mirea-qr/internal/config"
	"mirea-qr/internal/entity"
	"mirea-qr/pkg/crypto"
	"mirea-qr/pkg/mirea"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type SessionWarmer struct {
	DB        *gorm.DB
	Redis     *redis.Client
	Config    *config.Config
	Encryptor *crypto.Encryptor
	Log       *logrus.Logger
}

func NewSessionWarmer(db *gorm.DB, redis *redis.Client, cfg *config.Config, encryptor *crypto.Encryptor, log *logrus.Logger) *SessionWarmer {
	return &SessionWarmer{
		DB:        db,
		Redis:     redis,
		Config:    cfg,
		Encryptor: encryptor,
		Log:       log,
	}
}

// Start запускается один раз при старте приложения
func (w *SessionWarmer) Start() {
	w.Log.Info("[WARMER] Background Session Warmer initialized")
	go func() {
		// Жестко фиксируем Московское время (UTC+3)
		msk := time.FixedZone("MSK", 3*60*60)

		for {
			now := time.Now().In(msk)

			// Устанавливаем целевые точки: 03:00 и 07:00 утра по МСК
			t1 := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, msk)
			t2 := time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, msk)

			var nextRun time.Time

			if now.Before(t1) {
				nextRun = t1
			} else if now.Before(t2) {
				nextRun = t2
			} else {
				// Если время уже больше 07:00, ждем до 03:00 следующего дня
				nextRun = t1.Add(24 * time.Hour)
			}

			sleepDuration := nextRun.Sub(now)
			w.Log.Infof("[WARMER] Sleeping for %v until next warmup at %v (MSK)", sleepDuration, nextRun.Format("15:04:05"))

			// Воркер засыпает до нужного времени
			time.Sleep(sleepDuration)

			// Просыпается и выполняет прогрев
			w.warmup()
		}
	}()
}

func (w *SessionWarmer) warmup() {
	w.Log.Info("[WARMER] Starting massive session refresh...")
	var users []entity.User
	if err := w.DB.Find(&users).Error; err != nil {
		w.Log.Errorf("[WARMER] Failed to fetch users: %v", err)
		return
	}

	successCount := 0
	for _, u := range users {
		decryptedPassword, err := w.Encryptor.Decrypt(u.Password)
		if err != nil {
			continue
		}
		u.Password = decryptedPassword

		attendance := mirea.NewAttendance(w.Config, u, w.Redis)
		
		// Принудительно вызываем авторизацию для обновления куков в Redis
		err = attendance.Authorization()
		if err != nil {
			w.Log.Warnf("[WARMER] Failed to refresh session for %s: %v", u.Email, err)
		} else {
			successCount++
		}

		// Задержка от 2 до 4 секунд, чтобы прокси не задохнулся
		time.Sleep(time.Duration(2000+rand.Intn(2000)) * time.Millisecond)
	}

	w.Log.Infof("[WARMER] Refresh cycle finished. Successfully updated %d/%d sessions.", successCount, len(users))
}
