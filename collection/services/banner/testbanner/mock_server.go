package testbanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Pjt727/classy/collection"
	"github.com/Pjt727/classy/collection/services/banner"
	"github.com/google/uuid"
)

// Global map to store active JSESSIONIDs. Using a bool as value since we only care about existence.
type mockServerState struct {
	logger        slog.Logger
	sessions      map[string]string
	sessionsMutex sync.RWMutex
}

// generateSessionID creates a new unique session ID.
func generateSessionID() string {
	return uuid.New().String()
}

// requireSession middleware checks for a valid JSESSIONID cookie.
func (m *mockServerState) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("JSESSIONID")
		if err != nil || cookie.Value == "" {
			m.logger.Error("Cookie not set set")
			http.Error(w, "Session cookie not found or invalid", http.StatusUnauthorized)
			return
		}

		m.sessionsMutex.RLock()
		_, ok := m.sessions[cookie.Value]
		m.sessionsMutex.RUnlock()
		if !ok {
			m.logger.Error("cookie value not found in the sessions", "JSESSIONID", cookie.Value)
			http.Error(w, "Invalid JSESSIONID", http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// this route is used to get a cookie
func (m *mockServerState) handleTermSelectionSearch(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("mode") != "search" {
		m.logger.Error("mode query must be set to search")
		http.Error(w, "Bad Request: 'mode' query parameter must be 'search'", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie("JSESSIONID")

	if err != nil || cookie.Value == "" {
		sessionID := generateSessionID()
		m.sessionsMutex.Lock()
		m.sessions[sessionID] = ""
		m.sessionsMutex.Unlock()

		http.SetCookie(w, &http.Cookie{
			Name:     "JSESSIONID",
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Now().Add(24 * time.Hour), // Example: expires in 24 hours
		})
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
}

// this route is used to associate the session with a term
func (m *mockServerState) handleTermSearch(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		m.logger.Error("errors parsing form", "err", err)
		http.Error(w, "Bad Request: Could not parse form", http.StatusBadRequest)
		return
	}
	cookie, _ := r.Cookie("JSESSIONID") // cookie already checked in middleware

	term := r.FormValue("term")
	if term == "" {
		m.logger.Error("term must be set")
		http.Error(w, "Bad Request: 'term' parameter is required", http.StatusBadRequest)
		return
	}

	m.sessionsMutex.Lock()
	m.sessions[cookie.Value] = term
	m.sessionsMutex.Unlock()

	response := map[string]any{
		"term_associated": term,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type searchResults struct {
	TotalCount int              `json:"totalCount"`
	Data       []map[string]any `json:"data"`
}

// this route returns all the class data
func (m *mockServerState) handleSearchResults(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	term := query.Get("txt_term")
	pageOffsetStr := query.Get("pageOffset")
	pageMaxSizeStr := query.Get("pageMaxSize")

	if term == "" || pageOffsetStr == "" || pageMaxSizeStr == "" {
		m.logger.Error("values not not be null", "term", term, "pageOffsetStr", pageOffsetStr, "pageMaxSizeStr", pageMaxSizeStr)
		http.Error(w, "Bad Request: no null values allowed", http.StatusBadRequest)
		return
	}
	cookie, _ := r.Cookie("JSESSIONID") // cookie already checked in middleware

	m.sessionsMutex.RLock()
	termFromSession := m.sessions[cookie.Value]
	m.sessionsMutex.RUnlock()
	if term != termFromSession {
		m.logger.Error("terms don't match", "givenTerm", term, "sessionTerm", termFromSession)
		http.Error(w, fmt.Sprintf("Bad Request: given term `%s` does not match associated term `%s`", term, termFromSession), http.StatusBadRequest)
		return
	}

	pageOffset, err := strconv.Atoi(pageOffsetStr)
	if err != nil || pageOffset < 0 {
		m.logger.Error("invalid page offset", "pageOffset", pageOffsetStr)
		http.Error(w, fmt.Sprintf("Invalid 'pageOffset'=`%s`", pageOffsetStr), http.StatusBadRequest)
		return
	}
	pageMaxSize, err := strconv.Atoi(pageMaxSizeStr)
	if err != nil || pageMaxSize < 0 {
		m.logger.Error("invalid page max size", "pageMaxSize", pageMaxSize)
		http.Error(w, fmt.Sprintf("Invalid 'pageMaxsize'=`%s`", pageMaxSizeStr), http.StatusBadRequest)
		return
	}

	// might eventually add the different data or only read this once
	sectionsPath := filepath.Join(TESTING_ASSETS_BASE_DIR, "marist", "mock-server", "class-search.json")
	jsonData, err := os.ReadFile(sectionsPath)
	if err != nil {
		m.logger.Error("could not sections file", "path", sectionsPath)
		http.Error(w, fmt.Sprintf("Could not find sections path %s", sectionsPath), http.StatusInternalServerError)
		return
	}

	var sectionsData searchResults
	err = json.Unmarshal(jsonData, &sectionsData)
	if err != nil {
		m.logger.Error("could not parse sections", "err", err)
		http.Error(w, fmt.Sprintf("Could parse terms %v", err), http.StatusInternalServerError)
		return
	}

	if pageOffset > len(sectionsData.Data) {
		m.logger.Error("invalid page offset", "pageoffset", pageOffset, "sectionDataLength", len(sectionsData.Data))
		http.Error(w,
			fmt.Sprintf("Invalid page offset %d", pageOffset),
			http.StatusInternalServerError)
		return
	}

	// simulate the page offset
	sectionsData.Data = sectionsData.Data[pageOffset:min(len(sectionsData.Data), pageOffset+pageMaxSize)]

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sectionsData)
}

// this route is to get the terms
func (m *mockServerState) handleGetTerms(w http.ResponseWriter, r *http.Request) {

	// might eventually add the different data or only read this once
	termsPath := filepath.Join(TESTING_ASSETS_BASE_DIR, "marist", "mock-server", "terms.json")
	http.ServeFile(w, r, termsPath)
}

// this route gets the course description for a single course
func (m *mockServerState) handleGetCourseDescription(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm() // This handles application/x-www-form-urlencoded
	if err != nil {
		m.logger.Error("Could not parse course form", "err", err)
		http.Error(w, "Bad Request: Could not parse form", http.StatusBadRequest)
		return
	}

	coursePath := filepath.Join(TESTING_ASSETS_BASE_DIR, "marist", "mock-server", "course.json")
	http.ServeFile(w, r, coursePath)
}

// returns a new server which will be closed once the context ends
func NewMockServer(logger slog.Logger, ctx context.Context) *httptest.Server {
	serverState := mockServerState{
		logger:        logger,
		sessions:      make(map[string]string),
		sessionsMutex: sync.RWMutex{},
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /StudentRegistrationSsb/ssb/term/termSelection", serverState.handleTermSelectionSearch)
	mux.HandleFunc("POST /StudentRegistrationSsb/ssb/term/search", serverState.requireSession(serverState.handleTermSearch))
	mux.HandleFunc("GET /StudentRegistrationSsb/ssb/searchResults/searchResults", serverState.requireSession(serverState.handleSearchResults))
	mux.HandleFunc("GET /StudentRegistrationSsb/ssb/classSearch/getTerms", serverState.handleGetTerms)
	mux.HandleFunc("POST /StudentRegistrationSsb/ssb/searchResults/getCourseDescription", serverState.requireSession(serverState.handleGetCourseDescription))

	server := httptest.NewServer(mux)
	// close server once the context finishes
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	return httptest.NewServer(mux)
}

// this context is tied to the server and once the context closes the server will too
func GetMockTestingService(logger slog.Logger, ctx context.Context) (collection.Service, error) {
	mockServer := NewMockServer(logger, ctx)

	bannerService := banner.GetDefaultService()
	schools, err := bannerService.ListValidSchools(logger, ctx)
	if err != nil {
		return nil, err
	}
	for _, school := range schools {
		// for now just make all schools point to the same mockServer
		didSet := bannerService.SetHostname(school.ID, mockServer.URL)
		if !didSet {
			return nil, fmt.Errorf("Could not set hostname for %s", school.ID)
		}
	}
	return bannerService, nil
}
