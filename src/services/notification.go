package services

import (
	"fmt"
	"net/smtp"
	"os"

	"test_avito/config"
)

const (
	sendMessage = "\nThe price of your item has changed!\nSee here: %s"
)

type DatastoreNotification interface {
	SaveSubscription(config.Subscription) error
	UpdateSubscription(subscription config.Subscription) error
	GetAllUniqueUrlsAndPrices(chan config.CheckPriceRequest) error
	GetEmailsByUrl(url string) ([]config.Subscription, error)
	SendMessages(subs []config.Subscription)

	Confirm(hash string) (err error)
	RecordMailConfirm(email string) (err error)

	IsAuthorized(email string, authChan chan bool)
	IsDuplicate(email string, url string, dupChan chan bool)
}

func (db *DB) SaveSubscription(subscription config.Subscription) error {
	_, err := db.Exec("INSERT INTO subscription (acc_verified, email, price, url) values ($1, $2, $3, $4)",
		subscription.AccVerified,
		subscription.Email,
		subscription.Price,
		subscription.Url)
	return err
}

func (db *DB) UpdateSubscription(subscription config.Subscription) error {
	_, err := db.Exec("UPDATE subscription SET acc_verified = $1, email = $2, price = $3, url = $4 WHERE email = $2 and url = $4",
		subscription.AccVerified,
		subscription.Email,
		subscription.Price,
		subscription.Url)
	return err
}

func (db *DB) GetAllUniqueUrlsAndPrices(pairChan chan config.CheckPriceRequest) error {
	rows, err := db.Query("SELECT DISTINCT url, price FROM subscription where acc_verified = true")
	defer rows.Close()
	if err != nil {
		return err
	}

	for rows.Next() {
		var pair config.CheckPriceRequest
		err = rows.Scan(&pair.Url, &pair.OldPrice)
		if err != nil {
			return err
		}
		pairChan <- pair
	}
	return nil
}

func (db *DB) GetEmailsByUrl(url string) ([]config.Subscription, error) {
	subs := make([]config.Subscription, 0, 8)

	rows, err := db.Query("SELECT acc_verified, email, price, url FROM subscription WHERE url = $1 and acc_verified = true", url)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var sub config.Subscription
		err = rows.Scan(&sub.AccVerified, &sub.Email, &sub.Price, &sub.Url)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

func (db *DB) SendMessages(subs []config.Subscription) {
	serviceMail, _ := os.LookupEnv("service_mail")
	password, _ := os.LookupEnv("password")

	for _, value := range subs {
		msg := fmt.Sprintf(sendMessage, value.Url)
		_ = smtp.SendMail("smtp.gmail.com:587",
			smtp.PlainAuth(
				"",
				serviceMail,
				password,
				"smtp.gmail.com"),
			serviceMail, []string{value.Email}, []byte(msg))
	}
}
