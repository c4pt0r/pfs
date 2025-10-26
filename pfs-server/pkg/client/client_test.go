package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Create(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/files" {
			t.Errorf("expected /files, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("path") != "/test/file.txt" {
			t.Errorf("expected path=/test/file.txt, got %s", r.URL.Query().Get("path"))
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(SuccessResponse{Message: "file created"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Create("/test/file.txt")
	if err != nil {
		t.Errorf("Create failed: %v", err)
	}
}

func TestClient_Read(t *testing.T) {
	expectedData := []byte("hello world")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/files" {
			t.Errorf("expected /files, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(expectedData)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	data, err := client.Read("/test/file.txt", 0, -1)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}
	if string(data) != string(expectedData) {
		t.Errorf("expected %s, got %s", expectedData, data)
	}
}

func TestClient_Write(t *testing.T) {
	testData := []byte("test content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/files" {
			t.Errorf("expected /files, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SuccessResponse{Message: "OK"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	response, err := client.Write("/test/file.txt", testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if string(response) != "OK" {
		t.Errorf("expected OK, got %s", response)
	}
}

func TestClient_Mkdir(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/directories" {
			t.Errorf("expected /directories, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("mode") != "755" {
			t.Errorf("expected mode=755, got %s", r.URL.Query().Get("mode"))
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(SuccessResponse{Message: "directory created"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Mkdir("/test/dir", 0755)
	if err != nil {
		t.Errorf("Mkdir failed: %v", err)
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "file not found"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Read("/nonexistent", 0, -1)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
