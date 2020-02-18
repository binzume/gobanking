package utils

import (
	"bytes"
	"html"
	"log"
	"os"
	"regexp"
	"strings"

	"net/http"
	"net/http/cookiejar"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// UserAgent (must starts with Mozilla/...)
const UserAgent = "Mozilla/5.0 NetBankingtClient/0.1"

var Logger *log.Logger = log.New(os.Stderr, "", log.LstdFlags)
var Debug = false

func DebugLog(v ...interface{}) {
	if Debug && Logger != nil {
		Logger.Println(v...)
	}
}

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
	return &http.Client{Jar: jar, Transport: &agentSetter{}}, err
}

type agentSetter struct{}

func (t *agentSetter) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent)
	if Logger != nil {
		Logger.Println("REQUEST", req.Method, req.URL)
	}
	return http.DefaultTransport.RoundTrip(req)
}

type TransferStateMap map[string]interface{}

func (tr TransferStateMap) Amount() int64 {
	return tr["amount"].(int64)
}
func (tr TransferStateMap) Fee() int {
	return tr["fee"].(int)
}
func (tr TransferStateMap) FeeMessage() string {
	return tr["fee_msg"].(string)
}
