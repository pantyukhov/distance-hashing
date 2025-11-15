package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	dh "github.com/wallarm/distance-hashing"
)

// Global session generator (initialize once at startup)
var sessionGen *dh.SessionGenerator

func init() {
	var err error
	sessionGen, err = dh.NewSessionGenerator(10000)
	if err != nil {
		log.Fatalf("Failed to initialize session generator: %v", err)
	}
}

// SessionMiddleware extracts identifiers from HTTP request and generates session key
func SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract identifiers from various sources
		identifiers := dh.Identifiers{
			dh.IdentifierUserID: r.Header.Get("X-User-ID"),
			dh.IdentifierJWT:    extractJWT(r),
			dh.IdentifierCookie: extractCookie(r, "session_id"),
			dh.IdentifierDevice: r.Header.Get("X-Device-ID"),
			dh.IdentifierClient: r.Header.Get("X-Client-ID"),
		}

		// Generate unified session key
		sessionKey := sessionGen.GetSessionKey(identifiers)

		// Add session key to response header
		w.Header().Set("X-Session-Key", sessionKey)

		// Store in request context for downstream handlers
		r = r.WithContext(r.Context())

		// Log for debugging
		log.Printf("[Session] %s %s - Session: %s (UserID: %s, Cookie: %s)",
			r.Method,
			r.URL.Path,
			sessionKey,
			identifiers[dh.IdentifierUserID],
			identifiers[dh.IdentifierCookie],
		)

		next.ServeHTTP(w, r)
	})
}

// extractJWT extracts JWT token from Authorization header
func extractJWT(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// extractCookie extracts specific cookie value
func extractCookie(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// loginHandler simulates user login and links cookie to user ID
func loginHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate authentication
	userID := r.FormValue("user_id")
	cookieID := extractCookie(r, "session_id")

	if userID != "" && cookieID != "" {
		// Link cookie to authenticated user
		sessionGen.LinkIdentifiers(
			fmt.Sprintf("cookie:%s", cookieID),
			fmt.Sprintf("uid:%s", userID),
		)

		log.Printf("[Login] Linked cookie:%s to uid:%s", cookieID, userID)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Logged in as user: %s\n", userID)
	} else {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Missing user_id or session_id\n")
	}
}

// statsHandler returns session statistics
func statsHandler(w http.ResponseWriter, r *http.Request) {
	stats := sessionGen.GetStats()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"total_identifiers": %d,
		"total_sessions": %d,
		"cache_size": %d
	}`, stats.TotalIdentifiers, stats.TotalSessions, stats.CacheSize)
}

func main() {
	// Create HTTP server with middleware
	mux := http.NewServeMux()

	// Register handlers
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/stats", statsHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Session tracking active. Session key: %s\n",
			w.Header().Get("X-Session-Key"))
	})

	// Wrap with session middleware
	handler := SessionMiddleware(mux)

	// Start server
	addr := ":8080"
	log.Printf("Starting server on %s", addr)
	log.Printf("Try:")
	log.Printf("  curl -H 'Cookie: session_id=abc123' http://localhost:8080/")
	log.Printf("  curl -H 'Cookie: session_id=abc123' -X POST -d 'user_id=user_42' http://localhost:8080/login")
	log.Printf("  curl http://localhost:8080/stats")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
