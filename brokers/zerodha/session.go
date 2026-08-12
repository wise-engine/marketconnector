package zerodha

import (
	"fmt"

	"github.com/sunnyme20/marketconnector/model"
)

// NewSession authenticates with Zerodha and stores the resulting tokens on the
// broker.
//
// Zerodha uses a different login flow than Angel One: instead of a password and
// TOTP it exchanges an oauth request_token (obtained from the Kite Connect
// login URL redirect) with the api_secret. To keep the broker-agnostic
// signature, the parameters map as follows:
//
//	clientcode = request_token  (from the Kite Connect login redirect URL)
//	apikey     = API key
//	password   = API secret
//	totp       = unused
func (z *Zerodha) NewSession(requestToken, apiKey, apiSecret, _ string) (*model.Response[model.LoginResponse], error) {
	z.clientCode = requestToken
	z.apiKey = apiKey
	z.apiSecret = apiSecret
	z.client = nil

	session, err := z.kite().GenerateSession(requestToken, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("zerodha: generate session: %w", err)
	}

	z.SetAccessToken(session.AccessToken)
	z.SetRefreshToken(session.RefreshToken)

	return &model.Response[model.LoginResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data: model.LoginResponse{
			AccessToken:  session.AccessToken,
			RefreshToken: session.RefreshToken,
			// Zerodha has no feed token; the WebSocket uses the access token.
			FeedToken: "",
		},
	}, nil
}

// GetUserProfile returns the profile of the authenticated user.
func (z *Zerodha) GetUserProfile() (*model.Response[model.UserProfileResponse], error) {
	profile, err := z.kite().GetUserProfile()
	if err != nil {
		return nil, fmt.Errorf("zerodha: get user profile: %w", err)
	}

	return &model.Response[model.UserProfileResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data: model.UserProfileResponse{
			ClientCode: profile.UserID,
			Username:   profile.UserName,
			Email:      profile.Email,
			Exchanges:  profile.Exchanges,
			Products:   profile.Products,
		},
	}, nil
}

// GetRMSData returns the available funds and margin for the account. The
// equity segment margins are used, matching the common funds shape.
func (z *Zerodha) GetRMSData() (*model.Response[model.FundsResponse], error) {
	margins, err := z.kite().GetUserMargins()
	if err != nil {
		return nil, fmt.Errorf("zerodha: get user margins: %w", err)
	}

	return &model.Response[model.FundsResponse]{
		Success: true,
		Message: "SUCCESS",
		Broker:  "zerodha",
		Data: model.FundsResponse{
			NetMargin:     margins.Equity.Net,
			AvailableCash: margins.Equity.Available.Cash,
		},
	}, nil
}
