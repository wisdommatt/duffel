package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_status(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *http.Request
		want  func(response *http.Response)
	}{
		// ... add test cases here
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandler()

			request := tt.setup()
			recorder := httptest.NewRecorder()

			h.status(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()

			tt.want(response)
		})
	}
}

func TestHandler_health(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *http.Request
		want  func(response *http.Response)
	}{
		// ... add test cases here
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandler()

			request := tt.setup()
			recorder := httptest.NewRecorder()

			h.health(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()

			tt.want(response)
		})
	}
}

func TestHandler_ping(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *http.Request
		want  func(response *http.Response)
	}{
		// ... add test cases here
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHandler()

			request := tt.setup()
			recorder := httptest.NewRecorder()

			h.ping(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()

			tt.want(response)
		})
	}
}
