package cas

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxValidationResponse = 1 << 20

var ErrTicketInvalid = errors.New("CAS ticket is invalid")

type Identity struct {
	Subject     string
	UserName    string
	Email       string
	DisplayName string
	Avatar      string
}

type Client struct {
	baseURL        *url.URL
	validationURL  *url.URL
	validationHost string
	httpClient     *http.Client
}

func NewClient(baseURL, validationURL, validationHost string, timeout time.Duration) (*Client, error) {
	base, err := parseAbsoluteURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse CAS base URL: %w", err)
	}
	var validation *url.URL
	if strings.TrimSpace(validationURL) == "" {
		validation = endpoint(base, "serviceValidate")
	} else {
		validationBase, parseErr := parseAbsoluteURL(validationURL)
		err = parseErr
		if err != nil {
			return nil, fmt.Errorf("parse CAS validation URL: %w", err)
		}
		validation = endpoint(validationBase, "serviceValidate")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL:        base,
		validationURL:  validation,
		validationHost: strings.TrimSpace(validationHost),
		httpClient: &http.Client{
			Timeout:       timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func (c *Client) LoginURL(service string) string {
	value := endpoint(c.baseURL, "login")
	query := value.Query()
	query.Set("service", service)
	value.RawQuery = query.Encode()
	return value.String()
}

func (c *Client) LogoutURL(service string) string {
	value := endpoint(c.baseURL, "logout")
	query := value.Query()
	if strings.TrimSpace(service) != "" {
		query.Set("service", service)
	}
	value.RawQuery = query.Encode()
	return value.String()
}

func (c *Client) Validate(ctx context.Context, ticket, service string) (Identity, error) {
	if strings.TrimSpace(ticket) == "" {
		return Identity{}, ErrTicketInvalid
	}
	requestURL := *c.validationURL
	query := requestURL.Query()
	query.Set("ticket", ticket)
	query.Set("service", service)
	requestURL.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return Identity{}, err
	}
	if c.validationHost != "" {
		request.Host = c.validationHost
	}
	request.Header.Set("Accept", "application/xml, text/xml")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Identity{}, fmt.Errorf("request CAS validation: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("CAS validation returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxValidationResponse+1))
	if err != nil {
		return Identity{}, fmt.Errorf("read CAS validation response: %w", err)
	}
	if len(body) > maxValidationResponse {
		return Identity{}, errors.New("CAS validation response is too large")
	}
	upper := bytes.ToUpper(body)
	if bytes.Contains(upper, []byte("<!DOCTYPE")) || bytes.Contains(upper, []byte("<!ENTITY")) {
		return Identity{}, errors.New("CAS validation response contains a forbidden declaration")
	}

	var document serviceResponse
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = true
	if err := decoder.Decode(&document); err != nil {
		return Identity{}, fmt.Errorf("decode CAS validation response: %w", err)
	}
	if document.Success == nil {
		return Identity{}, ErrTicketInvalid
	}
	success := document.Success
	attributes := mergeAttributes(success.Attributes, success.UserAttributes)
	subject := strings.TrimSpace(success.User)
	userName := firstNonEmpty(attributes["username"], subject)
	if subject == "" || userName == "" {
		return Identity{}, ErrTicketInvalid
	}
	return Identity{
		Subject:     subject,
		UserName:    userName,
		Email:       firstNonEmpty(success.Email, attributes["email"]),
		DisplayName: firstNonEmpty(success.DisplayName, attributes["displayname"], attributes["display"], attributes["realname"], attributes["name"], userName),
		Avatar:      firstNonEmpty(success.Avatar, success.AvatarURL, attributes["avatar"], attributes["avatarurl"]),
	}, nil
}

type serviceResponse struct {
	Success *authenticationSuccess `xml:"authenticationSuccess"`
	Failure *authenticationFailure `xml:"authenticationFailure"`
}

type authenticationFailure struct {
	Code    string `xml:"code,attr"`
	Message string `xml:",chardata"`
}

type authenticationSuccess struct {
	User           string        `xml:"user"`
	Email          string        `xml:"email"`
	DisplayName    string        `xml:"displayName"`
	Avatar         string        `xml:"avatar"`
	AvatarURL      string        `xml:"avatarUrl"`
	Attributes     casAttributes `xml:"attributes"`
	UserAttributes casAttributes `xml:"userAttributes"`
}

type casAttributes struct {
	UserName    string             `xml:"username"`
	Name        string             `xml:"name"`
	Email       string             `xml:"email"`
	DisplayName string             `xml:"displayName"`
	Avatar      string             `xml:"avatar"`
	AvatarURL   string             `xml:"avatarUrl"`
	Items       []genericAttribute `xml:"attribute"`
}

type genericAttribute struct {
	Name      string `xml:"name,attr"`
	Value     string `xml:"value,attr"`
	Character string `xml:",chardata"`
}

func mergeAttributes(groups ...casAttributes) map[string]string {
	result := make(map[string]string)
	for _, group := range groups {
		putAttribute(result, "username", group.UserName)
		putAttribute(result, "name", group.Name)
		putAttribute(result, "email", group.Email)
		putAttribute(result, "displayname", group.DisplayName)
		putAttribute(result, "avatar", group.Avatar)
		putAttribute(result, "avatarurl", group.AvatarURL)
		for _, item := range group.Items {
			putAttribute(result, normalizeAttributeName(item.Name), firstNonEmpty(item.Value, item.Character))
		}
	}
	return result
}

func putAttribute(values map[string]string, name, value string) {
	name = normalizeAttributeName(name)
	value = strings.TrimSpace(value)
	if name != "" && value != "" && values[name] == "" {
		values[name] = value
	}
}

func normalizeAttributeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "")
	return strings.ReplaceAll(value, "-", "")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func endpoint(base *url.URL, name string) *url.URL {
	value := *base
	value.Path = strings.TrimRight(value.Path, "/") + "/" + name
	value.RawPath = ""
	return &value
}

func parseAbsoluteURL(raw string) (*url.URL, error) {
	value, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || value.Scheme == "" || value.Host == "" || value.User != nil {
		return nil, errors.New("URL must be absolute and must not contain user info")
	}
	if value.Scheme != "https" && value.Hostname() != "localhost" && value.Hostname() != "127.0.0.1" {
		return nil, errors.New("URL must use HTTPS")
	}
	return value, nil
}
