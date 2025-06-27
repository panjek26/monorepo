package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	redismock "github.com/go-redis/redismock/v9"
)

func TestHealthHandler_MockDBRedis(t *testing.T) {
	// mock PostgreSQL
	mockDB, mockSQL, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()
	db = mockDB

	mockSQL.ExpectPing()

	// mock Redis
	mockRedis, redisMock := redismock.NewClientMock()
	defer mockRedis.Close()
	rdb = mockRedis
	ctx = context.Background()

	redisMock.ExpectPing().SetVal("PONG")

	// create test HTTP request
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	// call handler
	healthHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}

	if body["database"] != "ok" {
		t.Errorf("expected database to be ok, got %s", body["database"])
	}
	if body["redis"] != "ok" {
		t.Errorf("expected redis to be ok, got %s", body["redis"])
	}
}
