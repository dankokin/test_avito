package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestSuccessConfirmHandler(t *testing.T) {
	scp, _, mock := NewTestData()

	hash := "$2a$04$oA1axCBmazUBEzNl0KjrHuVy.ssgX4oySz/MKZGdoUX5h7vim.TfG"
	row := mock.NewRows([]string{"email", "hash", "deadline"}).
		AddRow("d_kokin@inbox.ru", hash, time.Now().Add(time.Hour*100))

	mock.ExpectQuery("SELECT \\* FROM auth_confirmation").
		WithArgs(hash).WillReturnRows(row)
	mock.ExpectExec("UPDATE subscription").
		WithArgs("d_kokin@inbox.ru").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM auth_confirmation").
		WithArgs(hash).WillReturnResult(sqlmock.NewResult(1, 1))

	req, err := http.NewRequest("GET", "http://localhost/confirm"+"?hash="+hash, nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	env := EnvironmentNotification{
		Db:  scp.Db,
		Scp: scp,
	}
	confirmHandler := env.ConfirmEmailHandler

	confirmHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestConfirmBadRequest(t *testing.T) {
	scp, _, mock := NewTestData()

	hash := "$2a$04$oA1axCBmazUBEzNl0KjrHuVy.ssgX4oySz/MKZGdoUX5h7vim.TfG"
	row := mock.NewRows([]string{"email", "hash", "deadline"}).
		AddRow("d_kokin@inbox.ru", hash, "bad date")

	mock.ExpectQuery("SELECT \\* FROM auth_confirmation").
		WithArgs(hash).WillReturnRows(row)
	mock.ExpectExec("UPDATE subscription").
		WithArgs("d_kokin@inbox.ru").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM auth_confirmation").
		WithArgs(hash).WillReturnResult(sqlmock.NewResult(1, 1))

	req, err := http.NewRequest("GET", "http://localhost/confirm"+"?hash="+hash, nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	env := EnvironmentNotification{
		Db:  scp.Db,
		Scp: scp,
	}
	confirmHandler := env.ConfirmEmailHandler

	confirmHandler(w, req)

	// scan error case
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSubscriptionHandlerBadURLRequest(t *testing.T) {
	scp, _, _ := NewTestData()
	req, err := http.NewRequest("POST", "http://localhost/"+"?url=badurl", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	env := EnvironmentNotification{
		Db:  scp.Db,
		Scp: scp,
	}

	subscriptionHandler := env.SubscriptionHandler
	subscriptionHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubscriptionHandlerBadEmailRequest(t *testing.T) {
	scp, _, _ := NewTestData()
	req, err := http.NewRequest("POST", "http://localhost/"+"?url=avito.ru&email=badEmail", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	env := EnvironmentNotification{
		Db:  scp.Db,
		Scp: scp,
	}

	subscriptionHandler := env.SubscriptionHandler
	subscriptionHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubscriptionHandlerBadExistingLinkRequest(t *testing.T) {
	scp, _, _ := NewTestData()
	req, err := http.NewRequest("POST", "http://localhost/"+"?url=vk.com&email=d_kokin@inbox.ru", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	env := EnvironmentNotification{
		Db:  scp.Db,
		Scp: scp,
	}

	subscriptionHandler := env.SubscriptionHandler
	subscriptionHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubscriptionHandlerAuthInternalError(t *testing.T) {
	scp, testServer, mock := NewTestData()
	req, err := http.NewRequest("POST", "http://localhost/"+
		"?url="+testServer.URL+"&email=d_kokin@inbox.ru", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	env := EnvironmentNotification{
		Db:  scp.Db,
		Scp: scp,
	}

	mock.ExpectQuery("SELECT DISTINCT acc_verified").
		WithArgs("d_kokin@inbox.ru").
		WillReturnError(errors.New("internal error"))

	subscriptionHandler := env.SubscriptionHandler
	subscriptionHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSubscriptionHandlerDuplicateInternalError(t *testing.T) {
	scp, testServer, mock := NewTestData()
	req, err := http.NewRequest("POST", "http://localhost/"+
		"?url="+testServer.URL+"&email=d_kokin@inbox.ru", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	env := EnvironmentNotification{
		Db:  scp.Db,
		Scp: scp,
	}

	verifiedRow := mock.NewRows([]string{"acc_verified"}).
		AddRow(true)

	mock.ExpectQuery("SELECT DISTINCT acc_verified").
		WithArgs("d_kokin@inbox.ru").
		WillReturnRows(verifiedRow)

	mock.ExpectQuery("SELECT url FROM subscription").
		WithArgs("d_kokin@inbox.ru", testServer.URL).
		WillReturnError(errors.New("internal error"))

	subscriptionHandler := env.SubscriptionHandler
	subscriptionHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSubscriptionHandlerSaveSubError(t *testing.T) {
	scp, testServer, mock := NewTestData()
	req, err := http.NewRequest("POST", "http://localhost/"+
		"?url="+testServer.URL+"&email=d_kokin@inbox.ru", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	env := EnvironmentNotification{
		Db:  scp.Db,
		Scp: scp,
	}

	verifiedRow := mock.NewRows([]string{"acc_verified"}).
		AddRow(true)

	mock.ExpectQuery("SELECT DISTINCT acc_verified").
		WithArgs("d_kokin@inbox.ru").
		WillReturnRows(verifiedRow)

	notDuplicateRow := mock.NewRows([]string{"url"}).
		AddRow("otherUrl")

	mock.ExpectQuery("SELECT url FROM subscription").
		WithArgs("d_kokin@inbox.ru", testServer.URL).
		WillReturnRows(notDuplicateRow)

	mock.ExpectExec("INSERT INTO subscription").
		WithArgs(true, "d_kokin@inbox.ru", 8792009, testServer.URL).
		WillReturnError(errors.New("internal error"))

	subscriptionHandler := env.SubscriptionHandler
	subscriptionHandler(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSubscriptionHandlerStatusOK(t *testing.T) {
	scp, testServer, mock := NewTestData()
	req, err := http.NewRequest("POST", "http://localhost/subscribe"+
		"?url="+testServer.URL+"&email=d_kokin@inbox.ru", nil)
	assert.Nil(t, err)

	w := httptest.NewRecorder()
	env := EnvironmentNotification{
		Db:  scp.Db,
		Scp: scp,
	}

	VerifiedRow := mock.NewRows([]string{"acc_verified"}).
		AddRow(true)

	mock.ExpectQuery("SELECT DISTINCT acc_verified").
		WithArgs("d_kokin@inbox.ru").
		WillReturnRows(VerifiedRow)

	mock.ExpectExec("INSERT INTO subscription").
		WithArgs(true, "d_kokin@inbox.ru", 8792009, testServer.URL).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery("SELECT DISTINCT url FROM subscription").
		WithArgs("d_kokin@inbox.ru", testServer.URL).
		WillReturnError(errors.New("internal error"))

	subscriptionHandler := env.SubscriptionHandler
	subscriptionHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
