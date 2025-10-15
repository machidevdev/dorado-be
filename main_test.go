package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&User{})
	return db
}

func setupRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	r.POST("/users", func(c *gin.Context) {
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

	return r
}

func TestPostUserWithValidEmail(t *testing.T) {
	db := setupTestDB()
	router := setupRouter(db)

	userPost := UserPost{
		Email: "test@example.com",
	}
	jsonValue, _ := json.Marshal(userPost)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.Nil(t, err)
	assert.Equal(t, "user created", response["message"])

	// Verify user was actually created in database
	var user User
	result := db.Where("email = ?", "test@example.com").First(&user)
	assert.Nil(t, result.Error)
	assert.Equal(t, "test@example.com", user.Email)
}

func TestPostUserWithDuplicateEmail(t *testing.T) {
	db := setupTestDB()
	router := setupRouter(db)

	// Create initial user
	db.Create(&User{Email: "duplicate@example.com"})

	userPost := UserPost{
		Email: "duplicate@example.com",
	}
	jsonValue, _ := json.Marshal(userPost)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, 500, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.Nil(t, err)
	assert.Contains(t, response["error"], "UNIQUE")
}

func TestPostUserWithInvalidJSON(t *testing.T) {
	db := setupTestDB()
	router := setupRouter(db)

	invalidJSON := []byte(`{"email": }`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.Nil(t, err)
	assert.NotEmpty(t, response["error"])
}

func TestPostUserWithEmptyEmail(t *testing.T) {
	db := setupTestDB()
	router := setupRouter(db)

	userPost := UserPost{
		Email: "",
	}
	jsonValue, _ := json.Marshal(userPost)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.Nil(t, err)
	assert.Equal(t, "email cannot be empty", response["error"])
}

func TestPostUserWithMissingEmailField(t *testing.T) {
	db := setupTestDB()
	router := setupRouter(db)

	emptyJSON := []byte(`{}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(emptyJSON))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.Nil(t, err)
	assert.Equal(t, "email cannot be empty", response["error"])
}

func TestPostUserWithWhitespaceEmail(t *testing.T) {
	db := setupTestDB()
	router := setupRouter(db)

	userPost := UserPost{
		Email: "   ",
	}
	jsonValue, _ := json.Marshal(userPost)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.Nil(t, err)
	assert.Equal(t, "email cannot be empty", response["error"])
}

func TestPostUserWithInvalidEmailFormat(t *testing.T) {
	testCases := []struct {
		name  string
		email string
	}{
		{"missing @", "invalidemail.com"},
		{"missing domain", "user@"},
		{"missing local part", "@example.com"},
		{"no domain extension", "user@localhost"},
		{"multiple @", "user@@example.com"},
		{"invalid characters", "user name@example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupTestDB()
			router := setupRouter(db)

			userPost := UserPost{
				Email: tc.email,
			}
			jsonValue, _ := json.Marshal(userPost)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(jsonValue))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			assert.Equal(t, 400, w.Code, "Expected 400 for email: %s", tc.email)

			var response map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.Nil(t, err)
			assert.NotEmpty(t, response["error"])
		})
	}
}

func TestPostUserWithTooLongEmail(t *testing.T) {
	db := setupTestDB()
	router := setupRouter(db)

	// Create an email longer than 254 characters
	longEmail := strings.Repeat("a", 250) + "@example.com"

	userPost := UserPost{
		Email: longEmail,
	}
	jsonValue, _ := json.Marshal(userPost)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.Nil(t, err)
	assert.Equal(t, "email is too long (max 254 characters)", response["error"])
}

func TestPostUserWithEmailNormalization(t *testing.T) {
	db := setupTestDB()
	router := setupRouter(db)

	// Email with uppercase and whitespace should be normalized
	userPost := UserPost{
		Email: "  Test@Example.COM  ",
	}
	jsonValue, _ := json.Marshal(userPost)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/users", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	// Verify the email was normalized to lowercase and trimmed
	var user User
	result := db.Where("email = ?", "test@example.com").First(&user)
	assert.Nil(t, result.Error)
	assert.Equal(t, "test@example.com", user.Email)
}
