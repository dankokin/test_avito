package controllers

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"test_avito/config"
	"test_avito/src/services"
)

type Scrapper struct {
	Db          *services.DB
	Client      *http.Client
	WorkerCount int

	scrapperTimeout time.Duration
	requestTimeout  time.Duration
	pairChannel     chan config.CheckPriceRequest
}

// Creating a new scrapper according to the config
func NewScrapper(db *services.DB, cnf config.Config) Scrapper {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			MaxVersion: tls.VersionTLS12,
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(cnf.Scrapper.PageDownloadingTimeout) * time.Millisecond,
	}

	return Scrapper{
		Db:              db,
		Client:          client,
		scrapperTimeout: time.Minute * time.Duration(cnf.ScrapperTimeout),
		WorkerCount:     cnf.WorkerCount,
		pairChannel:     make(chan config.CheckPriceRequest, 512),
	}
}

// Function that starts the scrapper
func (scp *Scrapper) Start() {
	var wg sync.WaitGroup
	for {
		// Launching workers in different goroutines
		for i := 0; i < scp.WorkerCount; i++ {
			wg.Add(1)
			go scp.startWorker(&wg)
		}

		// Getting unique urls and prices from database for to transfer them to the workers
		err := scp.Db.GetAllUniqueUrlsAndPrices(scp.pairChannel)
		if err != nil {
			fmt.Println("Couldn't get links to ads")
		}
		close(scp.pairChannel)

		wg.Wait()
		time.Sleep(scp.scrapperTimeout)
		scp.pairChannel = make(chan config.CheckPriceRequest, 512)
	}
}

func (scp *Scrapper) startWorker(wg *sync.WaitGroup) {
	defer wg.Done()
	for pair := range scp.pairChannel {
		// Getting price from avito website
		chanPrice := make(chan config.GetPriceResponse, 1)
		go scp.getPrice(pair.Url, chanPrice)
		var productPrice int
		select {
		case value := <-chanPrice:
			if value.Error != nil {
				fmt.Printf("Error %s", value.Error)
				continue
			} else {
				productPrice = value.Price
			}
		case <-time.After(time.Millisecond * 3000):
			fmt.Printf("Link: %s is not available. Timeout", pair.Url)
			continue
		}

		if productPrice != pair.OldPrice {
			// Getting all subscribers for an ad that has changed its price
			subs, err := scp.Db.GetEmailsByUrl(pair.Url)
			if err != nil {
				fmt.Printf("Internal error, trying to get emails by url:%s", pair.Url)
				continue
			}

			// Sending a message about price changes
			scp.Db.SendMessages(subs)
			for _, value := range subs {
				value.Price = productPrice
				scp.Db.UpdateSubscription(value)
			}
		}
	}
}

func (scp *Scrapper) getPrice(url string, priceChan chan config.GetPriceResponse) {
	defer close(priceChan)
	response := config.GetPriceResponse{
		Price: -1,
		Error: nil,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		response.Error = err
		priceChan <- response
		return
	}

	resp, err := scp.Client.Do(req)
	if err != nil {
		response.Error = err
		priceChan <- response
		return
	}

	if resp.StatusCode != 200 {
		response.Error = errors.New("link is not available")
		priceChan <- response
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		response.Error = err
		priceChan <- response
		return
	}

	bodyString := string(body)
	priceStr := parsePrice(bodyString, `"dynx_price":`, ",")
	price, err := strconv.Atoi(priceStr)
	if err != nil {
		response.Error = err
		priceChan <- response
		return
	}

	response.Price = price
	priceChan <- response
}

func parsePrice(target string, begin string, end string) string {
	s := strings.Index(target, begin)
	if s == -1 {
		return ""
	}

	buffer := target[s+len(begin):]
	endIndex := strings.Index(buffer, end)
	if endIndex == -1 {
		return ""
	}

	result := buffer[:endIndex]
	return result
}
