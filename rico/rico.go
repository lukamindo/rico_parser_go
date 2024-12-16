package rico

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	url        = "https://www.rico.ge/ka"
	timezone   = "Asia/Tbilisi"
	timeFormat = "Jan 2 15:04:05"
)

type RateChecker struct {
	lastRate  float64
	botToken  string
	channelID string
	client    *http.Client
	location  *time.Location
}

// NewRateChecker creates a new instance of RateChecker with provided configuration.
func NewRateChecker(botToken, channelID string) (*RateChecker, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("failed to load timezone: %w", err)
	}

	rc := &RateChecker{
		lastRate:  0,
		botToken:  botToken,
		channelID: channelID,
		client: &http.Client{
			Timeout: 10 * time.Second, // set a reasonable timeout
		},
		location: loc,
	}
	return rc, nil
}

// CheckForRateChange checks if the rate has changed, and if so, sends a Telegram message.
func (rc *RateChecker) CheckForRateChange(ctx context.Context) {
	currRate, err := rc.fetchCurrentRate(ctx)
	if err != nil {
		log.Printf("Error fetching current rate: %v\n", err)
		return
	}

	// If there's no rate or zero, just log it. Zero might indicate a parsing issue.
	if currRate == 0 {
		log.Println("Fetched a rate of 0, which is unexpected; skipping message send.")
		return
	}

	if currRate == rc.lastRate {
		// No change in rate
		return
	}

	rc.lastRate = currRate
	if err := rc.sendTelegramMessage(ctx, currRate); err != nil {
		log.Printf("Error sending Telegram message: %v\n", err)
	}
}

// fetchCurrentRate retrieves the current exchange rate from the given URL.
func (rc *RateChecker) fetchCurrentRate(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}

	resp, err := rc.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("parsing HTML: %w", err)
	}

	rateText := doc.Find("#sell-usd").Text()
	if rateText == "" {
		return 0, fmt.Errorf("exchange rate not found on the page")
	}

	rateText = strings.TrimSpace(rateText)
	currRate, err := strconv.ParseFloat(rateText, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid exchange rate: %w", err)
	}

	return currRate, nil
}

// sendTelegramMessage sends the current exchange rate message to the specified Telegram channel.
func (rc *RateChecker) sendTelegramMessage(ctx context.Context, rate float64) error {
	currentDate := time.Now().In(rc.location)
	formattedTime := currentDate.Format(timeFormat)
	messageText := fmt.Sprintf("%s - 1$ ღირს %.4f", formattedTime, rate)

	telegramURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", rc.botToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, telegramURL, nil)
	if err != nil {
		return fmt.Errorf("creating telegram request: %w", err)
	}

	q := req.URL.Query()
	q.Add("chat_id", rc.channelID)
	q.Add("text", messageText)
	req.URL.RawQuery = q.Encode()

	resp, err := rc.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status from telegram: %d", resp.StatusCode)
	}

	log.Printf("Message sent: %s\n", messageText)
	return nil
}
