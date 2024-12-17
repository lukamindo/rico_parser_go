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

type USDRate struct {
	Buy  float64
	Sell float64
}

type RateChecker struct {
	USDRate   USDRate
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
		USDRate:   USDRate{},
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
	usdRate, err := rc.fetchCurrentRate(ctx)
	if err != nil {
		log.Printf("Error fetching current rate: %v\n", err)
		return
	}

	// If there's no rate or zero, just log it. Zero might indicate a parsing issue.
	if usdRate.Buy == 0 || usdRate.Sell == 0 {
		log.Println("Fetched a rate of 0, which is unexpected; skipping message send.")
		return
	}

	if usdRate.Buy == rc.USDRate.Buy && usdRate.Sell == rc.USDRate.Sell {
		// No change in rate
		return
	}

	rc.USDRate = usdRate
	if err := rc.sendTelegramMessage(ctx, usdRate); err != nil {
		log.Printf("Error sending Telegram message: %v\n", err)
	}
}

// fetchCurrentRate retrieves the current exchange rate from the given URL.
func (rc *RateChecker) fetchCurrentRate(ctx context.Context) (USDRate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return USDRate{}, fmt.Errorf("creating request: %w", err)
	}

	resp, err := rc.client.Do(req)
	if err != nil {
		return USDRate{}, fmt.Errorf("fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return USDRate{}, fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return USDRate{}, fmt.Errorf("parsing HTML: %w", err)
	}

	var ret USDRate
	doc.Find("tbody.first-table-body tr").Each(func(i int, s *goquery.Selection) {
		// only parse USD
		if i != 0 {
			return
		}

		currency := s.Find("td.flag-title").Text()

		// The currency values are likely in the subsequent cells:
		// 0th "currency-value" td might be Buy,
		// 1st "currency-value" td might be Sell (or vice versa).
		buyStr := s.Find("td.currency-value").Eq(0).Text()
		sellStr := s.Find("td.currency-value").Eq(1).Text()

		// Replace the comma with a dot for proper float parsing
		buyStr = strings.ReplaceAll(buyStr, ",", ".")
		sellStr = strings.ReplaceAll(sellStr, ",", ".")

		ret.Buy, err = strconv.ParseFloat(buyStr, 64)
		if err != nil {
			log.Printf("Error converting buyVal: %v", err)
		}

		ret.Sell, err = strconv.ParseFloat(sellStr, 64)
		if err != nil {
			log.Printf("Error converting sellVal: %v", err)
		}

		// Now buyVal and sellVal are floats you can work with.
		fmt.Printf("Currency: %s, ყიდვა: %.4f, გაყიდვა: %.4f\n", currency, ret.Buy, ret.Sell)
	})

	return ret, nil
}

// sendTelegramMessage sends the current exchange rate message to the specified Telegram channel.
func (rc *RateChecker) sendTelegramMessage(ctx context.Context, rate USDRate) error {
	currentDate := time.Now().In(rc.location)
	formattedTime := currentDate.Format(timeFormat)
	messageText := fmt.Sprintf(`%s - 1$ USD 
	ყიდვა: %.4f, გაყიდვა: %.4f`, formattedTime, rate.Buy, rate.Sell)

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
