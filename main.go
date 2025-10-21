package main

import (
	"log"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
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
		log.Println("ERROR: DATABASE_URL environment variable is not set")
		os.Exit(1)
	}

	db, err := gorm.Open(postgres.Open(db_url), &gorm.Config{})
	if err != nil {
		log.Printf("ERROR: Failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	// Auto-migrate database schema
	db.AutoMigrate(&User{})

	r := gin.Default()

	// CORS configuration - only allow requests from Vercel domain
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://dorado-waitlist.vercel.app"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Rate limiting configuration
	// 10 requests per minute per IP for global rate limiting
	rate := limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  10,
	}
	store := memory.NewStore()
	rateLimitMiddleware := mgin.NewMiddleware(limiter.New(store, rate))

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.GET("/users", func(c *gin.Context) {
		// Password protection for admin access
		const adminPassword = "Dorado2025!?"
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.JSON(401, gin.H{
				"error": "unauthorized: missing authorization header",
			})
			return
		}

		// Extract password from "Bearer <password>" format
		password := strings.TrimPrefix(authHeader, "Bearer ")
		if password == authHeader || password != adminPassword {
			c.JSON(401, gin.H{
				"error": "unauthorized: invalid password",
			})
			return
		}

		var users []User
		if err := db.Find(&users).Error; err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(200, users)
	})

	// Apply rate limiting to POST /users endpoint
	r.POST("/users", rateLimitMiddleware, func(c *gin.Context) {
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

	log.Println("Database connected successfully, starting server...")
	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
