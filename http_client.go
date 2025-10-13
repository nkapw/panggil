package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// HttpRequestData contains all the information needed to make an HTTP request.
// HttpRequestData berisi semua informasi yang dibutuhkan untuk membuat sebuah request HTTP.
type HttpRequestData struct {
	Method    string
	URL       string
	Headers   map[string]string
	Body      string
	AuthType  string
	AuthToken string
	AuthUser  string
	AuthPass  string
}

// HttpResponseData contains the results of an HTTP request.
// HttpResponseData berisi hasil dari sebuah request HTTP.
type HttpResponseData struct {
	Status        string
	StatusCode    int
	Duration      time.Duration
	ContentLength int64
	Headers       http.Header
	Body          []byte
	Error         error
}

// doHttpRequest is a pure function that sends an HTTP request and returns the result.
// It has no dependency on the UI (tview). /
// doHttpRequest adalah fungsi murni yang mengirim sebuah request HTTP dan mengembalikan hasilnya. Fungsi ini tidak memiliki dependensi ke UI (tview).
func doHttpRequest(data HttpRequestData) *HttpResponseData {
	var bodyReader io.Reader
	if data.Body != "" {
		bodyReader = bytes.NewBufferString(data.Body)
	}

	req, err := http.NewRequest(data.Method, data.URL, bodyReader)
	if err != nil {
		log.Printf("ERROR: Failed to create HTTP request for %s %s: %v", data.Method, data.URL, err)
		return &HttpResponseData{Error: fmt.Errorf("creating request: %w", err)}
	}

	for k, v := range data.Headers {
		req.Header.Set(k, v)
	}

	switch data.AuthType {
	case "Bearer Token":
		if data.AuthToken != "" {
			req.Header.Set("Authorization", "Bearer "+data.AuthToken)
		}
	case "Basic Auth":
		if data.AuthUser != "" {
			req.SetBasicAuth(data.AuthUser, data.AuthPass)
		}
	}

	log.Printf("INFO: Sending HTTP request: %s %s", data.Method, data.URL)
	start := time.Now()
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		log.Printf("ERROR: HTTP request failed for %s %s: %v", data.Method, data.URL, err)
		return &HttpResponseData{Error: err, Duration: duration}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: Failed to read HTTP response body: %v", err)
		return &HttpResponseData{Error: fmt.Errorf("reading response: %w", err), Duration: duration}
	}

	log.Printf("INFO: HTTP request to %s %s completed with status %s. Duration: %v", data.Method, data.URL, resp.Status, duration)

	return &HttpResponseData{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		Duration:      duration,
		ContentLength: resp.ContentLength,
		Headers:       resp.Header,
		Body:          body,
		Error:         nil,
	}
}
