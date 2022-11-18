package main

import (
	"fmt"
	"net/http"
	"encoding/json"
	"io/ioutil"
	"strings"
	"html"
//	"strconv"
	"time"
	"unicode/utf8"
	"os/exec"
	"crypto/md5"
	"github.com/hashicorp/golang-lru/v2"
//	"os"
)

type userBaseResponse struct {
	CoinPrices struct {
		Btc string `json:"btc"` 
		Xmr string `json:"xmr"`
		CoinCode string `json:"coin_code"`
	} `json:"coins"`

	Weather struct {
		SunMoon string `json:"sun_moon"`
		Day int `json:"day"`
		Month int `json:"month"`
		Year int `json:"year"`
		HumLowHigh [3]string `json:"hum_low_high"`
		Location string `json:"location"`
	} `json:"weather"`

	CurrPrices struct {
		Code []string `json:"code"`
		Volume []string `json:"volume"`
		Value []string `json:"value"`
		CoinCode string `json:"coin_code"`
		Date string `json:"date"`
	} `json:"currs"`
}
type cacheRecord struct {
	value string
	expiry time.Time

}
const CACHESIZE int = 10000
const HASHSIZE int = 16
var cache, _ = lru.New[[HASHSIZE]byte, cacheRecord](CACHESIZE)

var baseResp userBaseResponse
var coinPrices = &baseResp.CoinPrices
var weather = &baseResp.Weather

type userBaseRequest struct {
	CoinCode string `json:"coin_code"`	
	Param	 string `json:"param"`
	Location string `json:"location"`
}

func main() {
	http.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/style.css")
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})

//	jsonFile, err := os.Open("testfile.json")
//	if err != nil {
//		fmt.Println(err)
//	}
//	fmt.Println("reading testfile.json")
//	defer jsonFile.Close()
//
//	byteValue, _ := ioutil.ReadAll(jsonFile)
//	var base userBaseRequest 
//	json.Unmarshal(byteValue, &base)
//	fmt.Println(baseRequest.CoinCode)
	http.HandleFunc("/base_info", base_handler)
	http.HandleFunc("/forecast_info", forecast_handler)
	http.ListenAndServe(":8900", nil)
}

func base_handler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
	}
	var baseRequest userBaseRequest
	json.Unmarshal(body, &baseRequest)

	if baseRequest.CoinCode  == "" { 
		baseRequest.CoinCode = "usd"
	}
	baseRequest.CoinCode = html.EscapeString(baseRequest.CoinCode)
	coinPrices.CoinCode = baseRequest.CoinCode

	coinPrices.Btc = get_crypto_curr(coinPrices.CoinCode, "btc") 
	coinPrices.Xmr = get_crypto_curr(coinPrices.CoinCode, "xmr")

	if len(coinPrices.CoinCode) > 1 && baseRequest.Param == "conversion" {
		raw, _ := json.Marshal(coinPrices)
		w.Write(raw)
	} else {
		baseRequest.Location = baseRequest.Location
		if baseRequest.Location == "" {
			baseRequest.Location = "Zdar"
		}
		get_weather(baseRequest.Location)
		get_currency_rates()
		raw, err := json.Marshal(&baseResp)
		if err != nil {
			fmt.Println(err)
		}
		w.Write(raw)
	}
//	var session http.Cookie
//	session.Name = "sessionid"
//	session.Domain = "michalkukla.xyz"
//	session.Path = "/startpage"
//	session.HttpOnly = true
//	session.Secure = true

	
}

func forecast_handler(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println(err)
	}
	var baseRequest userBaseRequest
	json.Unmarshal(body, &baseRequest)
	fmt.Println(baseRequest.CoinCode)
//	weatherFile, err := os.Open("weatherreport")
//	if err != nil {
//		fmt.Println(err)
//	}
//	defer weatherFile.Close()
//	byteWeather, _ := ioutil.ReadAll(weatherFile)
//	w.Write(byteWeather)
}
func get_weather(location string) {
	tm := time.Now()
	year, month, day := tm.Date()
	if weather.HumLowHigh[1] == "" || weather.Location != location || weather.Day != day ||
		time.Month(weather.Month) != month || weather.Year != year {

		weather.Day = day 
		weather.Month = int(month) 
		weather.Year = year
		weather.Location = location
//		weather.HumLowHigh[0], weather.HumLowHigh[1],weather.HumLowHigh[2]
	}

	get_forecast(location)
	weather.SunMoon = get_sun_moon_info(location)
}

func get_crypto_curr(coinCode, name string) string {
	signature := fmt.Sprintf(`%s:%s`, coinCode, name)
	cacheSignature := hash(signature)
	record, found := get(cacheSignature)
	var answer string = ""
	if found {
		now := time.Now()
		d := record.expiry
		d = d.Add(time.Minute * 5)

		if  record.value != "" && d.After(now) {
			answer = record.value
			return answer
		}
	}
	value := get_coin_price(coinCode, name)
	answer = store(cacheSignature, value)
	return answer
}

func get_sun_moon_info(location string) string {
	format := "%S+%s+%m"	
	signature := fmt.Sprintf(`%s:%s`, location, format)
	cacheSignature := hash(signature)
	var answer string = ""
	record, found := get(cacheSignature)	
	if found {
		yearNow, monthNow, dayNow := time.Now().Date()	
		year, month, day := record.expiry.Date()	
		if record.value != "" && dayNow == day && monthNow == month && yearNow == year {
			answer = record.value
			return answer
		}
	}
	value := get_weather_info(format, location)
	answer = store(cacheSignature, value)
	return answer
//	if info := get_weather_info(format, location); info != "" {
//		weather.SunMoon = info
//	}
//	return weather.SunMoon
}

func get_weather_info(format, location string) string {
	url := fmt.Sprintf(`https://wttr.in/%s?format="%s"`, location, format)
	reqm, err := http.NewRequest("GET", url, nil)

	if err != nil {
		fmt.Println(err)
		return ""
	}
	reqm.Header.Set("Content-Type", "text/html")
	content, err := http.DefaultClient.Do(reqm)

	if err != nil {
		fmt.Println(err)
		return ""
	}
	out, err := ioutil.ReadAll(content.Body)

	if err != nil {
		fmt.Println(err)
		return ""
	}
	str_out := strings.ReplaceAll(string(out), "\"", "")

	return string(str_out)
}

func get_forecast(location string) string {
	signature := fmt.Sprintf(`%s:%s`, location, "forecast")
	cacheSignature := hash(signature)
	var answer string = ""
	record, found := get(cacheSignature)

	if found {
		now := time.Now()
		d := record.expiry
		d = d.Add(time.Hour * 6)
		if record.value != "" && d.After(now) {
			answer = record.value
			return answer
		}
	}
	output, err := exec.Command("/bin/sh", "sb-forecast.sh", location).Output()
	if err != nil {
		fmt.Printf("error %s", err)
	}
	hum_low_high := strings.Replace(string(output), "\n", "", 1)

	output, err = exec.Command("/bin/sh", "sb-forecast.sh", location, "23", "26").Output()
	if err != nil {
		fmt.Printf("error %s", err)
	}
	hum_low_high_next := strings.Replace(string(output), "\n", "", 1)
	output, err = exec.Command("/bin/sh", "sb-forecast.sh", location, "33", "36").Output()
	if err != nil {
		fmt.Printf("error %s", err)
	}
	hum_low_high_next2 := strings.Replace(string(output), "\n", "", 1)

	weather.HumLowHigh[0] = hum_low_high
	weather.HumLowHigh[1] = hum_low_high_next
	weather.HumLowHigh[2] = hum_low_high_next2
	value := strings.Join([]string{hum_low_high, hum_low_high_next, hum_low_high_next2}, "\n")
	answer = store(cacheSignature, value)
	return answer

}

func exec_shellscript(shellscript ...string) string {
	cmd := shellscript[0]
	var param1 string
	var param2 string
	if shellscript[1] != "" {
		param1 = shellscript[1]	
	}
	if shellscript[2] != "" {
		param1 = shellscript[2]	
	}
	output, err := exec.Command("/bin/sh", cmd, param1, param2).Output()
	if err != nil {
		fmt.Printf("error %s", err)
	}
	outputStr := string(output)
	return outputStr
}

func get_coin_price(showRates, coinCode string) string {

	url := fmt.Sprintf(`https://%s.rate.sx/1%s`, showRates, coinCode)
	reqm, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(err)
		return "" 
	}
	reqm.Header.Set("Content-Type", "text/html")
	content, err := http.DefaultClient.Do(reqm)

	if err != nil {
		fmt.Println(err)
		return ""
	}
	price, err := ioutil.ReadAll(content.Body)

	if err != nil {
		fmt.Println(err)
		return ""
	}
	priceStr := string(price)
	priceStr = priceStr[:len(priceStr)-1]
	return priceStr
}
func get_currency_rates() {
	now := time.Now()
	var rates string = ""
	signature := "cnb-rates"
	cacheSignature := hash(signature)

	record, found := get(cacheSignature)

	if found {
		dateStr := string(now.Day())+"."+string(int(now.Month()))+"."+string(now.Year())
		d := strings.Split(record.value, " ")[0]
		
		if d == dateStr || now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
			rates = record.value
		} else {
			rates = getCnbRates()
			rates = store(cacheSignature, rates)
		}

	} else {
		rates = getCnbRates()
		rates = store(cacheSignature, rates)
	}
	exchRates := strings.Split(rates, "\n")


	gbpCurr := strings.Split(exchRates[33], "|")
	gbpValue := gbpCurr[len(gbpCurr)-1]
	gbpCode := gbpCurr[len(gbpCurr)-2]
	gbpVolume := gbpCurr[len(gbpCurr)-3]

	eurCurr := strings.Split(exchRates[7], "|")
	eurValue := eurCurr[len(eurCurr)-1]
	eurCode := eurCurr[len(eurCurr)-2]
	eurVolume := eurCurr[len(eurCurr)-3]

	usdCurr := strings.Split(exchRates[32], "|")
	usdValue := usdCurr[len(usdCurr)-1]
	usdCode := usdCurr[len(usdCurr)-2]
	usdVolume := usdCurr[len(usdCurr)-3]

	var currPrices = userBaseResponse{}.CurrPrices
	currPrices.Code = append(currPrices.Code, gbpCode)
	currPrices.Code = append(currPrices.Code, eurCode)
	currPrices.Code = append(currPrices.Code, usdCode)
	currPrices.Volume = append(currPrices.Volume, gbpVolume)
	currPrices.Volume = append(currPrices.Volume, eurVolume)
	currPrices.Volume = append(currPrices.Volume, usdVolume)
	currPrices.Value = append(currPrices.Value, gbpValue)
	currPrices.Value = append(currPrices.Value, eurValue)
	currPrices.Value = append(currPrices.Value, usdValue)
//	currPrices.Json = json
	currPrices.CoinCode = "czk"
	currPrices.Date = strings.Split(exchRates[0], " ")[0]

	baseResp.CurrPrices = currPrices	

}

func getCnbRates() string {
	url := "https://cnb.cz/cs/financni-trhy/devizovy-trh/kurzy-devizoveho-trhu/kurzy-devizoveho-trhu/denni_kurz.txt"
	reqm, _ := http.NewRequest("GET", url, nil)

	reqm.Header.Set("Content-Type", "text/html")
	content, err := http.DefaultClient.Do(reqm)

	if err != nil {
		fmt.Println(err)
		return ""
	}
	b, err := ioutil.ReadAll(content.Body)

	if err != nil {
		fmt.Println(err)
		return "" 
	}
	return string(b)
}

func store(signature [HASHSIZE]byte, value string) string {
	cache.Add(signature, cacheRecord{value, time.Now()})
	return value
} 

func get(signature [HASHSIZE]byte) (cacheRecord, bool) {
	record, found := cache.Get(signature)
	return record, found
}

func hash(signature string) [HASHSIZE]byte {
	return md5.Sum([]byte(signature))
}

func download_sat_images() {
	output, err := exec.Command("/bin/sh", "sat-img.sh").Output()
	fmt.Println(output)
	if err != nil {
		fmt.Printf("error %s", err)
	}

}

func condenseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateStrings(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for !utf8.ValidString(s[:n]) {
		n--
	}
	return s[:n]
}
