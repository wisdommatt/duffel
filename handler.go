package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type handler struct {
	tracer trace.Tracer
	logger *slog.Logger
}

// newHandler builds a handler wired to the globally configured otel providers.
func newHandler() *handler {
	return &handler{
		tracer: otel.Tracer("handler"),
		logger: otelslog.NewLogger("handler"),
	}
}

func (h *handler) status(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"service":"duffel", "status":"running"}`))
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *handler) ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

// Airline A

// {
//   "destination": "JFK",
//   "origin": "LHR",
//   "departure_date": "2019-10-21"
// }

// {
//   "data": {
//     "offers": [
//       {
//         "arrival": "2019-12-21T12:30:00Z",
//         "departure": "2019-12-21T04:00:00Z",
//         "duration": 510,
//         "id": "a-d47b7da7-0fee-4365-813a-ed7816902372",
//         "total_amount": 55263,
//         "total_currency": "GBP",
//         "flight_number": "A111",
//         "origin": "LHR",
//         "destination": "JFK",
//       },
//     ],
//   },
// }

// Airline B

// {
// 	"origin": "LHR",
// 	"destination": "JFK",
// 	"departure_date": "2019-10-21"
// }

// {
//   "flights": [
//     {
//       "price": {
//         "amount": "572.85"
//       },
//       "arrival": "2019-12-21T11:30:00Z",
//       "departure": "2019-12-21T03:00:00Z",
//       "dest": "JFK",
//       "id": "b-c2398b86-9884-4ff2-b99a-c7556490af0a",
//       "origin": "LHR",
//       "currency": "GBP",
//       "flight_number": "B111"
//     }
//   ]
// }

// abstraction

// {
// 	"origin": "LHR",
// 	"destination": "JFK",
// 	"departure_date": "2019-10-21"
// }

// {
// 	"data": [
// 		{
// 			"id": "",
// 			"amount": {
// 				"value": "572.85",
// 				"currency": "GBP"
// 			},
// 			"flight_number": "B111",
// 			"origin": "LHR",
// 			"destination": "JFK",
// 			"departure": "2019-12-21T03:00:00Z",
// 			"arrival": "2019-12-21T11:30:00Z"
// 		}
// 	],
// 	"pagination": {}
// }

type SearchRequest struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Departure   string `json:"departure_date"`
}

type SearchResponse struct {
	Data []Flight `json:"data"`
	// Pagination
}

type Amount struct {
	Value    decimal.Decimal `json:"value"`
	Currency string          `json:"currency"`
}

type Flight struct {
	ID           string    `json:"id"`
	Amount       Amount    `json:"amount"`
	FlightNumber string    `json:"flight_number"`
	Origin       string    `json:"origin"`
	Destination  string    `json:"destination"`
	Departure    time.Time `json:"departure"`
	Arrival      time.Time `json:"arrival"`
	// CreatedAt
	// UpdatedAt
}

type Airline interface {
	Search(req SearchRequest) (SearchResponse, error)
}

type AirlineA struct{}

func (a *AirlineA) Search(req SearchRequest) (SearchResponse, error) {
	// Implement Airline A's search logic here
	return SearchResponse{}, nil
}

const airlineBEndpoint = "https://interview.duffel.com/airline_b"

type AirlineB struct {
	// Endpoint overrides the airline B search URL. Defaults to airlineBEndpoint.
	Endpoint string
	// Client overrides the HTTP client. Defaults to a client with a 10s timeout.
	Client *http.Client
}

// AirlineBResponse models the raw payload returned by airline B.
type AirlineBResponse struct {
	Flights []struct {
		Price struct {
			Amount decimal.Decimal `json:"amount"`
		} `json:"price"`
		Arrival      time.Time `json:"arrival"`
		Departure    time.Time `json:"departure"`
		Dest         string    `json:"dest"`
		ID           string    `json:"id"`
		Origin       string    `json:"origin"`
		Currency     string    `json:"currency"`
		FlightNumber string    `json:"flight_number"`
	} `json:"flights"`
}

func (b *AirlineB) Search(req SearchRequest) (SearchResponse, error) {
	endpoint := b.Endpoint
	if endpoint == "" {
		endpoint = airlineBEndpoint
	}
	client := b.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	// Airline B's request body matches SearchRequest's shape exactly.
	body, err := json.Marshal(req)
	if err != nil {
		return SearchResponse{}, fmt.Errorf("marshal search request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return SearchResponse{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return SearchResponse{}, fmt.Errorf("call airline B: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SearchResponse{}, fmt.Errorf("airline B returned status %d", resp.StatusCode)
	}

	var raw AirlineBResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return SearchResponse{}, fmt.Errorf("decode airline B response: %w", err)
	}

	out := SearchResponse{Data: make([]Flight, 0, len(raw.Flights))}
	for _, f := range raw.Flights {
		out.Data = append(out.Data, Flight{
			ID: f.ID,
			Amount: Amount{
				Value:    f.Price.Amount,
				Currency: f.Currency,
			},
			FlightNumber: f.FlightNumber,
			Origin:       f.Origin,
			Destination:  f.Dest,
			Departure:    f.Departure,
			Arrival:      f.Arrival,
		})
	}
	return out, nil
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	airline := AirlineB{}

	searchResult, err := airline.Search(SearchRequest{
		Origin:      "LHR",
		Destination: "JFK",
		Departure:   "2019-10-21",
	})
	if err != nil {
		// h.logger.Error("search failed", "error", err)
		log.Printf("search failed: %v", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	resultJSON, err := json.Marshal(searchResult)
	if err != nil {
		h.logger.Error("failed to marshal search result", "error", err)
		http.Error(w, "failed to marshal search result", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resultJSON)
}
