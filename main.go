package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

var (
	REFRESH_TIME = 2 * time.Minute
	WEATHER_URI  = "https://api.open-meteo.com/v1/forecast?latitude=56.96751622220031&longitude=24.10503208521148&current=temperature_2m,relative_humidity_2m,apparent_temperature,is_day,precipitation,showers,snowfall,rain,weather_code,cloud_cover,wind_speed_10m,wind_direction_10m,wind_gusts_10m&timezone=Europe%2FMoscow&forecast_days=1"
	REDIS_KEY    = "riga-weather:data"
)

var (
	thisApiKey string
	redisUri   string
)

var (
	redisClient *redis.Client
	logger      *slog.Logger
)

func safeLoad(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("Environment variable '%s' not defined!", key))
	}

	return value
}

func init() {
	godotenv.Load()

	thisApiKey = safeLoad("THIS_API_KEY")
	redisUri = safeLoad("REDIS_URI")

	logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	redisClient = redis.NewClient(&redis.Options{
		Addr: redisUri,
	})
}

func failRequest(w http.ResponseWriter, r *http.Request, status int, why string) {
	logger.Info("request failed", "status", status, "error", why)
	w.WriteHeader(status)
	w.Write([]byte(why))
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		failRequest(w, r, http.StatusBadRequest, "invalid method")
		return
	}

	if r.Header.Get("x-api-key") != thisApiKey {
		failRequest(w, r, http.StatusUnauthorized, "Invalid API Key")
		return
	}

	var body []byte
	err := redisClient.Get(context.Background(), REDIS_KEY).Scan(&body)
	if err != nil {
		failRequest(w, r, http.StatusInternalServerError, "Failed pulling data")
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func pullWeather() {
	logger.Info("Pulling new weather...")
	req, err := http.NewRequest("GET", WEATHER_URI, nil)
	if err != nil {
		logger.Info("something went wrong while pulling weather (request creationb)", "error", err)
		return
	}

	req.Header.Set("User-Agent", "riga-weather/1.0")

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Info("something went wrong while pulling weather (request transfer)", "error", err)
		return
	}
	defer response.Body.Close()

	fullBody, err := io.ReadAll(response.Body)
	if err != nil {
		logger.Info("something went wrong while pulling weather (reading response body)", "error", err)
		return
	}

	redisClient.Set(context.Background(), REDIS_KEY, fullBody, 0)
	slog.Info("new weather pulled")
}

func weatherRefresher() {
	ticker := time.NewTicker(REFRESH_TIME)

	for {
		<-ticker.C
		pullWeather()
	}
}

func main() {
	pullWeather()
	go weatherRefresher()

	http.HandleFunc("/v1/get-weather", handleRequest)
	http.ListenAndServe(":8080", nil)
}
