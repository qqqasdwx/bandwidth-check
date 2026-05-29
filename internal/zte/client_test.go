package zte

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOrderedRouterQueryWithoutCacheBust(t *testing.T) {
	got := orderedRouterQuery("loginsceneData", "login_token_json", nil, false)
	want := "_type=loginsceneData&_tag=login_token_json"
	if got != want {
		t.Fatalf("orderedRouterQuery() = %q, want %q", got, want)
	}
}

func TestOrderedRouterQueryWithCacheBust(t *testing.T) {
	got := orderedRouterQuery("vueData", "vue_internet_ethport_data", url.Values{"WANCID": {"IGD.WD1.ETH1"}}, true)
	if !strings.HasPrefix(got, "_type=vueData&_tag=vue_internet_ethport_data&WANCID=IGD.WD1.ETH1&_=") {
		t.Fatalf("orderedRouterQuery() produced unexpected order: %q", got)
	}
}

func TestWANPortStatusReusesSession(t *testing.T) {
	var loginRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.RawQuery == "_type=loginsceneData&_tag=login_token_json":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"_sessionToken": "session-token",
				"logintoken":    "login-token",
			})
		case r.Method == http.MethodPost && r.URL.RawQuery == "_type=loginData&_tag=login_entry":
			loginRequests++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"login_need_refresh": true,
				"sess_token":         "logged-in-token",
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.RawQuery, "_type=vueData&_tag=vue_internet_ethport_data&_="):
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(sampleEthPortXML))
		default:
			t.Fatalf("unexpected request: method=%s query=%q", r.Method, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "admin", "password", time.Second)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	first, err := client.WANPortStatus(t.Context(), "ETH_WAN")
	if err != nil {
		t.Fatalf("first WANPortStatus returned error: %v", err)
	}
	if !first.LoginAttempted || !first.InitialLogin || first.SessionReused {
		t.Fatalf("first check session state = %+v, want login without reuse", first)
	}

	second, err := client.WANPortStatus(t.Context(), "ETH_WAN")
	if err != nil {
		t.Fatalf("second WANPortStatus returned error: %v", err)
	}
	if second.LoginAttempted || second.InitialLogin || !second.SessionReused {
		t.Fatalf("second check session state = %+v, want reuse without login", second)
	}
	if loginRequests != 1 {
		t.Fatalf("loginRequests = %d, want 1", loginRequests)
	}
}

func TestWANPortStatusRetriesAfterSessionTimeout(t *testing.T) {
	var loginRequests int
	var ethPortRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.RawQuery == "_type=loginsceneData&_tag=login_token_json":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"_sessionToken": "session-token",
				"logintoken":    "login-token",
			})
		case r.Method == http.MethodPost && r.URL.RawQuery == "_type=loginData&_tag=login_entry":
			loginRequests++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"login_need_refresh": true,
				"sess_token":         "logged-in-token",
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.RawQuery, "_type=vueData&_tag=vue_internet_ethport_data&_="):
			ethPortRequests++
			w.Header().Set("Content-Type", "text/xml")
			if ethPortRequests == 1 {
				_, _ = w.Write([]byte("<ajax_response_xml_root><IF_ERRORSTR>SessionTimeout</IF_ERRORSTR></ajax_response_xml_root>"))
				return
			}
			_, _ = w.Write([]byte(sampleEthPortXML))
		default:
			t.Fatalf("unexpected request: method=%s query=%q", r.Method, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "admin", "password", time.Second)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	result, err := client.WANPortStatus(t.Context(), "ETH_WAN")
	if err != nil {
		t.Fatalf("WANPortStatus returned error: %v", err)
	}
	if !result.RetriedAfterTimeout {
		t.Fatalf("RetriedAfterTimeout = false, want true")
	}
	if loginRequests != 2 {
		t.Fatalf("loginRequests = %d, want 2", loginRequests)
	}
	if ethPortRequests != 2 {
		t.Fatalf("ethPortRequests = %d, want 2", ethPortRequests)
	}
}
