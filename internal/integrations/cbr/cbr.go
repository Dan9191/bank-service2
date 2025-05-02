package cbr

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Dan9191/bank-service/internal/config"
	"github.com/beevik/etree"
	"github.com/sirupsen/logrus"
)

// CBRClient handles integration with Central Bank of Russia
type CBRClient struct {
	url    string
	client *http.Client
	log    *logrus.Logger
}

// NewCBRClient initializes a new CBR client
func NewCBRClient(cfg *config.Config, log *logrus.Logger) *CBRClient {
	return &CBRClient{
		url: cfg.CBRURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		log: log,
	}
}

// buildSOAPRequest creates a SOAP request for key rate
func (c *CBRClient) buildSOAPRequest() string {
	fromDate := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	toDate := time.Now().Format("2006-01-02")
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
		<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">
			<soap12:Body>
				<KeyRate xmlns="http://web.cbr.ru/">
					<fromDate>%s</fromDate>
					<ToDate>%s</ToDate>
				</KeyRate>
			</soap12:Body>
		</soap12:Envelope>`, fromDate, toDate)
}

// sendRequest sends SOAP request to CBR
func (c *CBRClient) sendRequest(soapRequest string) ([]byte, error) {
	req, err := http.NewRequest("POST", c.url, bytes.NewBuffer([]byte(soapRequest)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	req.Header.Set("SOAPAction", "http://web.cbr.ru/KeyRate")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Log the raw XML response for debugging
	c.log.Debugf("CBR XML response: %s", string(body))

	return body, nil
}

// parseXMLResponse parses the XML response to extract key rate
func (c *CBRClient) parseXMLResponse(rawBody []byte) (float64, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(rawBody); err != nil {
		return 0, fmt.Errorf("failed to parse XML: %v", err)
	}

	// Use XPath from test main.go
	krElements := doc.FindElements("//diffgram/KeyRate/KR")
	if len(krElements) == 0 {
		return 0, fmt.Errorf("no key rate data found in XML")
	}

	// Get the latest key rate (first element)
	latestKR := krElements[0]
	rateElement := latestKR.FindElement("./Rate")
	if rateElement == nil {
		return 0, fmt.Errorf("rate element not found in XML")
	}

	var rate float64
	if _, err := fmt.Sscanf(rateElement.Text(), "%f", &rate); err != nil {
		return 0, fmt.Errorf("failed to parse rate: %v", err)
	}

	return rate, nil
}

// GetKeyRate retrieves the current key rate from CBR and adds bank margin
func (c *CBRClient) GetKeyRate() (float64, error) {
	soapRequest := c.buildSOAPRequest()
	body, err := c.sendRequest(soapRequest)
	if err != nil {
		return 0, err
	}

	rate, err := c.parseXMLResponse(body)
	if err != nil {
		return 0, err
	}

	// Add bank margin (e.g., 5%)
	const bankMargin = 5.0
	rate += bankMargin

	c.log.Infof("Retrieved key rate: %.2f%% (including %.2f%% bank margin)", rate, bankMargin)
	return rate, nil
}
