package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"spendings-backend/internal/models"
	"spendings-backend/tests/integration"
)

type UserSuite struct {
	integration.SuiteWithRequests
	authToken string
}

func TestUserEndpoints(t *testing.T) {
	suite.Run(t, &UserSuite{})
}

// Helper function to get auth token for testing
// In a real scenario, this would authenticate and get a valid token
func (s *UserSuite) getAuthToken() string {
	// For testing purposes, we'll use a mock token
	// In a real implementation, you would authenticate first
	return "mock-jwt-token-for-testing"
}

// Helper function to create auth headers
func (s *UserSuite) getAuthHeaders() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + s.getAuthToken(),
	}
}

func (s *UserSuite) TestGetUserProfile() {
	headers := s.getAuthHeaders()

	res, code := s.GetAPI("http://localhost:8080", "/users/me", headers, nil)
	s.Equal(http.StatusOK, code)

	var profile models.UserProfile
	err := json.Unmarshal(res, &profile)
	s.NoError(err)
	s.NotEmpty(profile.Name)
	s.NotEmpty(profile.Phone)
	s.NotEmpty(profile.Birthday)
}

func (s *UserSuite) TestUpdateUserProfile() {
	headers := s.getAuthHeaders()

	updateRequest := models.UpdateUserRequest{
		Name:     "Updated Test User",
		Birthday: "15.03.1995",
		Image:    "https://example.com/new-avatar.jpg",
	}

	body, err := json.Marshal(updateRequest)
	s.NoError(err)

	res, code := s.PostAPI("http://localhost:8080", "/users/me", body, headers, nil)
	s.Equal(http.StatusOK, code)

	// Verify the update by getting the profile again
	res, code = s.GetAPI("http://localhost:8080", "/users/me", headers, nil)
	s.Equal(http.StatusOK, code)

	var profile models.UserProfile
	err = json.Unmarshal(res, &profile)
	s.NoError(err)
	s.Equal(updateRequest.Name, profile.Name)
	s.Equal(updateRequest.Birthday, profile.Birthday)
	s.Equal(updateRequest.Image, profile.Image)
}

func (s *UserSuite) TestDeleteUserAccount() {
	headers := s.getAuthHeaders()

	_, code := s.DeleteAPI("http://localhost:8080", "/users/me", headers, nil)
	s.Equal(http.StatusOK, code)

	// Verify account is deleted by trying to get profile
	_, code = s.GetAPI("http://localhost:8080", "/users/me", headers, nil)
	s.Equal(http.StatusUnauthorized, code)
}

func (s *UserSuite) TestLogout() {
	headers := s.getAuthHeaders()

	_, code := s.PostAPI("http://localhost:8080", "/logout", nil, headers, nil)
	s.Equal(http.StatusOK, code)

	// Verify user is logged out by trying to access protected endpoint
	_, code = s.GetAPI("http://localhost:8080", "/users/me", headers, nil)
	s.Equal(http.StatusUnauthorized, code)
}

func (s *UserSuite) TestGetUserAddresses() {
	headers := s.getAuthHeaders()

	res, code := s.GetAPI("http://localhost:8080", "/addresses", headers, nil)
	s.Equal(http.StatusOK, code)

	var addresses []models.Address
	err := json.Unmarshal(res, &addresses)
	s.NoError(err)
	// Addresses can be empty for a new user
}

func (s *UserSuite) TestAddUserAddress() {
	headers := s.getAuthHeaders()

	newAddress := models.Address{
		Coordinates:  []float64{37.6173, 55.7558}, // Moscow coordinates
		AddressLine:  "Test Street, 123",
		Floor:        "5",
		Entrance:     "1",
		IntercomCode: "1234",
		Comment:      "Test address for integration testing",
	}

	body, err := json.Marshal(newAddress)
	s.NoError(err)

	res, code := s.PostAPI("http://localhost:8080", "/addresses", body, headers, nil)
	s.Equal(http.StatusOK, code)

	// Verify address was added by getting addresses list
	res, code = s.GetAPI("http://localhost:8080", "/addresses", headers, nil)
	s.Equal(http.StatusOK, code)

	var addresses []models.Address
	err = json.Unmarshal(res, &addresses)
	s.NoError(err)
	s.NotEmpty(addresses)

	// Find the added address
	var foundAddress *models.Address
	for i := range addresses {
		if addresses[i].AddressLine == newAddress.AddressLine {
			foundAddress = &addresses[i]
			break
		}
	}
	s.NotNil(foundAddress)
	s.Equal(newAddress.AddressLine, foundAddress.AddressLine)
	s.Equal(newAddress.Floor, foundAddress.Floor)
	s.Equal(newAddress.Entrance, foundAddress.Entrance)
}

func (s *UserSuite) TestUpdateUserAddress() {
	headers := s.getAuthHeaders()

	// First, add an address to update
	newAddress := models.Address{
		Coordinates:  []float64{37.6173, 55.7558},
		AddressLine:  "Original Street, 456",
		Floor:        "3",
		Entrance:     "2",
		IntercomCode: "5678",
		Comment:      "Original address",
	}

	body, err := json.Marshal(newAddress)
	s.NoError(err)

	res, code := s.PostAPI("http://localhost:8080", "/addresses", body, headers, nil)
	s.Equal(http.StatusOK, code)

	// Get the address ID from the response or by fetching addresses
	res, code = s.GetAPI("http://localhost:8080", "/addresses", headers, nil)
	s.Equal(http.StatusOK, code)

	var addresses []models.Address
	err = json.Unmarshal(res, &addresses)
	s.NoError(err)
	s.NotEmpty(addresses)

	addressID := addresses[0].ID

	// Update the address
	updatedAddress := models.Address{
		Coordinates:  []float64{37.6173, 55.7558},
		AddressLine:  "Updated Street, 789",
		Floor:        "7",
		Entrance:     "3",
		IntercomCode: "9999",
		Comment:      "Updated address",
	}

	body, err = json.Marshal(updatedAddress)
	s.NoError(err)

	res, code = s.PostAPI("http://localhost:8080", fmt.Sprintf("/addresses/%s", addressID), body, headers, nil)
	s.Equal(http.StatusOK, code)

	// Verify the update
	res, code = s.GetAPI("http://localhost:8080", "/addresses", headers, nil)
	s.Equal(http.StatusOK, code)

	err = json.Unmarshal(res, &addresses)
	s.NoError(err)

	var foundAddress *models.Address
	for i := range addresses {
		if addresses[i].ID == addressID {
			foundAddress = &addresses[i]
			break
		}
	}
	s.NotNil(foundAddress)
	s.Equal(updatedAddress.AddressLine, foundAddress.AddressLine)
	s.Equal(updatedAddress.Floor, foundAddress.Floor)
}

func (s *UserSuite) TestDeleteUserAddress() {
	headers := s.getAuthHeaders()

	// First, add an address to delete
	newAddress := models.Address{
		Coordinates:  []float64{37.6173, 55.7558},
		AddressLine:  "To Delete Street, 999",
		Floor:        "1",
		Entrance:     "1",
		IntercomCode: "0000",
		Comment:      "Address to be deleted",
	}

	body, err := json.Marshal(newAddress)
	s.NoError(err)

	res, code := s.PostAPI("http://localhost:8080", "/addresses", body, headers, nil)
	s.Equal(http.StatusOK, code)

	// Get the address ID
	res, code = s.GetAPI("http://localhost:8080", "/addresses", headers, nil)
	s.Equal(http.StatusOK, code)

	var addresses []models.Address
	err = json.Unmarshal(res, &addresses)
	s.NoError(err)
	s.NotEmpty(addresses)

	addressID := addresses[0].ID

	// Delete the address
	res, code = s.DeleteAPI("http://localhost:8080", fmt.Sprintf("/addresses/%s", addressID), headers, nil)
	s.Equal(http.StatusOK, code)

	// Verify the address was deleted
	res, code = s.GetAPI("http://localhost:8080", "/addresses", headers, nil)
	s.Equal(http.StatusOK, code)

	err = json.Unmarshal(res, &addresses)
	s.NoError(err)

	// The address should no longer exist
	for _, addr := range addresses {
		s.NotEqual(addressID, addr.ID)
	}
}

func (s *UserSuite) TestUnauthorizedAccess() {
	// Test accessing protected endpoints without authentication
	endpoints := []string{
		"/users/me",
		"/addresses",
		"/logout",
	}

	for _, endpoint := range endpoints {
		res, code := s.GetAPI("http://localhost:8080", endpoint, nil, nil)
		s.Equal(http.StatusUnauthorized, code, "Endpoint %s should require authentication", endpoint)

		// Check error response format
		var errorResp struct {
			Error string `json:"error"`
		}
		err := json.Unmarshal(res, &errorResp)
		s.NoError(err)
		s.NotEmpty(errorResp.Error)
	}
}

func (s *UserSuite) TestInvalidAddressData() {
	headers := s.getAuthHeaders()

	// Test with invalid coordinates (should have exactly 2 elements)
	invalidAddress := models.Address{
		Coordinates:  []float64{37.6173}, // Only one coordinate
		AddressLine:  "Invalid Street, 123",
		Floor:        "1",
		Entrance:     "1",
		IntercomCode: "1234",
		Comment:      "Invalid address",
	}

	body, err := json.Marshal(invalidAddress)
	s.NoError(err)

	res, code := s.PostAPI("http://localhost:8080", "/addresses", body, headers, nil)
	// Should return 400 for invalid data
	s.Equal(http.StatusBadRequest, code)

	// Check error response
	var errorResp struct {
		Error string `json:"error"`
	}
	err = json.Unmarshal(res, &errorResp)
	s.NoError(err)
	s.NotEmpty(errorResp.Error)
}

func (s *UserSuite) TestUpdateNonExistentAddress() {
	headers := s.getAuthHeaders()

	nonExistentID := "non-existent-address-id"
	updatedAddress := models.Address{
		Coordinates:  []float64{37.6173, 55.7558},
		AddressLine:  "Updated Street, 789",
		Floor:        "7",
		Entrance:     "3",
		IntercomCode: "9999",
		Comment:      "Updated address",
	}

	body, err := json.Marshal(updatedAddress)
	s.NoError(err)

	res, code := s.PostAPI("http://localhost:8080", fmt.Sprintf("/addresses/%s", nonExistentID), body, headers, nil)
	// Should return 404 for non-existent address
	s.Equal(http.StatusNotFound, code)

	// Check error response
	var errorResp struct {
		Error string `json:"error"`
	}
	err = json.Unmarshal(res, &errorResp)
	s.NoError(err)
	s.NotEmpty(errorResp.Error)
}
