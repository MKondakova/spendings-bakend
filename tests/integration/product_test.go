package integration_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"spendings-backend/internal/models"
	"spendings-backend/tests/integration"
)

type ProductSuite struct {
	integration.SuiteWithRequests
}

func TestProductEndpoints(t *testing.T) {
	suite.Run(t, &ProductSuite{})
}

func (s *ProductSuite) TestGetProductByID() {
	// Assume this product exists in the test DB
	id := "ff25265d-9dfc-49c3-bd01-678c6baa001f"

	res, code := s.GetAPI("http://localhost:8080", "/products/"+id, nil, nil)
	s.Equal(http.StatusOK, code)

	var product models.Product
	err := json.Unmarshal(res, &product)
	s.NoError(err)
	s.Equal(id, product.ID)
	s.NotEmpty(product.Name)
	s.NotEmpty(product.Image)
	s.NotZero(product.Price)
}

func (s *ProductSuite) TestGetProductsList() {
	res, code := s.GetAPI("http://localhost:8080", "/products", nil, nil)
	s.Equal(http.StatusOK, code)

	var products []models.ProductPreview
	err := json.Unmarshal(res, &products)
	s.NoError(err)
	s.NotEmpty(products)
}

func (s *ProductSuite) TestGetCategories() {
	res, code := s.GetAPI("http://localhost:8080", "/categories", nil, nil)
	s.Equal(http.StatusOK, code)

	var categories []models.Category
	err := json.Unmarshal(res, &categories)
	s.NoError(err)
	s.NotEmpty(categories)
}
