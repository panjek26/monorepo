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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var (
	db  *sql.DB
	rdb *redis.Client
	ctx = context.Background()

	httpRequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path"},
	)
)

func main() {
	initLog()
	initMetrics()
	initTracer()
	initDB()
	initRedis()

	http.HandleFunc("/", withMetrics(rootHandler))
	http.HandleFunc("/healthz", withMetrics(healthHandler))
	http.HandleFunc("/login", withMetrics(loginHandler))
	http.HandleFunc("/products", withMetrics(productsHandler))
	http.Handle("/metrics", promhttp.Handler())

	log.Println(`{"level":"info","msg":"Go service started on :8080"}`)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf(`{"level":"fatal","msg":"Failed to start server","error":"%v"}`, err)
	}
}

func initLog() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
}

func initMetrics() {
	prometheus.MustRegister(httpRequestCount)
	prometheus.MustRegister(httpRequestDuration)
	log.Println(`{"level":"info","msg":"Prometheus metrics registered"}`)
}

func initTracer() {
	exporter, err := stdouttrace.New()
	if err != nil {
		log.Fatalf(`{"level":"fatal","msg":"Failed to initialize tracer","error":"%v"}`, err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(tp)
	log.Println(`{"level":"info","msg":"OpenTelemetry tracer initialized"}`)
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

func rootHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(`{"level":"info","msg":"Root endpoint called"}`)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("Welcome to the Go service!")); err != nil {
		log.Printf(`{"level":"error","msg":"Failed to write root response","error":"%v"}`, err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
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
	log.Println(`{"level":"info","msg":"Login endpoint called"}`)
	if _, err := w.Write([]byte("Logged in")); err != nil {
		log.Printf(`{"level":"error","msg":"Failed to write login response","error":"%v"}`, err)
	}
}

func productsHandler(w http.ResponseWriter, r *http.Request) {
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

func withMetrics(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler(w, r)
		duration := time.Since(start).Seconds()

		httpRequestCount.WithLabelValues(r.URL.Path, r.Method).Inc()
		httpRequestDuration.WithLabelValues(r.URL.Path).Observe(duration)
	}
}
