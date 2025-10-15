package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type User struct {
	ID        uint
	Email     string `gorm:"unique"`
	CreatedAt time.Time
}

func main() {
	db_url := "postgresql://postgres:mRCoNoOHAXqjsxxUuMfAbtiMTPGLcWjD@caboose.proxy.rlwy.net:57748/railway"
	db, err := gorm.Open(postgres.Open(db_url), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	// Drop and recreate table (for testing only - remove before production!)
	db.AutoMigrate(&User{})

	result := db.Create(&User{Email: "bibibi"})
	if result.Error != nil {
		log.Printf("error: %v\n", result.Error)
	}
	log.Println("Saved userr bibibi")
	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
