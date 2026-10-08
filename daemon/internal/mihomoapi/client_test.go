package mihomoapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGroupsOmitSecretsAndKeepDelay(t *testing.T) {
	secret := "controller-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/proxies":
			_, _ = w.Write([]byte(`{"proxies":{"GLOBAL":{"name":"GLOBAL","type":"Selector","now":"Rayut","all":["DIRECT","Rayut"]},"Rayut":{"name":"Rayut","type":"Selector","now":"节点 A","all":["节点 A","a/b;$(rm)"],"password":"hidden","uuid":"uuid-value","server":"203.0.113.10","public-key":"reality-key","private-key":"reality-private"},"节点 A":{"name":"节点 A","type":"Vless","history":[{"delay":12}],"password":"hidden","uuid":"uuid-value"}}}`))
		case "/providers/proxies":
			_, _ = w.Write([]byte(`{"providers":{"subscription":{"vehicleType":"HTTP","url":"https://example.invalid/sub?token=abc","proxies":[{"name":"a/b;$(rm)","type":"Vless","history":[{"delay":34}],"server":"203.0.113.11","password":"hidden","uuid":"uuid-value"}]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	groups, err := New(srv.URL, secret).Groups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(groups)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, leaked := range []string{"hidden", "uuid-value", "203.0.113", "reality-key", "reality-private", "token=abc", "example.invalid"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("leaked %s in %s", leaked, text)
		}
	}
	if len(groups) != 1 || groups[0].Name != "Rayut" || groups[0].Now != "节点 A" || !groups[0].Selectable {
		t.Fatalf("groups %+v", groups)
	}
	if groups[0].Nodes[0].Delay != 12 || groups[0].Nodes[1].Name != "a/b;$(rm)" || groups[0].Nodes[1].Delay != 34 {
		t.Fatalf("nodes %+v", groups[0].Nodes)
	}
}

func TestGroupsStayInNameOrder(t *testing.T) {
	body := []byte(`{"proxies":{"m组":{"name":"m组","type":"URLTest","now":"n","all":["n"]},"b组":{"name":"b组","type":"Selector","now":"n","all":["n"]},"a组":{"name":"a组","type":"Selector","now":"n","all":["n"]}}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/proxies" {
			_, _ = w.Write(body)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	client := New(srv.URL, "token")
	for n := 0; n < 2; n++ {
		groups, err := client.Groups(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(groups) != 3 || groups[0].Name != "a组" || groups[1].Name != "b组" || groups[2].Name != "m组" {
			t.Fatalf("order %+v", groups)
		}
	}
}

func TestSelectAndDelayEscapeNames(t *testing.T) {
	var uris []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uris = append(uris, r.RequestURI)
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/proxies/组 A":
			var body struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Name != `节点"$/` {
				t.Fatalf("body %q", body.Name)
			}
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/proxies/a/b":
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/providers/proxies/") && r.URL.Path != "/providers/proxies":
			_, _ = w.Write([]byte(`{"delay":42}`))
		case r.URL.Path == "/providers/proxies":
			_, _ = w.Write([]byte(`{"providers":{"sub;1":{"proxies":[{"name":"a/b"}]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	client := New(srv.URL, "token")
	if err := client.Select(context.Background(), "组 A", `节点"$/`); err != nil {
		t.Fatal(err)
	}
	delay, err := client.Delay(context.Background(), "a/b")
	if err != nil {
		t.Fatal(err)
	}
	if delay != 42 {
		t.Fatalf("delay %d", delay)
	}
	joined := strings.Join(uris, "\n")
	if !strings.Contains(joined, "/proxies/%E7%BB%84%20A") {
		t.Fatalf("select uri %s", joined)
	}
	if !strings.Contains(joined, "/proxies/a%2Fb/delay") {
		t.Fatalf("delay uri %s", joined)
	}
	if !strings.Contains(joined, "/providers/proxies/sub%3B1/a%2Fb/healthcheck") {
		t.Fatalf("provider uri %s", joined)
	}
	if strings.Contains(joined, "/proxies/a/b/") {
		t.Fatalf("name became a path: %s", joined)
	}
}

func TestDelayUsesGroupTestURL(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/proxies" && !strings.Contains(r.URL.RawQuery, "timeout=") {
			_, _ = w.Write([]byte(`{"proxies":{"Rayut":{"name":"Rayut","type":"Selector","now":"a","all":["a"],"testUrl":"http://127.0.0.1/local"},"手动":{"name":"手动","type":"Selector","now":"a","all":["a"],"testUrl":"https://cp.cloudflare.com/generate_204"}}}`))
			return
		}
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"delay":80}`))
	}))
	defer srv.Close()
	delay, err := New(srv.URL, "token").Delay(context.Background(), "a")
	if err != nil || delay != 80 {
		t.Fatalf("delay %d err %v", delay, err)
	}
	if !strings.Contains(query, "cp.cloudflare.com") || strings.Contains(query, "127.0.0.1") {
		t.Fatalf("query %s", query)
	}
}

func TestDelayFailureIsExplicit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/delay") {
			w.WriteHeader(http.StatusGatewayTimeout)
			_, _ = w.Write([]byte(`{"message":"timeout","password":"hidden"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "token").Delay(context.Background(), "节点")
	if err != ErrTimeout {
		t.Fatalf("err %v", err)
	}
}

func TestConnectionsKeepFieldsAndDropSecrets(t *testing.T) {
	const body = `{"downloadTotal":10,"uploadTotal":4,"connections":[{"id":"11111111-2222-3333-4444-555555555555","upload":4,"download":10,"chains":["节点","Rayut"],"rule":"Domain","rulePayload":"example.com","metadata":{"host":"example.com","destinationIP":"203.0.113.10","destinationPort":"443","sourceIP":"10.0.0.8","processPath":"/usr/bin/secret","password":"super-secret-password","public-key":"reality-public-key"}}]}`
	got, err := mapConnections([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Destination != "example.com:443" || got[0].Rule != "Domain(example.com)" || got[0].Chain != "节点 → Rayut" || got[0].Upload != 4 || got[0].Download != 10 {
		t.Fatalf("%+v", got)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, secret := range []string{"11111111-2222-3333-4444-555555555555", "super-secret-password", "reality-public-key", "10.0.0.8", "/usr/bin/secret", "203.0.113.10"} {
		if strings.Contains(text, secret) {
			t.Fatalf("leaked %s in %s", secret, text)
		}
	}
}
