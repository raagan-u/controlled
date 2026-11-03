package shopping

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode"

	"github.com/raagan-u/controlled/internal/platform/httpapi"
	"go.uber.org/zap"
)

var ErrInvalidInput = errors.New("invalid purchase price check input")

type Service struct{ catalog Catalog }

func NewService(catalog Catalog) *Service { return &Service{catalog: catalog} }

type PriceCheck struct {
	Query               string `json:"query"`
	Matched             bool   `json:"matched"`
	ProductID           string `json:"productId,omitempty"`
	ProductTitle        string `json:"productTitle,omitempty"`
	Brand               string `json:"brand,omitempty"`
	ImageURL            string `json:"imageUrl,omitempty"`
	BestPriceMinor      int64  `json:"bestPriceMinor,omitempty"`
	IntendedAmountMinor int64  `json:"intendedAmountMinor"`
	SavingsMinor        int64  `json:"savingsMinor,omitempty"`
	Currency            string `json:"currency"`
	Merchant            string `json:"merchant,omitempty"`
	OfferURL            string `json:"offerUrl,omitempty"`
	Verdict             string `json:"verdict,omitempty"`
	Message             string `json:"message,omitempty"`
}

func (service *Service) CheckPrice(ctx context.Context, userID, query string, intendedMinor int64, currency string) (PriceCheck, error) {
	query, currency = strings.TrimSpace(query), strings.ToUpper(strings.TrimSpace(currency))
	if len(query) < 3 || len(query) > 160 || intendedMinor <= 0 || currency != "USD" {
		return PriceCheck{}, ErrInvalidInput
	}
	if service.catalog == nil {
		return PriceCheck{}, ErrProviderUnavailable
	}
	logger := httpapi.Logger(ctx).With(
		zap.Int("query_length", len(query)),
		zap.Int64("intended_amount_minor", intendedMinor),
		zap.String("currency", currency),
	)
	logger.Info("Purchase price check started")
	products, err := service.catalog.Search(ctx, userID, query, 5)
	if err != nil {
		logger.Error("Purchase catalog search failed", zap.Error(err))
		return PriceCheck{}, err
	}
	result := PriceCheck{Query: query, IntendedAmountMinor: intendedMinor, Currency: currency}
	for _, product := range products {
		if lexicalMatch(query, product.Title+" "+product.Brand) < 0.34 {
			continue
		}
		offers := eligibleOffers(product.Offers, currency)
		if len(offers) == 0 {
			continue
		}
		best := offers[0]
		result.Matched = true
		result.ProductID, result.ProductTitle, result.Brand, result.ImageURL = product.ID, product.Title, product.Brand, product.ImageURL
		result.BestPriceMinor, result.Merchant, result.OfferURL = best.PriceMinor, best.Domain, best.URL
		if intendedMinor > best.PriceMinor {
			result.SavingsMinor = intendedMinor - best.PriceMinor
		}
		switch {
		case result.SavingsMinor >= 500 && best.PriceMinor*100 <= intendedMinor*95:
			result.Verdict = "better_price"
			result.Message = "A meaningfully lower price is available for this product."
		case intendedMinor*100 < best.PriceMinor*95:
			result.Verdict = "below_market"
			result.Message = "Your intended payment is already below the lowest matching offer we found."
		default:
			result.Verdict = "competitive"
			result.Message = "Your intended payment is close to the best matching offer we found."
		}
		logger.Info("Purchase price check completed", zap.Bool("matched", true), zap.String("verdict", result.Verdict), zap.Int64("best_price_minor", result.BestPriceMinor), zap.Int64("savings_minor", result.SavingsMinor))
		return result, nil
	}
	result.Message = "We could not confidently match that description to an exact product."
	logger.Info("Purchase price check completed", zap.Bool("matched", false), zap.Int("catalog_results", len(products)))
	return result, nil
}

func eligibleOffers(offers []Offer, currency string) []Offer {
	eligible := make([]Offer, 0, len(offers))
	for _, offer := range offers {
		if offer.PriceMinor <= 0 || offer.Currency != currency || offer.Availability != "InStock" || (offer.Condition != "" && offer.Condition != "new") || strings.TrimSpace(offer.URL) == "" {
			continue
		}
		eligible = append(eligible, offer)
	}
	sort.SliceStable(eligible, func(i, j int) bool { return eligible[i].PriceMinor < eligible[j].PriceMinor })
	return eligible
}

func lexicalMatch(query, candidate string) float64 {
	queryTokens := tokenSet(query)
	if len(queryTokens) == 0 {
		return 0
	}
	candidateTokens := tokenSet(candidate)
	matched := 0
	for token := range queryTokens {
		if _, ok := candidateTokens[token]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(queryTokens))
}

func tokenSet(value string) map[string]struct{} {
	stop := map[string]struct{}{"a": {}, "an": {}, "and": {}, "for": {}, "the": {}, "to": {}}
	parts := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	tokens := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if len(part) < 2 {
			continue
		}
		if _, ignored := stop[part]; !ignored {
			tokens[part] = struct{}{}
		}
	}
	return tokens
}
