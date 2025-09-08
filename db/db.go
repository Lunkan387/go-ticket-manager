package db

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `gorm:"unique"`
	Password string
}

type Ticket struct {
	gorm.Model
	Title       string
	Description string
	User        string
	State       string
	ClosedAt    time.Time
	Priority    string
}

func InitDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("tickets.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&User{}, &Ticket{})
	return db, nil
}

func HashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}

func CheckPassword(hash string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func CheckUser(db *gorm.DB, username string) bool {
	var user User
	result := db.Where("username = ?", username).First(&user)
	return result.RowsAffected > 0
}

func TestDB() {
	db, err := InitDB()
	if err != nil {
		log.Fatal("Erreur DB :", err)
	}

	db.AutoMigrate(&User{})

	user := User{
		Username: "bob",
		Password: HashPassword("secret"),
	}

	if !CheckUser(db, user.Username) {
		result := db.Create(&user)
		if result.Error != nil {
			log.Fatal("Erreur ajout : ", result.Error)
		}
		fmt.Println("Utilisateur ajouté avec ID :", user.ID)
	} else {
		fmt.Println("Utilisateur existe déjà :", user.Username)
	}

	loginPassword := "secret"
	if CheckPassword(user.Password, loginPassword) {
		fmt.Println("✅ Mot de passe correct")
	} else {
		fmt.Println("❌ Mot de passe incorrect")
	}
}
