package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/go-chi/chi/v5"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Config struct {
	ListenAddr    string
	DBHost        string
	DBPort        int
	DBUser        string
	DBPass        string
	DBName        string
	DBParams      string
	MigrationsDir string
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mywebapp <serve|migrate> [flags]")
		os.Exit(2)
	}

	sub := os.Args[1]
	switch sub {
	case "serve":
		cfg, useSocket, err := parseServeFlags(os.Args[2:])
		if err != nil {
			fatalUsage(err)
		}
		if err := runServe(cfg, useSocket); err != nil {
			log.Fatal(err)
		}
	case "migrate":
		cfg, err := parseMigrateFlags(os.Args[2:])
		if err != nil {
			fatalUsage(err)
		}
		if err := runMigrate(cfg); err != nil {
			log.Fatal(err)
		}
	default:
		fatalUsage(fmt.Errorf("unknown subcommand: %s", sub))
	}
}

func fatalUsage(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(2)
}

func parseServeFlags(args []string) (Config, bool, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var cfg Config
	var dbPort int
	var useSocket bool

	fs.StringVar(&cfg.ListenAddr, "listen", "127.0.0.1:5200", "listen address (ignored if -socket-activation is set)")
	fs.BoolVar(&useSocket, "socket-activation", false, "use systemd socket activation")
	fs.StringVar(&cfg.DBHost, "db-host", "127.0.0.1", "db host")
	fs.IntVar(&dbPort, "db-port", 3306, "db port")
	fs.StringVar(&cfg.DBUser, "db-user", "mywebapp", "db user")
	fs.StringVar(&cfg.DBPass, "db-pass", "", "db password")
	fs.StringVar(&cfg.DBName, "db-name", "mywebapp", "db name")
	fs.StringVar(&cfg.DBParams, "db-params", "parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci", "extra mysql params")

	if err := fs.Parse(args); err != nil {
		return Config{}, false, err
	}
	cfg.DBPort = dbPort
	return cfg, useSocket, nil
}

func parseMigrateFlags(args []string) (Config, error) {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var cfg Config
	var dbPort int

	fs.StringVar(&cfg.MigrationsDir, "migrations", "migrations", "path to migrations directory")
	fs.StringVar(&cfg.DBHost, "db-host", "127.0.0.1", "db host")
	fs.IntVar(&dbPort, "db-port", 3306, "db port")
	fs.StringVar(&cfg.DBUser, "db-user", "mywebapp", "db user")
	fs.StringVar(&cfg.DBPass, "db-pass", "", "db password")
	fs.StringVar(&cfg.DBName, "db-name", "mywebapp", "db name")
	fs.StringVar(&cfg.DBParams, "db-params", "parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci", "extra mysql params")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	cfg.DBPort = dbPort
	return cfg, nil
}

func mysqlDSN(cfg Config) string {
	params := cfg.DBParams
	if params != "" && !strings.HasPrefix(params, "?") {
		params = "?" + params
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s%s", cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName, params)
}

func runMigrate(cfg Config) error {
	dsn := mysqlDSN(cfg)
	sqldb, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer sqldb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sqldb.PingContext(ctx); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}

	goose.SetDialect("mysql")
	if err := goose.Up(sqldb, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

func runServe(cfg Config, useSocket bool) error {
	dsn := mysqlDSN(cfg)
	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}

	r := chi.NewRouter()
	r.Get("/", renderRootHTML)

	r.Route("/health", func(r chi.Router) {
		r.Get("/alive", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})
		r.Get("/ready", func(w http.ResponseWriter, _ *http.Request) {
			sqlDB, err := gdb.DB()
			if err != nil {
				http.Error(w, "db handle not available", http.StatusInternalServerError)
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := sqlDB.PingContext(ctx); err != nil {
				http.Error(w, "db not ready", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})
	})

	r.Route("/items", func(r chi.Router) {
		r.Get("/", listItems(gdb))
		r.Post("/", createItem(gdb))
		r.Get("/{id}", getItem(gdb))
	})

	srv := &http.Server{
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if useSocket {
		listeners, err := activation.Listeners()
		if err != nil {
			return fmt.Errorf("systemd activation listeners: %w", err)
		}
		if len(listeners) == 0 {
			return errors.New("socket activation enabled but no listeners passed by systemd")
		}
		if len(listeners) > 1 {
			return fmt.Errorf("expected 1 systemd listener, got %d", len(listeners))
		}
		log.Printf("serving via systemd socket activation")
		return srv.Serve(listeners[0])
	}

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.ListenAddr, err)
	}
	log.Printf("serving on http://%s", cfg.ListenAddr)
	return srv.Serve(ln)
}

func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json")
}

func renderRootHTML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head><meta charset="utf-8"><title>mywebapp</title></head>
  <body>
    <h1>mywebapp endpoints</h1>
    <ul>
      <li>GET /items</li>
      <li>POST /items</li>
      <li>GET /items/{id}</li>
    </ul>
  </body>
</html>`))
}

type Item struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"type:varchar(255);not null"`
	Quantity  int       `json:"quantity" gorm:"not null"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
}

type createItemRequest struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

func listItems(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type row struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		}
		var items []row
		if err := db.Model(&Item{}).Select("id", "name").Order("id desc").Find(&items).Error; err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if wantsHTML(r) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			var b strings.Builder
			b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>items</title></head><body>")
			b.WriteString("<h1>Items</h1><table border=\"1\"><tr><th>id</th><th>name</th></tr>")
			for _, it := range items {
				b.WriteString("<tr><td>")
				b.WriteString(strconv.FormatUint(uint64(it.ID), 10))
				b.WriteString("</td><td>")
				b.WriteString(htmlEscape(it.Name))
				b.WriteString("</td></tr>")
			}
			b.WriteString("</table></body></html>")
			_, _ = w.Write([]byte(b.String()))
			return
		}
		writeJSON(w, items)
	}
}

func createItem(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		var req createItemRequest
		if strings.Contains(ct, "application/json") {
			if err := decodeJSON(r, &req); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
		} else {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad form", http.StatusBadRequest)
				return
			}
			req.Name = r.FormValue("name")
			q := r.FormValue("quantity")
			if q != "" {
				v, err := strconv.Atoi(q)
				if err != nil {
					http.Error(w, "quantity must be int", http.StatusBadRequest)
					return
				}
				req.Quantity = v
			}
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if req.Quantity < 0 {
			http.Error(w, "quantity must be >= 0", http.StatusBadRequest)
			return
		}

		item := Item{Name: req.Name, Quantity: req.Quantity, CreatedAt: time.Now().UTC()}
		if err := db.Create(&item).Error; err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		if wantsHTML(r) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("<!doctype html><html><body><p>created item id: " + strconv.FormatUint(uint64(item.ID), 10) + "</p></body></html>"))
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, item)
	}
}

func getItem(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id64, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil || id64 == 0 {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var item Item
		if err := db.First(&item, uint(id64)).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		if wantsHTML(r) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			created := item.CreatedAt.UTC().Format(time.RFC3339)
			html := "<!doctype html><html><head><meta charset=\"utf-8\"><title>item</title></head><body>" +
				"<h1>Item</h1>" +
				"<ul>" +
				"<li>id: " + strconv.FormatUint(uint64(item.ID), 10) + "</li>" +
				"<li>name: " + htmlEscape(item.Name) + "</li>" +
				"<li>quantity: " + strconv.Itoa(item.Quantity) + "</li>" +
				"<li>created_at: " + created + "</li>" +
				"</ul></body></html>"
			_, _ = w.Write([]byte(html))
			return
		}
		writeJSON(w, item)
	}
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

func decodeJSON(r *http.Request, out any) error {
	dec := jsonNewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := jsonNewEncoder(w)
	_ = enc.Encode(v)
}

type jsonDecoder interface {
	Decode(v any) error
	DisallowUnknownFields()
}

type jsonEncoder interface {
	Encode(v any) error
}

func jsonNewDecoder(r io.Reader) jsonDecoder {
	return json.NewDecoder(r)
}

func jsonNewEncoder(w io.Writer) jsonEncoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}
