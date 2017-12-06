package comms

import (
	"encoding/json"
	"fmt"
	"github.com/caarlos0/env"
	"github.com/keroserene/go-webrtc"
	"net/http"
	"net/url"
	"strings"
)

type authConfig struct {
	TwilioSid   string `env:"TWILIO_SID"`
	TwilioToken string `env:"TWILIO_TOKEN"`
}

type TwilioClient struct {
	authConfig *authConfig
}

type TwilioTokensResponse struct {
	IceServers []TwilioIceServer `json:"ice_servers"`
}

type TwilioIceServer struct {
	Url        string
	Credential string
	Username   string
}

func NewTwilioClient() (client *TwilioClient, err error) {
	client = new(TwilioClient)
	client.authConfig = new(authConfig)
	env.Parse(client.authConfig)

	if client.authConfig.TwilioSid == "" || client.authConfig.TwilioToken == "" {
		return nil, fmt.Errorf("unable to parse env varables to get twilio config")
	}

	return
}

func (tc *TwilioClient) IceServers() (iceServers []webrtc.IceServer, err error) {
	client := &http.Client{}

	u := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Tokens.json", tc.authConfig.TwilioSid)
	form := url.Values{}
	form.Add("Ttl", "21600")
	req, err := http.NewRequest("POST", u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("unable to generate request: %v", err)
	}

	req.SetBasicAuth(tc.authConfig.TwilioSid, tc.authConfig.TwilioToken)
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("unable to get response: %v", err)
	}

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("twilio server returned status code %d", resp.StatusCode)
	}

	var tokens TwilioTokensResponse
	err = json.NewDecoder(resp.Body).Decode(&tokens)
	if err != nil {
		return nil, fmt.Errorf("unable to read JSON response: %v", err)
	}

	if len(tokens.IceServers) == 0 {
		return nil, fmt.Errorf("JSON did not contain any ice servers: %v", err)
	}

	for _, ices := range tokens.IceServers {
		fmt.Printf("adding server %v\n", ices)
		var server webrtc.IceServer
		server.Urls = append(server.Urls, ices.Url)
		server.Username = ices.Username
		server.Credential = ices.Credential
		iceServers = append(iceServers, server)
	}

	return
}
