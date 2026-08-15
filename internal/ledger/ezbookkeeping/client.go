package ezbookkeeping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

const (
	transactionPageSize = 50
	maxResponseBytes    = 16 << 20
)

var _ ledger.Ledger = (*Client)(nil)

type Client struct {
	baseURL  *url.URL
	token    string
	timezone string
	http     *http.Client
}

func NewClient(baseURL, token, timezone string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse ezbookkeeping base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("ezbookkeeping base URL must include scheme and host")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("ezbookkeeping API token is required")
	}
	if strings.TrimSpace(timezone) == "" {
		return nil, errors.New("ezbookkeeping timezone is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return &Client{
		baseURL:  parsed,
		token:    token,
		timezone: timezone,
		http:     httpClient,
	}, nil
}

func (c *Client) ListAccounts(ctx context.Context) ([]ledger.Account, error) {
	var result []apiAccount
	if err := c.get(ctx, "accounts/list.json", nil, &result); err != nil {
		return nil, err
	}

	accounts := make([]ledger.Account, 0, len(result))
	for _, raw := range result {
		if err := appendAccountTree(&accounts, raw); err != nil {
			return nil, err
		}
	}
	return accounts, nil
}

func (c *Client) ListCategories(ctx context.Context) ([]ledger.Category, error) {
	var result map[string][]apiCategory
	if err := c.get(ctx, "transaction/categories/list.json", nil, &result); err != nil {
		return nil, err
	}

	categories := make([]ledger.Category, 0)
	for rawType, items := range result {
		value, err := strconv.Atoi(rawType)
		if err != nil {
			return nil, fmt.Errorf("%w: category map key %q", ledger.ErrUnknownEnum, rawType)
		}
		expected, err := categoryType(value)
		if err != nil {
			return nil, err
		}
		for _, raw := range items {
			if err := appendCategoryTree(&categories, raw, expected); err != nil {
				return nil, err
			}
		}
	}
	return categories, nil
}

func (c *Client) ListTransactions(ctx context.Context, q ledger.TransactionQuery) ([]ledger.Transaction, error) {
	if q.Type != ledger.TransactionTypeUnknown && !validTransactionType(q.Type) {
		return nil, fmt.Errorf("%w: transaction query type %d", ledger.ErrUnknownEnum, q.Type)
	}

	cursor := int64(0)
	seen := map[int64]struct{}{0: {}}
	transactions := make([]ledger.Transaction, 0)

	for {
		params := url.Values{}
		params.Set("count", strconv.Itoa(transactionPageSize))
		params.Set("max_time", strconv.FormatInt(cursor, 10))
		params.Set("trim_account", "false")
		params.Set("trim_category", "true")
		params.Set("trim_tag", "true")
		params.Set("with_pictures", "false")
		if q.Type != ledger.TransactionTypeUnknown {
			params.Set("type", strconv.Itoa(int(q.Type)))
		}
		if len(q.CategoryIDs) > 0 {
			params.Set("category_ids", strings.Join(q.CategoryIDs, ","))
		}
		if len(q.AccountIDs) > 0 {
			params.Set("account_ids", strings.Join(q.AccountIDs, ","))
		}

		var page apiTransactionPage
		if err := c.get(ctx, "transactions/list.json", params, &page); err != nil {
			return nil, err
		}
		for _, raw := range page.Items {
			normalized, err := normalizeTransaction(raw)
			if err != nil {
				return nil, err
			}
			transactions = append(transactions, normalized)
		}

		if page.NextTimeSequenceID == 0 || len(page.Items) == 0 {
			return transactions, nil
		}
		if _, exists := seen[page.NextTimeSequenceID]; exists {
			return nil, fmt.Errorf("ezbookkeeping pagination cursor did not advance: %d", page.NextTimeSequenceID)
		}
		seen[page.NextTimeSequenceID] = struct{}{}
		cursor = page.NextTimeSequenceID
	}
}

func (c *Client) get(ctx context.Context, path string, query url.Values, result any) error {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/" + strings.TrimLeft(path, "/")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("build ezbookkeeping request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-Timezone-Name", c.timezone)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ezbookkeeping request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ezbookkeeping HTTP status %d", resp.StatusCode)
	}

	var envelope apiEnvelope
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes))
	if err := decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("decode ezbookkeeping response: %w", err)
	}
	if !envelope.Success {
		return fmt.Errorf("ezbookkeeping API error %d: %s", envelope.ErrorCode, envelope.ErrorMessage)
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode ezbookkeeping result: %w", err)
	}
	return nil
}

type apiEnvelope struct {
	Success      bool            `json:"success"`
	Result       json.RawMessage `json:"result"`
	ErrorCode    int64           `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage"`
}

type apiAccount struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	ParentID    string       `json:"parentId"`
	Category    int          `json:"category"`
	Type        int          `json:"type"`
	Currency    string       `json:"currency"`
	Balance     int64        `json:"balance"`
	IsAsset     bool         `json:"isAsset"`
	IsLiability bool         `json:"isLiability"`
	Hidden      bool         `json:"hidden"`
	SubAccounts []apiAccount `json:"subAccounts"`
}

type apiCategory struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	ParentID      string        `json:"parentId"`
	Type          int           `json:"type"`
	Hidden        bool          `json:"hidden"`
	SubCategories []apiCategory `json:"subCategories"`
}

type apiTransactionPage struct {
	Items              []apiTransaction `json:"items"`
	NextTimeSequenceID int64            `json:"nextTimeSequenceId"`
	TotalCount         int64            `json:"totalCount"`
}

type apiTransaction struct {
	ID                   string      `json:"id"`
	TimeSequenceID       string      `json:"timeSequenceId"`
	Type                 int         `json:"type"`
	CategoryID           string      `json:"categoryId"`
	Time                 int64       `json:"time"`
	UTCOffset            int         `json:"utcOffset"`
	SourceAccountID      string      `json:"sourceAccountId"`
	SourceAccount        *apiAccount `json:"sourceAccount"`
	DestinationAccountID string      `json:"destinationAccountId"`
	DestinationAccount   *apiAccount `json:"destinationAccount"`
	SourceAmount         int64       `json:"sourceAmount"`
	DestinationAmount    int64       `json:"destinationAmount"`
	HideAmount           bool        `json:"hideAmount"`
	Comment              string      `json:"comment"`
	Editable             bool        `json:"editable"`
}

func appendAccountTree(dst *[]ledger.Account, raw apiAccount) error {
	category, err := accountCategory(raw.Category)
	if err != nil {
		return err
	}
	structure, err := accountStructure(raw.Type)
	if err != nil {
		return err
	}
	*dst = append(*dst, ledger.Account{
		ID:          raw.ID,
		Name:        raw.Name,
		ParentID:    raw.ParentID,
		Category:    category,
		Structure:   structure,
		Balance:     money.Money{Minor: raw.Balance, Currency: raw.Currency},
		IsAsset:     raw.IsAsset,
		IsLiability: raw.IsLiability,
		Hidden:      raw.Hidden,
	})
	for _, child := range raw.SubAccounts {
		if err := appendAccountTree(dst, child); err != nil {
			return err
		}
	}
	return nil
}

func appendCategoryTree(dst *[]ledger.Category, raw apiCategory, expected ledger.CategoryType) error {
	typeValue, err := categoryType(raw.Type)
	if err != nil {
		return err
	}
	if typeValue != expected {
		return fmt.Errorf("category %s type %d does not match response group %d", raw.ID, typeValue, expected)
	}
	*dst = append(*dst, ledger.Category{
		ID:       raw.ID,
		Name:     raw.Name,
		ParentID: raw.ParentID,
		Type:     typeValue,
		Hidden:   raw.Hidden,
	})
	for _, child := range raw.SubCategories {
		if err := appendCategoryTree(dst, child, expected); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTransaction(raw apiTransaction) (ledger.Transaction, error) {
	typeValue, err := transactionType(raw.Type)
	if err != nil {
		return ledger.Transaction{}, err
	}
	if raw.SourceAccount == nil || strings.TrimSpace(raw.SourceAccount.Currency) == "" {
		return ledger.Transaction{}, fmt.Errorf("transaction %s source account currency is missing", raw.ID)
	}

	normalized := ledger.Transaction{
		ID:                   raw.ID,
		TimeSequenceID:       raw.TimeSequenceID,
		Type:                 typeValue,
		CategoryID:           raw.CategoryID,
		OccurredAt:           time.Unix(raw.Time, 0).UTC(),
		UTCOffsetMinutes:     raw.UTCOffset,
		SourceAccountID:      raw.SourceAccountID,
		DestinationAccountID: raw.DestinationAccountID,
		SourceAmount:         money.Money{Minor: raw.SourceAmount, Currency: raw.SourceAccount.Currency},
		AmountHidden:         raw.HideAmount,
		Comment:              raw.Comment,
		Editable:             raw.Editable,
	}

	if raw.DestinationAccountID != "" && raw.DestinationAccountID != "0" {
		if raw.DestinationAccount == nil || strings.TrimSpace(raw.DestinationAccount.Currency) == "" {
			return ledger.Transaction{}, fmt.Errorf("transaction %s destination account currency is missing", raw.ID)
		}
		amount := money.Money{Minor: raw.DestinationAmount, Currency: raw.DestinationAccount.Currency}
		normalized.DestinationAmount = &amount
	}
	return normalized, nil
}

func accountCategory(value int) (ledger.AccountCategory, error) {
	if value < int(ledger.AccountCategoryCash) || value > int(ledger.AccountCategoryCertificateOfDeposit) {
		return ledger.AccountCategoryUnknown, fmt.Errorf("%w: account category %d", ledger.ErrUnknownEnum, value)
	}
	return ledger.AccountCategory(value), nil
}

func accountStructure(value int) (ledger.AccountStructure, error) {
	if value < int(ledger.AccountStructureSingle) || value > int(ledger.AccountStructureMultipleSubAccounts) {
		return ledger.AccountStructureUnknown, fmt.Errorf("%w: account structure %d", ledger.ErrUnknownEnum, value)
	}
	return ledger.AccountStructure(value), nil
}

func categoryType(value int) (ledger.CategoryType, error) {
	if value < int(ledger.CategoryTypeIncome) || value > int(ledger.CategoryTypeTransfer) {
		return ledger.CategoryTypeUnknown, fmt.Errorf("%w: category type %d", ledger.ErrUnknownEnum, value)
	}
	return ledger.CategoryType(value), nil
}

func transactionType(value int) (ledger.TransactionType, error) {
	if value < int(ledger.TransactionTypeBalanceModification) || value > int(ledger.TransactionTypeTransfer) {
		return ledger.TransactionTypeUnknown, fmt.Errorf("%w: transaction type %d", ledger.ErrUnknownEnum, value)
	}
	return ledger.TransactionType(value), nil
}

func validTransactionType(value ledger.TransactionType) bool {
	return value >= ledger.TransactionTypeBalanceModification && value <= ledger.TransactionTypeTransfer
}
