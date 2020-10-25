# Схема работы сервиса

####Формальная схема реализованного сервиса:
![scheme](https://github.com/dankokin/test_avito/pictures/app_scheme.png)

Как следует из схемы, сервис можно условно разделить на несколько частей:
* Endpoints
* Controllers
* Scrapper
* Services
* Database PostgreSQL

Пройдемся по каждому из них. Всего реализовано 2 эндпоинта:
* ```/subscribe``` - основной эндпоинт сервиса, принимающий в качестве параметров ```url``` объявления и ```email```, на который необходимо
высылать уведомления об изменении стоимости товара

* ```/confirm``` - эндпоинт, необходимый для подтверждения почты пользователя, ожидающий уникальный и заранее сгенерированный
хэш (```hash```)

При обращении к эндпоинту, вызывается функция-контроллер. SubscriptionHandler и ConfirmEmailHandler соответственно
#####Фрагмент кода, реализующий задачу подписки на изменение цены
```go

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
```
Подробнее см. в ```src/controllers```
При первой попытке подписаться на уведомления, в таблице ```auth_confirmation``` появляется запись
о новом пользователе с полями:
* ```email```
* ```hash```
* ```deadline```

Где ```email``` - почта пользователя, на которую необходимо высылать уведомления, ```hash``` - уникальное
значение для подтверждения почты, ```deadline``` - время, до которого нужно подтвердить почту.
После этого пользователь получает сообщение о необходимости подтвердить указанную почту.
Для этого ему необходимо перейти по сгенерированной ссылке, в которой одним из аргументов
в адресной строке будет уникальный ```hash```. Таким образом, пользователь обращается к заранее
подготовленному эндпоинту. Если указанный в аргументах ```hash``` совпадает и ```deadline```
не истек, то почта считается подтвержденной.

####Отслеживание изменение стоимости товара
Для того чтобы решить эту задачу я реализовал скраппер, который запускается в отдельной горутине
параллельно серверу.
Скраппер можно настроить с помощью конфига см. ```config/config.yml``` а именно:
* Настроить количество потоков, используемых скраппером для ускорения работы
* Пауза, спустя которую скраппер будет проверять обновление цены на сайте Авито
* Максимальное время на запрос к сайту Авито

#####Фрагмент кода, отслеживающий изменение стоимости товара:
```go
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
```

Обращаю внимание на то, что скраппер работает только с уникальными ссылками на объявления, для
того, чтобы не проверять лишний раз одно и то же объявление. Если же цена изменилась, то
скраппер запрашивает у базы данных все почтовые ящики, которые подписаны на данное объявление и 
рассылает уведомления.

#####Фрагмент кода, решающий задачу отправки уведомлений на почту:
```go
const (
	sendMessage = "\nThe price of your item has changed!\nSee here: %s"
)

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

```

####Работа с БД
Для работы с базой данных я реализовал ряд функций-сервисов. Я использую связку controller-service
для минимальной зависимости сервера от базы данных, так как между ними есть "прослойка". База
данных инкапсулирована, а контроллеры обращаются к бд по предоставленному интерфейсу.
Вот лишь некоторые функции для работы с бд, подробнее см. ```src/services```
Схема бд см. в ```src/db/init.sql```
```go
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
```

#### Тестирование и Docker
Написаны unit-тесты, покрывающие 69,2 % кода, реализующего контроллеры.

Также реализована сборка сервиса с помощью Docker.

#### Запуск приложения
Для того, чтобы запустить сервис, необходимо экспортировать 2 переменные окружения - почту
для рассылки уведомлений и пароль от нее:
```
$ export service_mail=example@gmail.com
$ export password=example_password
```

После этого, можно отрегулировать конфигурацию скраппера и следующими командами запустить сервис:
```
$ docker-compose build
$ docker-compose up
```
