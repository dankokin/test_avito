package config

import "time"

// Structure is necessary to confirm your subscription to update the prices
type AuthConfirmation struct {
	Email    string
	Hash     string
	Deadline time.Time
}

// Scrapper options
type Scrapper struct {
	WorkerCount            int   `yaml:"worker_count"`
	ScrapperTimeout        int64 `yaml:"timeout"`
	PageDownloadingTimeout int64 `yaml:"page_timeout"`
}

// DataBase options
type DataBase struct {
	Driver   string `yaml:"driver"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Name     string `yaml:"name"`
	SslMode  string `yaml:"ssl_mode"`
}

// Server options
type Server struct {
	Port int `yaml:"port"`
}

// Main structure for the service
type Subscription struct {
	AccVerified bool
	Email       string
	Price       int
	Url         string
}

// Convenient structure for launching the service
type Config struct {
	Scrapper `yaml:"crawler"`
	DataBase `yaml:"data_base"`
	Server   `yaml:"server"`
}

// Convenient structure for checking price updates
type CheckPriceRequest struct {
	OldPrice int
	Url      string
}

type GetPriceResponse struct {
	Price int
	Error error
}
