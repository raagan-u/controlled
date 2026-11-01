package shopping

import (
	"errors"
	"net/http"

	"github.com/raagan-u/controlled/internal/identity"
	"github.com/raagan-u/controlled/internal/platform/httpapi"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (handler *Handler) CheckPrice(response http.ResponseWriter, request *http.Request) {
	user, ok := identity.CurrentUser(request.Context())
	if !ok {
		httpapi.WriteError(response, http.StatusUnauthorized, "authentication_required", "A valid Controlled session is required.")
		return
	}
	var input struct {
		Query       string `json:"query"`
		AmountMinor int64  `json:"amountMinor"`
		Currency    string `json:"currency"`
	}
	if err := httpapi.ReadJSON(response, request, &input, httpapi.DefaultMaxBodyBytes); err != nil {
		httpapi.WriteError(response, http.StatusBadRequest, "invalid_request", "Request body must contain a product description and amount.")
		return
	}
	result, err := handler.service.CheckPrice(request.Context(), string(user.ID()), input.Query, input.AmountMinor, input.Currency)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			httpapi.WriteError(response, http.StatusUnprocessableEntity, "invalid_price_check", "Add a specific product name or model for a USD price check.")
		case errors.Is(err, ErrProviderUnavailable):
			httpapi.WriteError(response, http.StatusServiceUnavailable, "price_check_unavailable", "Price checking is temporarily unavailable. Your payment is unaffected.")
		default:
			httpapi.WriteError(response, http.StatusBadGateway, "price_check_unavailable", "Price checking is temporarily unavailable. Your payment is unaffected.")
		}
		return
	}
	httpapi.WriteJSON(response, http.StatusOK, struct {
		PriceCheck PriceCheck `json:"priceCheck"`
	}{PriceCheck: result})
}
