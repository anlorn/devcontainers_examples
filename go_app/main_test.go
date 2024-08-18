package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// API response for /GET endpoint
type ItemValue struct {
	Value string `json:"value"`
}

type APITestSuite struct {
	suite.Suite
	router         *gin.Engine
	wg             *sync.WaitGroup
	stopDBPoolChan chan bool
}

// Test setup: This is a helper function to set up the router and any necessary mocks.
func (s *APITestSuite) SetupSuite() {
	// Mock or set up a test database connection
	s.wg = &sync.WaitGroup{}
	testContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dbPool, stopDBPoolChan, err := connectToDB(testContext, s.wg) // For testing, consider mocking this
	if err != nil {
		s.T().Fatal(err)
	}
	s.stopDBPoolChan = stopDBPoolChan

	// Initialize database structure
	err = initDBStructure(testContext, dbPool)
	if err != nil {
		s.T().Fatal(err)
	}

	// Create a router using the same function used in your main code
	apiRouter, err := createRouter(dbPool)
	if err != nil {
		s.T().Fatal(err)
	}
	s.router = apiRouter
}

func (s *APITestSuite) TearDownSuite() {
	// Close the database connection pool
	s.stopDBPoolChan <- true
	s.wg.Wait()
}

func (s *APITestSuite) TestGetItem() {
	// PREPARE
	// Create a test item to insert into the database
	testItem := Item{
		ItemId: uuid.NewString(),
		Value:  uuid.NewString(),
	}
	body, err := json.Marshal(testItem)
	if err != nil {
		s.T().Fatal(err)
	}
	// Insert test items into the database, for simplicity, we call DB directly from handlers.
	// So, here we call a handler
	postReq, _ := http.NewRequest("POST", "/", bytes.NewReader(body))
	postReq.Header.Set("Content-Type", "application/json")
	postRecorder := httptest.NewRecorder()
	s.router.ServeHTTP(postRecorder, postReq)
	if postRecorder.Code != http.StatusCreated {
		s.T().Fatal("Failed to create item")
	}
	// Prepare request to get inserted item
	req, _ := http.NewRequest("GET", fmt.Sprintf("/%s", testItem.ItemId), nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// ACT
	s.router.ServeHTTP(w, req)

	// CHECK
	assert.Equal(s.T(), http.StatusOK, w.Code)
	resp := ItemValue{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), testItem.Value, resp.Value)
}

// We fetch non-existing item and expect 404 status code
func (s *APITestSuite) TestGetItemNotFound() {
	// PREPARE
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/fake_item", nil)

	// ACT
	s.router.ServeHTTP(w, req)

	// CHECK
	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

// We create new unique item and expect 201 status code
func (s *APITestSuite) TestCreateItem() {

	// PREPARE
	testItem := Item{
		ItemId: uuid.NewString(),
		Value:  uuid.NewString(),
	}
	body, err := json.Marshal(testItem)
	if err != nil {
		s.T().Fatal(err)
	}
	req, _ := http.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// ACT
	s.router.ServeHTTP(w, req)

	// CHECK
	assert.Equal(s.T(), http.StatusCreated, w.Code)
}

// We attempt item twice, we don't expect error, but attempt to insert the same items again
// will return 200 code instead of 201
func (s *APITestSuite) TestCreateDuplicateItem() {

	// PREPARE
	testItem := Item{
		ItemId: uuid.NewString(),
		Value:  uuid.NewString(),
	}
	body, err := json.Marshal(testItem)
	if err != nil {
		s.T().Fatal(err)
	}
	firstReq, _ := http.NewRequest("POST", "/", bytes.NewReader(body))
	firstReq.Header.Set("Content-Type", "application/json")
	secondReq, _ := http.NewRequest("POST", "/", bytes.NewReader(body))
	secondReq.Header.Set("Content-Type", "application/json")
	firstCall := httptest.NewRecorder()
	secondCall := httptest.NewRecorder()

	// ACT
	s.router.ServeHTTP(firstCall, firstReq)
	s.router.ServeHTTP(secondCall, secondReq)

	// CHECK
	assert.Equal(s.T(), http.StatusCreated, firstCall.Code)
	assert.Equal(s.T(), http.StatusOK, secondCall.Code)
}

// We attempt to post item with invalid json, we expect 400 code
func (s *APITestSuite) TestPostItemBadRequest() {
	// PREPARE
	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"invalid_json}`)
	req, _ := http.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "application/json")

	// ACT
	s.router.ServeHTTP(w, req)

	// TEST
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func TestAPISuiteRun(t *testing.T) {
	suite.Run(t, new(APITestSuite))
}
