package zte

import (
	"net/url"
	"strings"
	"testing"
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
