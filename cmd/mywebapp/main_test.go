package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	glebsqlite "github.com/glebarez/sqlite"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(glebsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&Item{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func chiCtxWithParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestWantsHTML(t *testing.T) {
	cases := []struct {
		accept string
		want   bool
	}{
		{"text/html", true},
		{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", true},
		{"application/json", false},
		{"text/html,application/json", false},
		{"", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", tc.accept)
		got := wantsHTML(req)
		if got != tc.want {
			t.Errorf("wantsHTML(%q) = %v, want %v", tc.accept, got, tc.want)
		}
	}
}

func TestHtmlEscape(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{"<script>", "&lt;script&gt;"},
		{"a & b", "a &amp; b"},
		{`say "hi"`, "say &quot;hi&quot;"},
		{"it's", "it&#39;s"},
	}
	for _, tc := range cases {
		if got := htmlEscape(tc.in); got != tc.want {
			t.Errorf("htmlEscape(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMysqlDSN(t *testing.T) {
	cfg := Config{
		DBUser:   "user",
		DBPass:   "pass",
		DBHost:   "localhost",
		DBPort:   3306,
		DBName:   "mydb",
		DBParams: "parseTime=true",
	}
	got := mysqlDSN(cfg)
	want := "user:pass@tcp(localhost:3306)/mydb?parseTime=true"
	if got != want {
		t.Errorf("mysqlDSN = %q, want %q", got, want)
	}

	cfg.DBParams = "?parseTime=true"
	got = mysqlDSN(cfg)
	if !strings.HasSuffix(got, "?parseTime=true") {
		t.Errorf("unexpected DSN with leading ?: %q", got)
	}

	cfg.DBParams = ""
	got = mysqlDSN(cfg)
	if strings.Contains(got, "?") {
		t.Errorf("expected no '?' in DSN with empty params, got %q", got)
	}
}

func TestParseServeFlags(t *testing.T) {
	cfg, useSocket, err := parseServeFlags([]string{
		"-listen", "0.0.0.0:8080",
		"-db-host", "dbhost",
		"-db-port", "3307",
		"-db-user", "u",
		"-db-pass", "p",
		"-db-name", "n",
	})
	if err != nil {
		t.Fatalf("parseServeFlags: %v", err)
	}
	if cfg.ListenAddr != "0.0.0.0:8080" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.DBPort != 3307 {
		t.Errorf("DBPort = %d", cfg.DBPort)
	}
	if useSocket {
		t.Error("expected useSocket=false")
	}

	cfg2, _, err := parseServeFlags(nil)
	if err != nil {
		t.Fatalf("parseServeFlags defaults: %v", err)
	}
	if cfg2.ListenAddr != "127.0.0.1:5200" {
		t.Errorf("default ListenAddr = %q", cfg2.ListenAddr)
	}
}

func TestParseServeFlagsError(t *testing.T) {
	_, _, err := parseServeFlags([]string{"-unknown-flag"})
	if err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestParseMigrateFlags(t *testing.T) {
	cfg, err := parseMigrateFlags([]string{
		"-db-host", "h",
		"-db-port", "3308",
		"-db-user", "u",
		"-db-pass", "p",
		"-db-name", "db",
		"-migrations", "/tmp/mig",
	})
	if err != nil {
		t.Fatalf("parseMigrateFlags: %v", err)
	}
	if cfg.MigrationsDir != "/tmp/mig" {
		t.Errorf("MigrationsDir = %q", cfg.MigrationsDir)
	}
	if cfg.DBPort != 3308 {
		t.Errorf("DBPort = %d", cfg.DBPort)
	}
}

func TestParseMigrateFlagsError(t *testing.T) {
	_, err := parseMigrateFlags([]string{"-nope"})
	if err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestRenderRootHTML(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	renderRootHTML(w, req)
	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q", ct)
	}
	if !strings.Contains(w.Body.String(), "mywebapp") {
		t.Error("body does not contain 'mywebapp'")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, map[string]string{"key": "value"})
	res := w.Result()
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	var out map[string]string
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if out["key"] != "value" {
		t.Errorf("unexpected JSON: %v", out)
	}
}

func TestDecodeJSON(t *testing.T) {
	body := bytes.NewBufferString(`{"name":"test","quantity":5}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	var out createItemRequest
	if err := decodeJSON(req, &out); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}
	if out.Name != "test" || out.Quantity != 5 {
		t.Errorf("unexpected: %+v", out)
	}
}

func TestDecodeJSONUnknownField(t *testing.T) {
	body := bytes.NewBufferString(`{"name":"x","bogus":1}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	var out createItemRequest
	if err := decodeJSON(req, &out); err == nil {
		t.Error("expected error for unknown field")
	}
}

func TestListItemsEmpty(t *testing.T) {
	db := newTestDB(t)
	h := listItems(db)

	req := httptest.NewRequest(http.MethodGet, "/items/", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var items []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty list, got %d items", len(items))
	}
}

func TestListItemsWithData(t *testing.T) {
	db := newTestDB(t)
	db.Create(&Item{Name: "apple", Quantity: 3})
	h := listItems(db)

	req := httptest.NewRequest(http.MethodGet, "/items/", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var items []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestListItemsHTML(t *testing.T) {
	db := newTestDB(t)
	db.Create(&Item{Name: "banana", Quantity: 1})
	h := listItems(db)

	req := httptest.NewRequest(http.MethodGet, "/items/", nil)
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "banana") {
		t.Error("HTML body does not contain 'banana'")
	}
}

func TestCreateItemJSON(t *testing.T) {
	db := newTestDB(t)
	h := createItem(db)

	body := bytes.NewBufferString(`{"name":"widget","quantity":10}`)
	req := httptest.NewRequest(http.MethodPost, "/items/", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateItemForm(t *testing.T) {
	db := newTestDB(t)
	h := createItem(db)

	body := strings.NewReader("name=gadget&quantity=2")
	req := httptest.NewRequest(http.MethodPost, "/items/", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}
}

func TestCreateItemHTMLResponse(t *testing.T) {
	db := newTestDB(t)
	h := createItem(db)

	body := bytes.NewBufferString(`{"name":"thing","quantity":0}`)
	req := httptest.NewRequest(http.MethodPost, "/items/", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "created item") {
		t.Error("HTML response missing 'created item'")
	}
}

func TestCreateItemEmptyName(t *testing.T) {
	db := newTestDB(t)
	h := createItem(db)

	body := bytes.NewBufferString(`{"name":"","quantity":1}`)
	req := httptest.NewRequest(http.MethodPost, "/items/", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateItemNegativeQuantity(t *testing.T) {
	db := newTestDB(t)
	h := createItem(db)

	body := bytes.NewBufferString(`{"name":"x","quantity":-1}`)
	req := httptest.NewRequest(http.MethodPost, "/items/", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateItemBadJSON(t *testing.T) {
	db := newTestDB(t)
	h := createItem(db)

	body := bytes.NewBufferString(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/items/", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateItemFormBadQuantity(t *testing.T) {
	db := newTestDB(t)
	h := createItem(db)

	body := strings.NewReader("name=x&quantity=abc")
	req := httptest.NewRequest(http.MethodPost, "/items/", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetItemFound(t *testing.T) {
	db := newTestDB(t)
	item := Item{Name: "bolt", Quantity: 100}
	db.Create(&item)

	h := getItem(db)
	req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
	req = chiCtxWithParam(req, "id", "1")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var out Item
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Name != "bolt" {
		t.Errorf("name = %q", out.Name)
	}
}

func TestGetItemHTMLResponse(t *testing.T) {
	db := newTestDB(t)
	db.Create(&Item{Name: "nut", Quantity: 50})

	h := getItem(db)
	req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
	req.Header.Set("Accept", "text/html")
	req = chiCtxWithParam(req, "id", "1")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "nut") {
		t.Error("HTML body does not contain 'nut'")
	}
}

func TestGetItemNotFound(t *testing.T) {
	db := newTestDB(t)
	h := getItem(db)

	req := httptest.NewRequest(http.MethodGet, "/items/999", nil)
	req = chiCtxWithParam(req, "id", "999")
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetItemInvalidID(t *testing.T) {
	db := newTestDB(t)
	h := getItem(db)

	for _, id := range []string{"abc", "0", "-1"} {
		req := httptest.NewRequest(http.MethodGet, "/items/"+id, nil)
		req = chiCtxWithParam(req, "id", id)
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("id=%q: expected 400, got %d", id, w.Code)
		}
	}
}
