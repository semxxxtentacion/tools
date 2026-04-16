package main

import (
	"fmt"
	"log"
	"mirea-qr/internal/config"
	"mirea-qr/internal/entity"
	"mirea-qr/pkg/crypto"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.NewConfig()

	encryptor, err := crypto.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize encryptor: %v", err)
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Europe/Moscow",
		cfg.DatabaseHost,
		cfg.DatabaseUser,
		cfg.DatabasePassword,
		cfg.DatabaseDb,
		cfg.DatabasePort,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	var users []entity.User
	if err := db.Find(&users).Error; err != nil {
		log.Fatalf("Failed to fetch users: %v", err)
	}

	log.Printf("Found %d users to check", len(users))

	migrated := 0
	for _, user := range users {
		// Try to decrypt — if success, password is already encrypted
		_, err := encryptor.Decrypt(user.Password)
		if err == nil {
			continue
		}

		// Decrypt failed → password is plaintext, encrypt it
		encrypted, err := encryptor.Encrypt(user.Password)
		if err != nil {
			log.Printf("SKIP user %s: failed to encrypt: %v", user.Email, err)
			continue
		}

		if err := db.Model(&user).Update("password", encrypted).Error; err != nil {
			log.Printf("SKIP user %s: failed to update: %v", user.Email, err)
			continue
		}

		log.Printf("Migrated user %s", user.Email)
		migrated++
	}

	log.Printf("Done. Migrated %d / %d users.", migrated, len(users))
}
