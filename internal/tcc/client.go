package tcc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	DefaultBaseURL = "https://mytotalconnectcomfort.com"

	// Paths
	LoginPath      = "/portal"
	ZoneListPath   = "/portal/Device/GetZoneListData"
	DeviceDataPath = "/portal/Device/CheckDataSession/%d"
	ControlPath    = "/portal/Device/SubmitControlScreenChanges"

	// Rate limiting: minimum 10 minutes between polls
	MinPollInterval = 10 * time.Minute
)

// Client is a TCC API client
type Client struct {
	baseURL   string
	session   *Session
	limiter   *rate.Limiter
	lastPoll  time.Time
	pollMu    sync.Mutex
	devices   []ThermostatState
	devicesMu sync.RWMutex
}

// NewClient creates a new TCC client
func NewClient(baseURL string) (*Client, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	session, err := NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Rate limiter: 1 request per minute with burst of 5
	limiter := rate.NewLimiter(rate.Every(time.Minute), 5)

	return &Client{
		baseURL: baseURL,
		session: session,
		limiter: limiter,
	}, nil
}

// SetCredentials sets the login credentials
func (c *Client) SetCredentials(username, password string) {
	c.session.SetCredentials(username, password)
}

// Login authenticates with the TCC service
func (c *Client) Login(ctx context.Context) error {
	username, password := c.session.GetCredentials()
	if username == "" || password == "" {
		return fmt.Errorf("credentials not set")
	}

	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit wait: %w", err)
	}

	// First, get the login page to get any required tokens
	loginURL := c.baseURL + LoginPath
	req, err := http.NewRequestWithContext(ctx, "GET", loginURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create login page request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.session.GetClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to get login page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login page: %w", err)
	}

	// Extract RequestVerificationToken if present
	token := extractVerificationToken(string(body))

	// Prepare login form data
	formData := url.Values{}
	formData.Set("UserName", username)
	formData.Set("Password", password)
	formData.Set("RememberMe", "false")
	if token != "" {
		formData.Set("__RequestVerificationToken", token)
	}

	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit wait: %w", err)
	}

	// Submit login
	req, err = http.NewRequestWithContext(ctx, "POST", loginURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = c.session.GetClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to submit login: %w", err)
	}
	defer resp.Body.Close()

	// Check for successful login (redirect to portal or home)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusFound {
		// Verify we're logged in by checking the redirect or response
		location := resp.Header.Get("Location")
		if location != "" && !strings.Contains(location, "Login") && !strings.Contains(location, "Error") {
			c.session.MarkAuthenticated()
			return nil
		}

		// Read response body to check for login success
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "LogoutLink") || strings.Contains(string(body), "Welcome") {
			c.session.MarkAuthenticated()
			return nil
		}
	}

	return fmt.Errorf("login failed: unexpected response %d", resp.StatusCode)
}

// IsAuthenticated returns true if the client is authenticated
func (c *Client) IsAuthenticated() bool {
	return c.session.IsAuthenticated()
}

// GetDevices retrieves all thermostat devices
func (c *Client) GetDevices(ctx context.Context) ([]ThermostatState, error) {
	if !c.session.IsAuthenticated() {
		if err := c.Login(ctx); err != nil {
			return nil, fmt.Errorf("login required: %w", err)
		}
	}

	// Check poll interval
	c.pollMu.Lock()
	if time.Since(c.lastPoll) < MinPollInterval && len(c.devices) > 0 {
		c.pollMu.Unlock()
		c.devicesMu.RLock()
		defer c.devicesMu.RUnlock()
		return c.devices, nil
	}
	c.pollMu.Unlock()

	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	// Get zone list
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+ZoneListPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zone list request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.session.GetClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		c.session.MarkUnauthenticated()
		return nil, fmt.Errorf("session expired")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read zone list: %w", err)
	}

	// Parse response
	var devices []ThermostatState

	// Try to parse as array first
	var zones []ZoneData
	if err := json.Unmarshal(body, &zones); err == nil {
		for _, z := range zones {
			devices = append(devices, ThermostatState{
				DeviceID:     z.DeviceID,
				Name:         z.Name,
				CurrentTemp:  z.CurrentTemp,
				HeatSetpoint: z.HeatSetpoint,
				CoolSetpoint: z.CoolSetpoint,
				SystemMode:   SystemModeFromTCC(z.SystemSwitchPos),
				Humidity:     z.IndoorHumidity,
				IsHeating:    IsEquipmentHeating(z.EquipmentStatus),
				IsCooling:    IsEquipmentCooling(z.EquipmentStatus),
				UpdatedAt:    time.Now(),
			})
		}
	} else {
		// Try to parse as locations response
		var locResp []LocationData
		if err := json.Unmarshal(body, &locResp); err == nil {
			for _, loc := range locResp {
				for _, z := range loc.Devices {
					devices = append(devices, ThermostatState{
						DeviceID:     z.DeviceID,
						Name:         z.Name,
						CurrentTemp:  z.CurrentTemp,
						HeatSetpoint: z.HeatSetpoint,
						CoolSetpoint: z.CoolSetpoint,
						SystemMode:   SystemModeFromTCC(z.SystemSwitchPos),
						Humidity:     z.IndoorHumidity,
						IsHeating:    IsEquipmentHeating(z.EquipmentStatus),
						IsCooling:    IsEquipmentCooling(z.EquipmentStatus),
						UpdatedAt:    time.Now(),
					})
				}
			}
		}
	}

	// Update cached devices
	c.devicesMu.Lock()
	c.devices = devices
	c.devicesMu.Unlock()

	c.pollMu.Lock()
	c.lastPoll = time.Now()
	c.pollMu.Unlock()

	c.session.RefreshSession()

	return devices, nil
}

// GetDeviceData retrieves detailed data for a specific device
func (c *Client) GetDeviceData(ctx context.Context, deviceID int) (*ThermostatState, error) {
	if !c.session.IsAuthenticated() {
		if err := c.Login(ctx); err != nil {
			return nil, fmt.Errorf("login required: %w", err)
		}
	}

	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait: %w", err)
	}

	path := fmt.Sprintf(DeviceDataPath, deviceID)
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create device data request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.session.GetClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get device data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		c.session.MarkUnauthenticated()
		return nil, fmt.Errorf("session expired")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read device data: %w", err)
	}

	// Parse response
	var dataResp struct {
		LatestData struct {
			UIData UIData `json:"uiData"`
		} `json:"latestData"`
	}
	if err := json.Unmarshal(body, &dataResp); err != nil {
		return nil, fmt.Errorf("failed to parse device data: %w", err)
	}

	ui := dataResp.LatestData.UIData
	state := &ThermostatState{
		DeviceID:     deviceID,
		CurrentTemp:  ui.DispTemperature,
		HeatSetpoint: ui.HeatSetpoint,
		CoolSetpoint: ui.CoolSetpoint,
		SystemMode:   SystemModeFromTCC(ui.SystemSwitchPosition),
		Humidity:     ui.IndoorHumidity,
		IsHeating:    IsEquipmentHeating(ui.EquipmentOutputStatus),
		IsCooling:    IsEquipmentCooling(ui.EquipmentOutputStatus),
		Units:        ui.DisplayedUnits,
		UpdatedAt:    time.Now(),
	}

	c.session.RefreshSession()

	return state, nil
}

// SetHeatSetpoint sets the heating setpoint
func (c *Client) SetHeatSetpoint(ctx context.Context, deviceID int, temp float64) error {
	return c.submitControl(ctx, ControlRequest{
		DeviceID:       deviceID,
		HeatSetpoint:   &temp,
		StatusHeat:     intPtr(1), // Hold
		HeatNextPeriod: intPtr(0),
	})
}

// SetCoolSetpoint sets the cooling setpoint
func (c *Client) SetCoolSetpoint(ctx context.Context, deviceID int, temp float64) error {
	return c.submitControl(ctx, ControlRequest{
		DeviceID:       deviceID,
		CoolSetpoint:   &temp,
		StatusCool:     intPtr(1), // Hold
		CoolNextPeriod: intPtr(0),
	})
}

// SetSystemMode sets the system mode
func (c *Client) SetSystemMode(ctx context.Context, deviceID int, mode string) error {
	tccMode := SystemModeToTCC(mode)
	return c.submitControl(ctx, ControlRequest{
		DeviceID:     deviceID,
		SystemSwitch: &tccMode,
	})
}

// submitControl sends a control request to TCC
func (c *Client) submitControl(ctx context.Context, req ControlRequest) error {
	if !c.session.IsAuthenticated() {
		if err := c.Login(ctx); err != nil {
			return fmt.Errorf("login required: %w", err)
		}
	}

	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit wait: %w", err)
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal control request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+ControlPath, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create control request: %w", err)
	}
	c.setHeaders(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.session.GetClient().Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to submit control: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		c.session.MarkUnauthenticated()
		return fmt.Errorf("session expired")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("control request failed: %d - %s", resp.StatusCode, string(body))
	}

	c.session.RefreshSession()

	// Clear cache to force refresh on next poll
	c.pollMu.Lock()
	c.lastPoll = time.Time{}
	c.pollMu.Unlock()

	return nil
}

// setHeaders sets common headers for TCC requests
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
}

// TestConnection tests if the credentials are valid
func (c *Client) TestConnection(ctx context.Context) error {
	// Clear session to force fresh login
	c.session.ClearSession()

	if err := c.Login(ctx); err != nil {
		return err
	}

	// Try to get devices to confirm we're really logged in
	_, err := c.GetDevices(ctx)
	return err
}

// extractVerificationToken extracts the __RequestVerificationToken from HTML
func extractVerificationToken(html string) string {
	re := regexp.MustCompile(`name="__RequestVerificationToken"[^>]*value="([^"]+)"`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func intPtr(i int) *int {
	return &i
}
