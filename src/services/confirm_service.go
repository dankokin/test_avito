package services

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"net/smtp"
	"os"
	"time"

	"test_avito/config"
)

const (
	msgConst = "\nFrom :%s\nTo: %s\nPlease confirm your email: %s"
	url      = "\n127.0.0.1:8080/confirm?hash="
)

func addressGenerator(email string) (str string) {
	hashedLogin, _ := bcrypt.GenerateFromPassword([]byte(email), 4)
	return string(hashedLogin)
}

// True if email subscribed on this url
func (db *DB) IsDuplicate(email string, url string, dupChan chan bool) {
	defer close(dupChan)
	row := db.QueryRow("SELECT DISTINCT url FROM subscription where email = $1 AND url = $2", email, url)

	var duplicate string
	err := row.Scan(&duplicate)
	if err != nil {
		dupChan <- false
		return
	}
	dupChan <- true
}

// True if email is verified (acc_verified == true)
func (db *DB) IsAuthorized(email string, authChan chan bool) {
	defer close(authChan)
	row := db.QueryRow("SELECT DISTINCT acc_verified FROM subscription where email = $1", email)

	var isAuthorized bool
	err := row.Scan(&isAuthorized)
	if err != nil {
		authChan <- false
		return
	}
	authChan <- true
}

// Creating a new email waiting for confirmation
func (db *DB) RecordMailConfirm(email string) error {
	secret := addressGenerator(email)
	deadlineTime := time.Now().Add(24 * time.Hour)
	_, err := db.Exec("INSERT INTO auth_confirmation (email, hash, deadline) values ($1, $2, $3)",
		email, secret, deadlineTime)
	if err != nil {
		return err
	}

	// Sending to user message with confirmation link
	err = db.sendMail(email)
	if err != nil {
		return err
	}
	return nil
}

// Function which update auth_confirmation if the confirmation time has expired
func (db *DB) confirmFieldUpdate(email string, hash string) (err error) {
	_, err = db.Exec("UPDATE auth_confirmation SET hash = $1, deadline = $2 where email = $3",
		hash, time.Now().Add(24*time.Hour), email)
	return err
}

// Function that sends a message to the user at the specified email address
func (db *DB) sendMail(email string) error {
	row := db.QueryRow("SELECT * FROM auth_confirmation WHERE email = $1", email)

	var obj config.AuthConfirmation
	err := row.Scan(&obj.Email, &obj.Hash, &obj.Deadline)
	if err != nil {
		return err
	}

	serviceMail, _ := os.LookupEnv("service_mail")
	password, _ := os.LookupEnv("password")

	msg := fmt.Sprintf(msgConst, serviceMail, email, url+obj.Hash)

	err = smtp.SendMail("smtp.gmail.com:587",
		smtp.PlainAuth(
			"",
			serviceMail,
			password,
			"smtp.gmail.com"),
		serviceMail, []string{email}, []byte(msg))

	if err != nil {
		return err
	}
	return nil
}

// Function which confirm email or send a new email if the confirmation time has expired
func (db *DB) Confirm(hash string) error {
	var authInfo config.AuthConfirmation
	row := db.QueryRow("SELECT * FROM auth_confirmation WHERE hash = $1", hash)
	err := row.Scan(&authInfo.Email, &authInfo.Hash, &authInfo.Deadline)
	if err != nil {
		return err
	}

	if authInfo.Deadline.Before(time.Now()) {
		newHash := addressGenerator(authInfo.Email)
		err = db.confirmFieldUpdate(authInfo.Email, newHash)
		if err != nil {
			return err
		}
		err = db.sendMail(authInfo.Email)
		return err
	} else {
		_, err = db.Exec("UPDATE subscription SET acc_verified = true where email = $1", authInfo.Email)
		if err != nil {
			return err
		}
		_, err = db.Exec("DELETE FROM auth_confirmation WHERE hash = $1", hash)
		return err
	}
}
