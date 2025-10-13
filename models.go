package main

import "time"

// Request represents a single saved HTTP or gRPC request.
// Request merepresentasikan satu request HTTP atau gRPC yang disimpan.
type Request struct {
	Name string    `json:"name"`
	Body string    `json:"body"`
	Time time.Time `json:"time"`
	Type string    `json:"type"` // "http" or "grpc"

	// HTTP specific fields / Field spesifik HTTP
	Method  string            `json:"method,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	// gRPC specific fields / Field spesifik gRPC
	GrpcServer   string `json:"grpc_server,omitempty"`
	GrpcMethod   string `json:"grpc_method,omitempty"`
	GrpcMetadata string `json:"grpc_metadata,omitempty"`
}

// CollectionNode represents a node in the collections tree. It can be a folder or a request.
// CollectionNode merepresentasikan sebuah node di dalam tree Collections. Node bisa berupa folder atau request.
type CollectionNode struct {
	Name     string            `json:"name"`
	IsFolder bool              `json:"is_folder"`
	Request  *Request          `json:"request,omitempty"`
	Children []*CollectionNode `json:"children,omitempty"`
	Expanded bool              `json:"-"` // Excluded from JSON serialization. / Dikecualikan dari serialisasi JSON.
}
