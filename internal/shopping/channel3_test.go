package shopping

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestChannel3CatalogSearchMapsProductsAndForwardsUser(t *testing.T) {
	t.Parallel()
	catalog := &Channel3Catalog{apiBase: "https://channel3.test/v1", apiKey: "secret", client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/search" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("x-api-key") != "secret" || request.Header.Get("x-user-id") != "user-1" {
			t.Fatalf("headers = %#v", request.Header)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"products":[{"id":"product-1","title":"Sony WH-1000XM5","brands":[{"name":"Sony"}],"images":[{"url":"https://image.test/a.png","cleaned_url":"https://image.test/clean.png","is_main_image":true}],"offers":[{"domain":"shop.test","url":"https://shop.test/item","availability":"InStock","condition":"new","price":{"price":299.99,"currency":"USD"}}]}]}`))}, nil
	})}}
	products, err := catalog.Search(context.Background(), "user-1", "Sony WH-1000XM5", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(products) != 1 || products[0].ID != "product-1" || products[0].Brand != "Sony" || products[0].ImageURL != "https://image.test/clean.png" {
		t.Fatalf("Search() = %#v", products)
	}
	if len(products[0].Offers) != 1 || products[0].Offers[0].PriceMinor != 29999 {
		t.Fatalf("offers = %#v", products[0].Offers)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
