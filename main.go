package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type OrderStatus string

const (
	StatusPending  OrderStatus = "PENDING"
	StatusPaid     OrderStatus = "PAID"
	StatusCanceled OrderStatus = "CANCELED"
)

// --- Events ---
type EventType string

const (
	EventOrderCreated  EventType = "OrderCreated"
	EventOrderPaid     EventType = "OrderPaid"
	EventOrderCanceled EventType = "OrderCanceled"
)

type Event struct {
	Type      EventType       `json:"type"`
	OrderID   string          `json:"order_id"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// --- Read model (in-memory) ---
type Order struct {
	ID     string      `json:"id"`
	Status OrderStatus `json:"status"`
}

var (
	eventLog []Event              // упрощённый event store
	orders   = map[string]Order{} // read model
	mutex    sync.Mutex
)

// --- Command Handlers ---
func createOrder(w http.ResponseWriter, r *http.Request) {
	orderID := uuid.New().String()
	event := Event{
		Type:      EventOrderCreated,
		OrderID:   orderID,
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{}`),
	}
	appendEvent(event)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"order_id": orderID})
}

func payOrder(w http.ResponseWriter, r *http.Request) {
	orderID := mux.Vars(r)["id"]
	event := Event{
		Type:      EventOrderPaid,
		OrderID:   orderID,
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{}`),
	}
	appendEvent(event)
	w.WriteHeader(http.StatusNoContent)
}

func cancelOrder(w http.ResponseWriter, r *http.Request) {
	orderID := mux.Vars(r)["id"]
	event := Event{
		Type:      EventOrderCanceled,
		OrderID:   orderID,
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{}`),
	}
	appendEvent(event)
	w.WriteHeader(http.StatusNoContent)
}

// --- Query Handlers ---
func getOrder(w http.ResponseWriter, r *http.Request) {
	orderID := mux.Vars(r)["id"]
	mutex.Lock()
	order, ok := orders[orderID]
	mutex.Unlock()

	if !ok {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(order)
}

func getAllEvents(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()
	json.NewEncoder(w).Encode(eventLog)
}

// --- Event Store & Projection ---
func appendEvent(e Event) {
	mutex.Lock()
	defer mutex.Unlock()
	eventLog = append(eventLog, e)
	applyEvent(e)
}

func applyEvent(e Event) {
	switch e.Type {
	case EventOrderCreated:
		orders[e.OrderID] = Order{ID: e.OrderID, Status: StatusPending}
	case EventOrderPaid:
		if o, ok := orders[e.OrderID]; ok {
			o.Status = StatusPaid
			orders[e.OrderID] = o
		}
	case EventOrderCanceled:
		if o, ok := orders[e.OrderID]; ok {
			o.Status = StatusCanceled
			orders[e.OrderID] = o
		}
	}
}

// --- Init ---
func rebuildState() {
	for _, e := range eventLog {
		applyEvent(e)
	}
}

func main() {
	rebuildState()
	r := mux.NewRouter()

	// Команды
	r.HandleFunc("/orders", createOrder).Methods("POST")
	r.HandleFunc("/orders/{id}/pay", payOrder).Methods("POST")
	r.HandleFunc("/orders/{id}/cancel", cancelOrder).Methods("POST")

	// Запросы
	r.HandleFunc("/orders/{id}", getOrder).Methods("GET")
	r.HandleFunc("/events", getAllEvents).Methods("GET")

	log.Println("Listening on :8080")
	http.ListenAndServe(":8080", r)
}
