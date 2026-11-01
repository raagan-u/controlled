package shopping

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrProviderUnavailable = errors.New("product search provider is unavailable")

type Catalog interface {
	Search(context.Context, string, string, int) ([]Product, error)
}

type Product struct {
	ID       string
	Title    string
	Brand    string
	ImageURL string
	Offers   []Offer
}

type Offer struct {
	Domain       string
	URL          string
	PriceMinor   int64
	Currency     string
	Availability string
	Condition    string
}

type Channel3Options struct {
	APIBase string
	APIKey  string
	Timeout time.Duration
}

type Channel3Catalog struct {
	apiBase string
	apiKey  string
	client  *http.Client
}

func NewChannel3Catalog(options Channel3Options) (*Channel3Catalog, error) {
	options.APIBase = strings.TrimRight(strings.TrimSpace(options.APIBase), "/")
	options.APIKey = strings.TrimSpace(options.APIKey)
	if options.APIBase == "" || options.APIKey == "" || options.Timeout <= 0 {
		return nil, ErrProviderUnavailable
	}
	return &Channel3Catalog{apiBase: options.APIBase, apiKey: options.APIKey, client: &http.Client{Timeout: options.Timeout}}, nil
}

func (catalog *Channel3Catalog) Search(ctx context.Context, userID, query string, limit int) ([]Product, error) {
	body, err := json.Marshal(struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}{Query: query, Limit: limit})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, catalog.apiBase+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("x-api-key", catalog.apiKey)
	if userID = strings.TrimSpace(userID); userID != "" {
		request.Header.Set("x-user-id", userID)
	}
	response, err := catalog.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("search Channel3: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return nil, fmt.Errorf("%w: Channel3 returned HTTP %d", ErrProviderUnavailable, response.StatusCode)
	}
	var payload channel3SearchResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Channel3 response: %w", err)
	}
	products := make([]Product, 0, len(payload.Products))
	for _, item := range payload.Products {
		product := Product{ID: item.ID, Title: item.Title}
		if len(item.Brands) > 0 {
			product.Brand = item.Brands[0].Name
		}
		for _, image := range item.Images {
			if image.IsMain || product.ImageURL == "" {
				product.ImageURL = image.CleanedURL
				if product.ImageURL == "" {
					product.ImageURL = image.URL
				}
			}
			if image.IsMain {
				break
			}
		}
		for _, offer := range item.Offers {
			amount := offer.Price.Price
			if amount == 0 {
				amount = offer.Price.Amount
			}
			product.Offers = append(product.Offers, Offer{
				Domain: offer.Domain, URL: offer.URL, PriceMinor: int64(amount*100 + 0.5),
				Currency: strings.ToUpper(offer.Price.Currency), Availability: offer.Availability,
				Condition: strings.ToLower(offer.Condition),
			})
		}
		products = append(products, product)
	}
	return products, nil
}

type channel3SearchResponse struct {
	Products []struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Brands []struct {
			Name string `json:"name"`
		} `json:"brands"`
		Images []struct {
			URL        string `json:"url"`
			CleanedURL string `json:"cleaned_url"`
			IsMain     bool   `json:"is_main_image"`
		} `json:"images"`
		Offers []struct {
			URL          string `json:"url"`
			Domain       string `json:"domain"`
			Availability string `json:"availability"`
			Condition    string `json:"condition"`
			Price        struct {
				Price    float64 `json:"price"`
				Amount   float64 `json:"amount"`
				Currency string  `json:"currency"`
			} `json:"price"`
		} `json:"offers"`
	} `json:"products"`
}
