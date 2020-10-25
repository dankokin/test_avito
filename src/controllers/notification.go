package controllers

import (
	"net/http"
	"test_avito/config"
	"test_avito/src/services"
	"test_avito/utils"
)

type EnvironmentNotification struct {
	Db  services.DatastoreNotification
	Scp Scrapper
}

// The main handler of the service. Accepts subscription requests
func (env *EnvironmentNotification) SubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	// Taking the url from the address bar arguments and validate the correctness of the url
	url := r.URL.Query().Get("url")
	err := utils.CheckUrl(url)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Taking the email from the address bar arguments and validate the correctness of the email
	email := r.URL.Query().Get("email")
	err = utils.CheckEmail(email)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Making a request to the avito website to get the price
	// 400th error in case of a nonexistent link
	priceChan := make(chan config.GetPriceResponse, 1)
	go env.Scp.getPrice(url, priceChan)

	// Checking whether the user has confirmed the specified email
	authChan := make(chan bool, 1)
	go env.Db.IsAuthorized(email, authChan)

	// Checking the case when the same user sends a repeated url
	dupChan := make(chan bool, 1)
	go env.Db.IsDuplicate(email, url, dupChan)

	response := <-priceChan
	if response.Error != nil || response.Price == -1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	isDuplicate := <-dupChan
	isAuthorized := <-authChan

	// 400th error in case duplicate url
	if isDuplicate {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sub := config.Subscription{
		AccVerified: isAuthorized,
		Email:       email,
		Url:         url,
		Price:       response.Price,
	}

	// Saving subscription info to database
	// 500th error in case of internal database error
	err = env.Db.SaveSubscription(sub)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Do not sending a confirmation email if the user has already confirmed it
	if !isAuthorized {
		err = env.Db.RecordMailConfirm(email)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// Handler for user's email confirmation
func (env *EnvironmentNotification) ConfirmEmailHandler(w http.ResponseWriter, r *http.Request) {
	// Taking unique hash from the address bar arguments
	hash := r.URL.Query().Get("hash")

	// Confirm email or send a new email if the confirmation time has expired
	err := env.Db.Confirm(hash)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
