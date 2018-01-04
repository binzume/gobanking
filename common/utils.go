package common

import (
	"bytes"
	"html"
	"log"
	"regexp"
	"strings"

	"net/http"
	"net/http/cookiejar"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// UserAgent (must starts with Mozilla/...)
const UserAgent = "Mozilla/5.0 NetBankingtClient/0.1"

func GetMatched(htmlStr, reStr, def string) string {
	re := regexp.MustCompile(reStr)
	if m := re.FindStringSubmatch(htmlStr); m != nil {
		re := regexp.MustCompile(`<[^>]+>`)
		return strings.TrimSpace(html.UnescapeString(re.ReplaceAllString(m[1], "")))
	}
	return def
}

func ToSJIS(s string) string {
	buf := &bytes.Buffer{}
	w := transform.NewWriter(buf, japanese.ShiftJIS.NewEncoder())
	w.Write([]byte(s))
	return buf.String()
}

func NewHttpClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	return &http.Client{Jar: jar, Transport: &AgentSetter{}}, err
}

type AgentSetter struct{}

func (t *AgentSetter) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent)
	log.Println("REQUEST", req.Method, req.URL)
	return http.DefaultTransport.RoundTrip(req)
}

type TempTransactionMap map[string]interface{}

func (tr TempTransactionMap) Amount() int64 {
	return tr["amount"].(int64)
}
func (tr TempTransactionMap) Fee() int {
	return tr["fee"].(int)
}
func (tr TempTransactionMap) FeeMessage() string {
	return tr["fee_msg"].(string)
}
