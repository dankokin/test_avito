package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"test_avito/config"
	"test_avito/src/controllers"
	"test_avito/src/services"
)

var (
	pathToConfig = "./config/config.yml"
	pathToScheme = "./src/db/init.sql"
)

func main() {
	var conf config.Config
	conf.LoadFromYaml(pathToConfig)

	db := services.ConnectToDB(conf)

	services.Setup(pathToScheme, db)
	log.Println("Database is ready")

	scp := controllers.NewScrapper(db, conf)
	env := controllers.EnvironmentNotification{
		Db:  db,
		Scp: scp,
	}

	r := mux.NewRouter()

	r.HandleFunc("/subscribe", env.SubscriptionHandler).Methods("POST")
	r.HandleFunc("/confirm", env.ConfirmEmailHandler).Methods("GET")

	go env.Scp.Start()
	log.Println("scrapper is launched")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", conf.Server.Port), r))
}
