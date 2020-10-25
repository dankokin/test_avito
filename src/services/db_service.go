package services

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
	"os"
	"time"

	"test_avito/config"
)

// structure for functions that access the database
type DB struct {
	*sql.DB
}

// Preparing an expression for connecting to the database
func ReadDatabaseSettings(conf config.Config) string {
	DbDriver := conf.DataBase.Driver
	DbUsername := conf.DataBase.Username
	DbPassword := conf.DataBase.Password
	DbHost := conf.DataBase.Host
	DbPort := conf.DataBase.Port
	DbName := conf.DataBase.Name
	DbSslMode := conf.DataBase.SslMode

	return fmt.Sprintf("%s://%s:%s@%s:%s/%s?sslmode=%s",
		DbDriver, DbUsername, DbPassword, DbHost, DbPort, DbName, DbSslMode)
}

// Creating new database
func NewDB(conf config.Config) (*DB, error) {
	dbSourceName := ReadDatabaseSettings(conf)
	db, err := sql.Open("postgres", dbSourceName)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	log.Println("Successfully connected!")
	return &DB{db}, nil
}

func Setup(filename string, db *DB) {
	file, err := os.Open(filename)
	if err != nil {
		log.Println("Setupfile opening error: ", err)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		log.Println("Error after opening setupfile: ", err)
		return
	}

	bs := make([]byte, stat.Size())
	_, err = file.Read(bs)
	if err != nil {
		log.Println("Error after opening setupfile: ", err)
		panic(err)
	}

	command := string(bs)
	_, err = db.Exec(command)
	if err != nil {
		log.Println("Command error")
	}
}

func ConnectToDB(conf config.Config) *DB {
	var db *DB
	chanDB := make(chan *DB, 1)
	timeout := time.After(time.Second * 15)
	go func() {
		for {
			db, err := NewDB(conf)
			if err != nil {
				log.Println(err)
				time.Sleep(time.Millisecond * 1500)
			} else {
				chanDB <- db
				return
			}
		}
	}()

MAIN:
	for {
		select {
		case database := <-chanDB:
			db = database
			log.Println("Connected to database")
			break MAIN
		case <-timeout:
			log.Println("Timout: connection was not established")
			panic("timeout")
		}
	}
	return db
}
