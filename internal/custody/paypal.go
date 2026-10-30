package custody

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/raagan-u/controlled/internal/money"
)

type PayPalOptions struct {
	APIBase      string
	CancelURL    string
	Client       *http.Client
	ClientID     string
	ClientSecret string
	ReturnURL    string
}

type PayPal struct {
	apiBase      string
	cancelURL    string
	client       *http.Client
	clientID     string
	clientSecret string
	returnURL    string
	token        string
	tokenExpiry  time.Time
	tokenMu      sync.Mutex
}

func NewPayPal(options PayPalOptions) (*PayPal, error) {
	options.APIBase = strings.TrimRight(strings.TrimSpace(options.APIBase), "/")
	options.ClientID = strings.TrimSpace(options.ClientID)
	options.ClientSecret = strings.TrimSpace(options.ClientSecret)
	options.ReturnURL = strings.TrimSpace(options.ReturnURL)
	options.CancelURL = strings.TrimSpace(options.CancelURL)
	if options.APIBase == "" || options.ClientID == "" || options.ClientSecret == "" || options.ReturnURL == "" || options.CancelURL == "" {
		return nil, ErrInvalidRequest
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 15 * time.Second}
	}
	return &PayPal{
		apiBase: options.APIBase, cancelURL: options.CancelURL, client: options.Client,
		clientID: options.ClientID, clientSecret: options.ClientSecret, returnURL: options.ReturnURL,
	}, nil
}

func (p *PayPal) CreateFundingOrder(ctx context.Context, request CreateFundingOrder) (FundingOrder, error) {
	if p == nil || strings.TrimSpace(request.IdempotencyKey) == "" || !request.Amount.Positive() {
		return FundingOrder{}, ErrInvalidRequest
	}
	body := struct {
		Intent        string `json:"intent"`
		PurchaseUnits []struct {
			Amount struct {
				Currency string `json:"currency_code"`
				Value    string `json:"value"`
			} `json:"amount"`
			CustomID    string `json:"custom_id"`
			Description string `json:"description"`
			ReferenceID string `json:"reference_id"`
		} `json:"purchase_units"`
		PaymentSource struct {
			PayPal struct {
				ExperienceContext struct {
					CancelURL  string `json:"cancel_url"`
					ReturnURL  string `json:"return_url"`
					UserAction string `json:"user_action"`
				} `json:"experience_context"`
			} `json:"paypal"`
		} `json:"payment_source"`
	}{Intent: "CAPTURE", PurchaseUnits: make([]struct {
		Amount struct {
			Currency string `json:"currency_code"`
			Value    string `json:"value"`
		} `json:"amount"`
		CustomID    string `json:"custom_id"`
		Description string `json:"description"`
		ReferenceID string `json:"reference_id"`
	}, 1)}
	body.PurchaseUnits[0].ReferenceID = request.IdempotencyKey
	body.PurchaseUnits[0].CustomID = request.IdempotencyKey
	body.PurchaseUnits[0].Description = "Fund Controlled sandbox wallet"
	body.PurchaseUnits[0].Amount.Currency = request.Amount.Currency()
	body.PurchaseUnits[0].Amount.Value = decimalAmount(request.Amount)
	body.PaymentSource.PayPal.ExperienceContext.UserAction = "PAY_NOW"
	body.PaymentSource.PayPal.ExperienceContext.ReturnURL = p.returnURL
	body.PaymentSource.PayPal.ExperienceContext.CancelURL = p.cancelURL
	var response struct {
		ID    string `json:"id"`
		Links []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	if err := p.request(ctx, http.MethodPost, "/v2/checkout/orders", paypalRequestID("fund", request.IdempotencyKey), body, &response); err != nil {
		return FundingOrder{}, err
	}
	approvalURL := ""
	for _, link := range response.Links {
		if link.Rel == "payer-action" || link.Rel == "approve" {
			approvalURL = link.Href
			break
		}
	}
	if strings.TrimSpace(response.ID) == "" || approvalURL == "" {
		return FundingOrder{}, ErrUnavailable
	}
	return FundingOrder{ID: response.ID, Amount: request.Amount, Status: FundingAwaitingApproval, ApprovalURL: approvalURL}, nil
}

func (p *PayPal) CaptureFundingOrder(ctx context.Context, orderID string) (FundingOrder, error) {
	orderID = strings.TrimSpace(orderID)
	if p == nil || orderID == "" {
		return FundingOrder{}, ErrInvalidRequest
	}
	var response struct {
		ID            string `json:"id"`
		PurchaseUnits []struct {
			Payments struct {
				Captures []struct {
					Amount struct {
						Currency string `json:"currency_code"`
						Value    string `json:"value"`
					} `json:"amount"`
					Status string `json:"status"`
				} `json:"captures"`
			} `json:"payments"`
		} `json:"purchase_units"`
		Status string `json:"status"`
	}
	if err := p.request(ctx, http.MethodPost, "/v2/checkout/orders/"+url.PathEscape(orderID)+"/capture", paypalRequestID("capture", orderID), struct{}{}, &response); err != nil {
		return FundingOrder{}, err
	}
	if response.Status != "COMPLETED" || len(response.PurchaseUnits) == 0 || len(response.PurchaseUnits[0].Payments.Captures) == 0 {
		return FundingOrder{}, ErrUnavailable
	}
	capture := response.PurchaseUnits[0].Payments.Captures[0]
	if capture.Status != "COMPLETED" {
		return FundingOrder{}, ErrUnavailable
	}
	amount, err := parsePayPalAmount(capture.Amount.Value, capture.Amount.Currency)
	if err != nil {
		return FundingOrder{}, ErrUnavailable
	}
	return FundingOrder{ID: orderID, Amount: amount, Status: FundingCaptured}, nil
}

func (p *PayPal) CreatePayout(ctx context.Context, request CreatePayout) (Payout, error) {
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	request.Destination = strings.TrimSpace(request.Destination)
	if p == nil || request.IdempotencyKey == "" || request.Destination == "" || !request.Amount.Positive() {
		return Payout{}, ErrInvalidRequest
	}
	batchID := paypalReference("batch", request.IdempotencyKey)
	itemID := paypalReference("item", request.IdempotencyKey)
	body := struct {
		Header struct {
			EmailMessage  string `json:"email_message"`
			EmailSubject  string `json:"email_subject"`
			SenderBatchID string `json:"sender_batch_id"`
		} `json:"sender_batch_header"`
		Items []struct {
			Amount struct {
				Currency string `json:"currency"`
				Value    string `json:"value"`
			} `json:"amount"`
			Note          string `json:"note"`
			Receiver      string `json:"receiver"`
			RecipientType string `json:"recipient_type"`
			SenderItemID  string `json:"sender_item_id"`
		} `json:"items"`
	}{Items: make([]struct {
		Amount struct {
			Currency string `json:"currency"`
			Value    string `json:"value"`
		} `json:"amount"`
		Note          string `json:"note"`
		Receiver      string `json:"receiver"`
		RecipientType string `json:"recipient_type"`
		SenderItemID  string `json:"sender_item_id"`
	}, 1)}
	body.Header.SenderBatchID = batchID
	body.Header.EmailSubject = "Your Controlled Sandbox withdrawal"
	body.Header.EmailMessage = "A Controlled Sandbox withdrawal was sent to your PayPal account."
	body.Items[0].RecipientType = "EMAIL"
	body.Items[0].Receiver = request.Destination
	body.Items[0].SenderItemID = itemID
	body.Items[0].Amount.Value = decimalAmount(request.Amount)
	body.Items[0].Amount.Currency = request.Amount.Currency()
	body.Items[0].Note = "Controlled Sandbox wallet withdrawal"
	var response struct {
		Header struct {
			BatchID string `json:"payout_batch_id"`
			Status  string `json:"batch_status"`
		} `json:"batch_header"`
	}
	if err := p.request(ctx, http.MethodPost, "/v1/payments/payouts", paypalRequestID("payout", request.IdempotencyKey), body, &response); err != nil {
		return Payout{}, err
	}
	if response.Header.BatchID == "" {
		return Payout{}, ErrUnavailable
	}
	status := PayoutPending
	if response.Header.Status == "DENIED" || response.Header.Status == "CANCELED" {
		status = PayoutFailed
	}
	return Payout{ID: response.Header.BatchID, Amount: request.Amount, Destination: request.Destination, Status: status}, nil
}

func (p *PayPal) GetPayout(ctx context.Context, payoutID string) (Payout, error) {
	payoutID = strings.TrimSpace(payoutID)
	if p == nil || payoutID == "" {
		return Payout{}, ErrInvalidRequest
	}
	var response struct {
		Header struct {
			Status string `json:"batch_status"`
		} `json:"batch_header"`
		Items []struct {
			PayoutItem struct {
				Amount struct {
					Currency string `json:"currency"`
					Value    string `json:"value"`
				} `json:"amount"`
				Receiver string `json:"receiver"`
			} `json:"payout_item"`
			Status string `json:"transaction_status"`
		} `json:"items"`
	}
	if err := p.request(ctx, http.MethodGet, "/v1/payments/payouts/"+url.PathEscape(payoutID)+"?page_size=100&page=1&total_required=true", "", nil, &response); err != nil {
		return Payout{}, err
	}
	result := Payout{ID: payoutID, Status: payoutStatus(response.Header.Status, "")}
	if len(response.Items) > 0 {
		item := response.Items[0]
		result.Status = payoutStatus(response.Header.Status, item.Status)
		result.Destination = item.PayoutItem.Receiver
		if parsed, err := parsePayPalAmount(item.PayoutItem.Amount.Value, item.PayoutItem.Amount.Currency); err == nil {
			result.Amount = parsed
		}
	}
	return result, nil
}

func (p *PayPal) request(ctx context.Context, method, path, requestID string, input, output any) error {
	token, err := p.accessToken(ctx)
	if err != nil {
		return err
	}
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return ErrInvalidRequest
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, p.apiBase+path, body)
	if err != nil {
		return ErrInvalidRequest
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if requestID != "" {
		request.Header.Set("PayPal-Request-Id", requestID)
	}
	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return ErrPayoutNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, response.Body)
		return ErrUnavailable
	}
	if output == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(output); err != nil {
		return fmt.Errorf("%w: decode response", ErrUnavailable)
	}
	return nil
}

func (p *PayPal) accessToken(ctx context.Context) (string, error) {
	p.tokenMu.Lock()
	defer p.tokenMu.Unlock()
	if p.token != "" && time.Now().Add(30*time.Second).Before(p.tokenExpiry) {
		return p.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiBase+"/v1/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", ErrInvalidRequest
	}
	request.SetBasicAuth(p.clientID, p.clientSecret)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Language", "en_US")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := p.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, response.Body)
		return "", ErrUnavailable
	}
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&token); err != nil || token.AccessToken == "" {
		return "", ErrUnavailable
	}
	p.token = token.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return p.token, nil
}

func decimalAmount(value money.Money) string {
	minor := value.AmountMinor()
	return fmt.Sprintf("%d.%02d", minor/100, minor%100)
}

func parsePayPalAmount(value, currency string) (money.Money, error) {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) > 2 || len(parts) == 0 || parts[0] == "" {
		return money.Money{}, errors.New("invalid PayPal amount")
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return money.Money{}, errors.New("invalid PayPal amount")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return money.Money{}, errors.New("invalid PayPal amount")
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	cents, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return money.Money{}, errors.New("invalid PayPal amount")
	}
	return money.New(whole*100+cents, strings.ToUpper(strings.TrimSpace(currency)))
}

func paypalRequestID(prefix, key string) string {
	return paypalReference(prefix, key)
}

func paypalReference(prefix, key string) string {
	sum := sha256.Sum256([]byte(key))
	return prefix + "-" + hex.EncodeToString(sum[:12])
}

func payoutStatus(batch, item string) PayoutStatus {
	switch item {
	case "SUCCESS":
		return PayoutSucceeded
	case "FAILED", "BLOCKED", "RETURNED", "REFUNDED", "REVERSED":
		return PayoutFailed
	case "PENDING", "UNCLAIMED", "ONHOLD":
		return PayoutPending
	}
	if batch == "DENIED" || batch == "CANCELED" {
		return PayoutFailed
	}
	return PayoutPending
}
