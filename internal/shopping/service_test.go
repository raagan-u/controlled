package shopping

import (
	"context"
	"testing"
)

type fakeCatalog struct {
	products []Product
	err      error
}

func (catalog fakeCatalog) Search(context.Context, string, string, int) ([]Product, error) {
	return catalog.products, catalog.err
}

func TestCheckPriceReturnsMeaningfulBetterOffer(t *testing.T) {
	t.Parallel()
	service := NewService(fakeCatalog{products: []Product{{
		ID: "p1", Title: "Sony WH-1000XM5 Wireless Headphones", Brand: "Sony", ImageURL: "https://image.test/product.png",
		Offers: []Offer{
			{Domain: "used.test", URL: "https://used.test/p1", PriceMinor: 22000, Currency: "USD", Availability: "InStock", Condition: "used"},
			{Domain: "retailer.test", URL: "https://retailer.test/p1", PriceMinor: 29900, Currency: "USD", Availability: "InStock", Condition: "new"},
			{Domain: "expensive.test", URL: "https://expensive.test/p1", PriceMinor: 32900, Currency: "USD", Availability: "InStock", Condition: "new"},
		},
	}}})
	result, err := service.CheckPrice(context.Background(), "user-1", "Sony WH-1000XM5 headphones", 34900, "USD")
	if err != nil {
		t.Fatalf("CheckPrice() error = %v", err)
	}
	if !result.Matched || result.BestPriceMinor != 29900 || result.SavingsMinor != 5000 || result.Verdict != "better_price" || result.Merchant != "retailer.test" {
		t.Fatalf("CheckPrice() = %#v", result)
	}
}

func TestCheckPriceDoesNotForceAmbiguousMatch(t *testing.T) {
	t.Parallel()
	service := NewService(fakeCatalog{products: []Product{{
		ID: "p1", Title: "Sony WH-1000XM5", Brand: "Sony",
		Offers: []Offer{{Domain: "retailer.test", URL: "https://retailer.test/p1", PriceMinor: 29900, Currency: "USD", Availability: "InStock", Condition: "new"}},
	}}})
	result, err := service.CheckPrice(context.Background(), "user-1", "birthday gift", 34900, "USD")
	if err != nil {
		t.Fatalf("CheckPrice() error = %v", err)
	}
	if result.Matched || result.Message == "" {
		t.Fatalf("CheckPrice() = %#v", result)
	}
}

func TestCheckPriceRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	_, err := NewService(fakeCatalog{}).CheckPrice(context.Background(), "user-1", "x", 100, "USD")
	if err != ErrInvalidInput {
		t.Fatalf("CheckPrice() error = %v", err)
	}
}
