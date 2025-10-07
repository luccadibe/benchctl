package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Response represents the API response structure
type Response struct {
	Message   string   `json:"message"`
	Timestamp int64    `json:"timestamp"`
	Duration  int      `json:"duration_ms"`
	Status    string   `json:"status"`
	TaskType  TaskType `json:"task_type"`
}

// WorkSimulator simulates different types of work
type WorkSimulator struct {
	baseDelay int
	variance  int
	errorRate float64
}

func NewWorkSimulator(baseDelay, variance int, errorRate float64) *WorkSimulator {
	return &WorkSimulator{
		baseDelay: baseDelay,
		variance:  variance,
		errorRate: errorRate,
	}
}

type TaskType int

const (
	TaskTypeA TaskType = iota
	TaskTypeB
	TaskTypeC
)

func (ws *WorkSimulator) SimulateWork() (int, TaskType, error) {
	// Simulate variable work duration
	delay := ws.baseDelay + rand.Intn(ws.variance)

	// Simulate occasional errors
	if rand.Float64() < ws.errorRate {
		return 0, 0, fmt.Errorf("simulated error")
	}

	time.Sleep(time.Duration(delay) * time.Millisecond)

	//pick random task type
	taskType := TaskType(rand.Intn(3))

	return delay, taskType, nil
}

func main() {
	// Configuration from environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	baseDelay, _ := strconv.Atoi(os.Getenv("BASE_DELAY"))
	if baseDelay == 0 {
		baseDelay = 100 // Default 100ms base delay
	}

	variance, _ := strconv.Atoi(os.Getenv("VARIANCE"))
	if variance == 0 {
		variance = 50 // Default 50ms variance
	}

	errorRate, _ := strconv.ParseFloat(os.Getenv("ERROR_RATE"), 64)
	if errorRate == 0 {
		errorRate = 0.05 // Default 5% error rate
	}

	simulator := NewWorkSimulator(baseDelay, variance, errorRate)

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "healthy",
			"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
		})
	})

	// Main work endpoint
	http.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		duration, taskType, err := simulator.SimulateWork()
		elapsed := time.Since(start)

		response := Response{
			Message:   "Work completed",
			Timestamp: time.Now().Unix(),
			Duration:  duration,
			Status:    "success",
			TaskType:  taskType,
		}

		if err != nil {
			response.Status = "error"
			response.Message = err.Error()
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

		// Log the request for monitoring
		log.Printf("Request completed in %v (simulated: %dms, status: %s)",
			elapsed, duration, response.Status)
	})

	// Metrics endpoint for monitoring
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "# Server metrics\n")
		fmt.Fprintf(w, "server_uptime_seconds %d\n", time.Now().Unix())
		fmt.Fprintf(w, "server_base_delay_ms %d\n", baseDelay)
		fmt.Fprintf(w, "server_variance_ms %d\n", variance)
		fmt.Fprintf(w, "server_error_rate %f\n", errorRate)
	})

	log.Printf("Starting server on port %s", port)
	log.Printf("Configuration: base_delay=%dms, variance=%dms, error_rate=%.2f%%",
		baseDelay, variance, errorRate*100)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
