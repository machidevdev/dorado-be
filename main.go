package main

import (
	"log"
	"net/mail"
	"os"
	"strings"
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

type UserPost struct {
	Email string `json:"email"`
}

// validateEmail performs comprehensive email validation
func validateEmail(email string) (string, error) {
	// Trim whitespace
	email = strings.TrimSpace(email)

	// Check if empty
	if email == "" {
		return "", &ValidationError{Field: "email", Message: "email cannot be empty"}
	}

	// Check length constraints
	if len(email) > 254 {
		return "", &ValidationError{Field: "email", Message: "email is too long (max 254 characters)"}
	}

	// Parse email using Go's standard library
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "", &ValidationError{Field: "email", Message: "invalid email format"}
	}

	// Extract the email part (in case name was provided like "John Doe <john@example.com>")
	email = addr.Address

	// Split email into local and domain parts
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "", &ValidationError{Field: "email", Message: "invalid email format"}
	}

	localPart := parts[0]
	domain := parts[1]

	// Validate local part
	if len(localPart) == 0 || len(localPart) > 64 {
		return "", &ValidationError{Field: "email", Message: "email local part is invalid"}
	}

	// Validate domain part
	if len(domain) == 0 || len(domain) > 255 {
		return "", &ValidationError{Field: "email", Message: "email domain is invalid"}
	}

	// Check for at least one dot in domain
	if !strings.Contains(domain, ".") {
		return "", &ValidationError{Field: "email", Message: "email domain must contain at least one dot"}
	}

	// Convert to lowercase for consistency
	email = strings.ToLower(email)

	return email, nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func main() {
	db_url := os.Getenv("DATABASE_URL")
	if db_url == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}
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

	r.POST("/users", func(c *gin.Context) {
		// read user from request body
		var user UserPost
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		// validate and normalize email
		validatedEmail, err := validateEmail(user.Email)
		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		// create user with validated email
		result := db.Create(&User{Email: validatedEmail})
		if result.Error != nil {
			c.JSON(500, gin.H{
				"error": result.Error.Error(),
			})
			return
		}

		c.JSON(200, gin.H{
			"message": "user created",
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
