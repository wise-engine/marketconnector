package angelone

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/endpoints"
	"github.com/sunnyme20/marketconnector/brokers/angelone/internal/wire"
	"github.com/sunnyme20/marketconnector/model"
)

// NewSession authenticates with the Angel One API using client code, API key,
// password and a TOTP code, and stores the resulting tokens on the broker.
func (a *Angelone) NewSession(clientCode, apiKey, password, totp string) (*model.Response[model.LoginResponse], error) {
	a.clientCode = clientCode
	a.apiKey = apiKey

	httpClient := a.httpClient()
	httpClient.SetAccessToken("")

	req := wire.LoginRequest{
		ClientCode: clientCode,
		Password:   password,
		TOTP:       totp,
	}

	var resp wire.LoginResponse
	if err := httpClient.Post(endpoints.API.Login, req, &resp); err != nil {
		return nil, err
	}

	data, err := wire.Decode[wire.LoginData](resp.Envelope, resp.Data)
	if err != nil {
		return nil, err
	}

	a.SetAccessToken(data.JWTToken)
	a.SetFeedToken(data.FeedToken)
	a.SetRefreshToken(data.RefreshToken)
	httpClient.SetAccessToken(data.JWTToken)

	return &model.Response[model.LoginResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
		Data: model.LoginResponse{
			AccessToken:  data.JWTToken,
			FeedToken:    data.FeedToken,
			RefreshToken: data.RefreshToken,
		},
	}, nil
}

// GetUserProfile returns the profile of the authenticated user. The Angel One
// getProfile endpoint requires the refresh token obtained at login, passed as a
// JSON-encoded query parameter.
func (a *Angelone) GetUserProfile() (*model.Response[model.UserProfileResponse], error) {
	httpClient := a.httpClient()
	httpClient.SetAccessToken(a.accessToken)

	if a.refreshToken == "" {
		return nil, fmt.Errorf("getProfile requires a refresh token; call NewSession first")
	}

	payload, _ := json.Marshal(map[string]string{"refreshToken": a.refreshToken})
	query := strings.ReplaceAll(url.QueryEscape(string(payload)), "+", "%20")
	profileURL := endpoints.API.Profile + "?" + query

	var resp wire.Profile
	if err := httpClient.Get(profileURL, nil, &resp); err != nil {
		return nil, err
	}

	data, err := wire.Decode[wire.ProfileData](resp.Envelope, resp.Data)
	if err != nil {
		return nil, err
	}

	return &model.Response[model.UserProfileResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
		Data: model.UserProfileResponse{
			ClientCode: data.ClientCode,
			Username:   data.Username,
			Email:      data.Email,
			Exchanges:  data.Exchanges,
			Products:   data.Products,
		},
	}, nil
}

// GetRMSData returns the available funds and margin for the account.
func (a *Angelone) GetRMSData() (*model.Response[model.FundsResponse], error) {
	httpClient := a.httpClient()
	httpClient.SetAccessToken(a.accessToken)

	var resp wire.Funds
	if err := httpClient.Get(endpoints.API.RMS, nil, &resp); err != nil {
		return nil, err
	}

	data, err := wire.Decode[wire.FundsData](resp.Envelope, resp.Data)
	if err != nil {
		return nil, err
	}

	netMargin, _ := strconv.ParseFloat(data.NetMargin, 64)
	availableCash, _ := strconv.ParseFloat(data.AvailableCash, 64)

	return &model.Response[model.FundsResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "angelone",
		Data: model.FundsResponse{
			NetMargin:     netMargin,
			AvailableCash: availableCash,
		},
	}, nil
}
