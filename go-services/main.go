package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	metricglobal "go.opentelemetry.io/otel/metric/global"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var (
	db            *sql.DB
	rdb           *redis.Client
	ctx           = context.Background()
	requestMetric metric.Int64Counter
)

func main() {
	initLog()
	initTracer()
	initMetrics()
	initDB()
	initRedis()

	http.HandleFunc("/healthz", healthHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/products", productsHandler)

	log.Println(`{"level":"info","msg":"Go service started on :8080"}`)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf(`{"level":"fatal","msg":"Failed to start server","error":"%v"}`, err)
	}
}

func initLog() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
}

func initTracer() {
	exporter, err := stdouttrace.New()
	if err != nil {
		log.Fatalf(`{"level":"fatal","msg":"Failed to initialize tracer","error":"%v"}`, err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(tp)
}

func initMetrics() {
	exporter, err := prometheus.New()
	if err != nil {
		log.Fatalf(`{"level":"fatal","msg":"Failed to initialize prometheus exporter","error":"%v"}`, err)
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	metricglobal.SetMeterProvider(provider)

	http.Handle("/metrics", exporter)

	meter := metricglobal.Meter("go-service")
	requestMetric, err = meter.Int64Counter("http_requests_total")
	if err != nil {
		log.Fatalf(`{"level":"fatal","msg":"Failed to create metric","error":"%v"}`, err)
	}
}

func initDB() {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf(`{"level":"fatal","msg":"Failed to connect to DB","error":"%v"}`, err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf(`{"level":"fatal","msg":"Failed to ping DB","error":"%v"}`, err)
	}

	log.Println(`{"level":"info","msg":"Connected to PostgreSQL"}`)
}

func initRedis() {
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	rdb = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
		DB:   0,
	})

	ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctxTimeout).Err(); err != nil {
		log.Fatalf(`{"level":"fatal","msg":"Failed to connect to Redis","error":"%v"}`, err)
	}
	log.Println(`{"level":"info","msg":"Connected to Redis"}`)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	requestMetric.Add(ctx, 1)

	dbErr := db.Ping()
	redisErr := rdb.Ping(ctx).Err()

	status := map[string]string{
		"database": "ok",
		"redis":    "ok",
	}
	code := http.StatusOK

	if dbErr != nil {
		status["database"] = "unreachable"
		code = http.StatusServiceUnavailable
	}
	if redisErr != nil {
		status["redis"] = "unreachable"
		code = http.StatusServiceUnavailable
	}

	statusJSON, _ := json.Marshal(status)
	log.Printf(`{"level":"info","msg":"Health check","status":%s}`, statusJSON)

	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf(`{"level":"error","msg":"Failed to encode health response","error":"%v"}`, err)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	requestMetric.Add(ctx, 1)

	log.Println(`{"level":"info","msg":"Login endpoint called"}`)
	if _, err := w.Write([]byte("Logged in")); err != nil {
		log.Printf(`{"level":"error","msg":"Failed to write login response","error":"%v"}`, err)
	}
}

func productsHandler(w http.ResponseWriter, r *http.Request) {
	requestMetric.Add(ctx, 1)

	rows, err := db.Query("SELECT name FROM products")
	if err != nil {
		log.Printf(`{"level":"error","msg":"DB query failed","error":"%v"}`, err)
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var products []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Printf(`{"level":"error","msg":"Row scan failed","error":"%v"}`, err)
			continue
		}
		products = append(products, name)
	}

	if err := json.NewEncoder(w).Encode(products); err != nil {
		log.Printf(`{"level":"error","msg":"Failed to encode products","error":"%v"}`, err)
	}
}
