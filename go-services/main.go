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
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

var (
	db    *sql.DB
	rdb   *redis.Client
	ctx   = context.Background()
)

func main() {
	initTracer()
	initLog()
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
	logFile, err := os.OpenFile("logs.json", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf(`{"level":"fatal","msg":"Failed to open log file","error":"%v"}`, err)
	}
	log.SetOutput(logFile)
}

func initTracer() {
	exporter, _ := stdouttrace.New()
	tp := trace.NewTracerProvider(trace.WithBatcher(exporter))
	otel.SetTracerProvider(tp)
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

	log.Printf(`{"level":"info","msg":"Health check","status":%q}`, status)
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(status)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(`{"level":"info","msg":"Login endpoint called"}`)
	w.Write([]byte("Logged in"))
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

	_ = json.NewEncoder(w).Encode(products)
}
