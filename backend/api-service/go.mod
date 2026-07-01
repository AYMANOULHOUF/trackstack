module github.com/trackproj/api-service

go 1.22.2

replace golang.org/x/net => github.com/golang/net v0.17.0

replace golang.org/x/crypto => github.com/golang/crypto v0.21.0

require (
	github.com/go-chi/chi/v5 v5.0.12
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/gorilla/websocket v1.5.1
	github.com/lib/pq v1.12.3
	golang.org/x/crypto v0.21.0
)

require golang.org/x/net v0.21.0 // indirect
