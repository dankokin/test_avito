package controllers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"

	"test_avito/config"
	"test_avito/src/services"
)

func NewTestData() (Scrapper, *httptest.Server, sqlmock.Sqlmock) {
	db, sqlMock, err := sqlmock.New()
	if err != nil {
		panic(err)
	}

	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, avitoHTML)
		w.WriteHeader(http.StatusOK)
	}))

	scp := Scrapper{
		Db:              &services.DB{DB: db},
		Client:          testServer.Client(),
		WorkerCount:     3,
		scrapperTimeout: 0,
		pairChannel:     make(chan config.CheckPriceRequest, 5),
	}
	return scp, testServer, sqlMock
}

func TestGetPrice(t *testing.T) {
	scp, testServer, _ := NewTestData()
	priceChan := make(chan config.GetPriceResponse, 1)
	scp.getPrice(testServer.URL, priceChan)
	value := <-priceChan
	assert.Equal(t, 8792009, value.Price)
	assert.Nil(t, value.Error)
}

func TestWorkerStart(t *testing.T) {
	scp, testServer, sqlMock := NewTestData()

	pairs := []config.CheckPriceRequest{config.CheckPriceRequest{
		OldPrice: 8792008,
		Url:      testServer.URL,
	}, config.CheckPriceRequest{
		OldPrice: 8792009,
		Url:      testServer.URL,
	}, config.CheckPriceRequest{
		OldPrice: 42,
		Url:      "bad link",
	}}

	rows := sqlMock.NewRows([]string{"acc_verified", "email", "price", "url"}).
		AddRow(true, "d_kokin@inbox.ru", 8792009, testServer.URL)

	sqlMock.ExpectQuery(
		"SELECT acc_verified, email, price, url FROM subscription").
		WithArgs(testServer.URL).WillReturnRows(rows)

	for _, value := range pairs {
		scp.pairChannel <- value
	}
	close(scp.pairChannel)

	var wg sync.WaitGroup
	for i := 0; i < scp.WorkerCount; i++ {
		wg.Add(1)
		go scp.startWorker(&wg)
	}
	wg.Wait()
}

func TestFuzzConstructor(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}

	var cnf config.Config
	for i := 0; i < 100; i++ {
		f := fuzz.New().NilChance(0)
		f.Fuzz(&cnf)

		scp := NewScrapper(&services.DB{DB: db}, cnf)

		assert.NotNil(t, scp.Db)
		assert.NotNil(t, scp.Client)
	}
}

const avitoHTML = `window.dataLayer = [{"dynx_user":"a","dynx_region":"moskva","dynx_prodid":1791027290,"dynx_price":8792009,"dynx_category":"avtomobili","dynx_vertical":0,"dynx_pagetype":"item"},{"pageType":"Item","itemID":1791027290,"vertical":"AUTO","categoryId":9,"categorySlug":"avtomobili","microCategoryId":23191,"locationId":637640,"isShop":1,"isClientType1":1,"itemPrice":8792009,"withDelivery":1,"brand":"BMW","model":"M5","year":"2019","body_type":"Седан","kolichestvo_dverey":"4","generation":"F90 (2017—н. в.)","engine_type":"Бензин","drive":"Полный","transmission":"Автомат","max_discount":480000,"tradein_discount":150000,"credit_discount":300000,"insurance_discount":30000,"upravlenie_klimatom":"климат-контроль однозонный","salon":"кожа","fary":"светодиодные","vehicle_type":"Новые","capacity":"600 л.с.","engine":"4.4","color":"Чёрный","wheel":"Левый","isNewAuto":1}];
 (function(w, d, s, l, i) {
 w[l] = w[l] || [];
 w[l].push({
 'gtm.start': new Date().getTime(),
 event: 'gtm.js'
 });
 var f = d.getElementsByTagName(s)[0],
 j = d.createElement(s),
 dl = l != 'dataLayer' ? '&l=' + l : '';
 j.async = true;
 j.src = '//www.googletagmanager.com/gtm.js?id=' + i + dl;
 f.parentNode.insertBefore(j, f);
 })(window, document, 'script', 'dataLayer', 'GTM-KP9Q9H');`
