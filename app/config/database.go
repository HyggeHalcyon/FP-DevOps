package config

import (
	"fmt"
	"log"
	"os"

	"FP-DevOps/entity"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func RunExtension(db *gorm.DB) {
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";")
}

func SetUpDatabaseConnection() *gorm.DB {
	if os.Getenv("ENV") == "" {
		err := godotenv.Load(".env")
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")

	dsn := fmt.Sprintf("host=%v user=%v password=%v dbname=%v port=%v TimeZone=Asia/Jakarta", dbHost, dbUser, dbPass, dbName, dbPort)

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	RunExtension(db)

	if err := db.AutoMigrate(
		&entity.User{},
		&entity.File{},
	); err != nil {
		panic(err)
	}

	return db
}

func CloseDatabaseConnection(db *gorm.DB) {
	dbSQL, err := db.DB()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	dbSQL.Close()
}

func SetUpTestDatabaseConnection() *gorm.DB {
    dbURL := "file::memory:?cache=shared" // Menggunakan SQLite in-memory untuk tes

    db, err := gorm.Open(sqlite.Open(dbURL), &gorm.Config{
        // Opsional: Aktifkan logging untuk melihat query DB saat tes berjalan
        Logger: logger.Default.LogMode(logger.Info),
    })
    if err != nil {
        log.Fatalf("Failed to connect to test database: %v", err) // Menggunakan log.Fatalf
    }

    // AutoMigrate model-model yang dibutuhkan oleh tes
    err = db.AutoMigrate(&entity.User{}, &entity.File{}) // Pastikan ini model yang benar
    if err != nil {
        log.Fatalf("Failed to auto migrate test database: %v", err)
    }

    return db
}