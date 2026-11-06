package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/file"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/notification"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/platform/objectstore"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/go-chi/chi/v5"
)

func TestUC1Integration(t *testing.T) {
	dsn := os.Getenv("PHASE4_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PHASE4_DATABASE_URL to an isolated migrated Phase 4 database")
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Path != "/sa_lathe_shop_phase4_test" {
		t.Fatal("dedicated Phase 4 test database required")
	}
	password := os.Getenv("TEST_ADMIN_PASSWORD")
	if len(password) < 12 {
		t.Fatal("supply a private TEST_ADMIN_PASSWORD")
	}
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Fatal("a private disposable S3 service is required")
	}
	ctx := context.Background()
	db, err := Open(ctx, dsn, 10)
	if err != nil {
		t.Fatal("test DB unavailable")
	}
	defer db.Close()
	var count int
	if err = db.pool.QueryRow(ctx, `SELECT count(*) FROM account`).Scan(&count); err != nil || count != 0 {
		t.Fatal("test DB must be fresh; no existing data will be overwritten")
	}
	hasher := auth.NewArgonHasher()
	authentication, err := auth.NewService(db, hasher, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	customers := auth.NewCustomerService(db, hasher)
	requests := req.NewService(db)
	store := objectstore.New(objectstore.Config{Endpoint: endpoint, Region: "us-east-1", Bucket: "sa-lathe-shop-private", AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"), PathStyle: true})
	files := file.NewService(db, store, requests)
	_, err = authentication.BootstrapAdmin(ctx, "uc1_admin", "Admin", password)
	if err != nil {
		t.Fatal(err)
	}
	adminLogin, err := authentication.Login(ctx, "uc1_admin", password)
	if err != nil {
		t.Fatal(err)
	}
	_, err = authentication.CreateWorker(ctx, adminLogin.Account, auth.WorkerInput{Username: "uc1_worker", Password: password, Name: "Worker", EmployeeNo: "UC1-W", Email: "worker@example.com", Phone: "+66812345678"})
	if err != nil {
		t.Fatal(err)
	}
	handler := auth.NewHandler(authentication)
	router := chi.NewRouter()
	handler.Routes(router)
	handler.CustomerRoutes(router, customers)
	req.NewHandler(requests).Routes(router, handler.Authenticate)
	files.Routes(router, handler.Authenticate)
	notification.Routes(router, notification.NewInbox(db), handler.Authenticate)
	call := func(method, path, token string, input any, status int) []byte {
		t.Helper()
		var body io.Reader
		if input != nil {
			encoded, _ := json.Marshal(input)
			body = bytes.NewReader(encoded)
		}
		r := httptest.NewRequest(method, path, body)
		r.RemoteAddr = "127.0.0.1:1234"
		if input != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: expected %d, got %d", method, path, status, w.Code)
		}
		return w.Body.Bytes()
	}
	decode := func(data []byte, out any) {
		t.Helper()
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatal(err)
		}
	}
	login := func(username string) auth.LoginResult {
		t.Helper()
		var result auth.LoginResult
		decode(call("POST", "/auth/login", "", map[string]string{"username": username, "password": password}, 200), &result)
		return result
	}
	register := func(username, kind, company, invite string) auth.LoginResult {
		t.Helper()
		address := ""
		if invite == "" {
			address = "1 Test Road, Bangkok"
		}
		call("POST", "/auth/register", "", auth.Registration{Username: username, Name: username, Password: password, CustomerType: kind, CompanyName: company, Invitation: invite, ContactName: username, Address: address}, 201)
		return login(username)
	}
	owner := register("company_owner", "company", "Example Company", "")
	var profiles struct {
		Items []auth.CustomerProfile `json:"items"`
	}
	decode(call("GET", "/customers", owner.AccessToken, nil, 200), &profiles)
	if len(profiles.Items) != 1 || !profiles.Items[0].Owner {
		t.Fatal("company owner not established")
	}
	companyID := profiles.Items[0].ID
	var invitation struct {
		Token string `json:"invitation"`
	}
	decode(call("POST", "/customers/"+companyID+"/invitations", owner.AccessToken, nil, 201), &invitation)
	coworker := register("company_coworker", "company", "", invitation.Token)
	call("POST", "/auth/register", "", auth.Registration{Username: "replay_user", Name: "Replay", Password: password, CustomerType: "company", Invitation: invitation.Token}, 400)
	var orphan int
	db.pool.QueryRow(ctx, `SELECT count(*) FROM person WHERE name='Replay'`).Scan(&orphan)
	if orphan != 0 {
		t.Fatal("failed invitation left an orphan person")
	}
	outsider := register("outside_customer", "individual", "", "")
	call("POST", "/auth/register", "", auth.Registration{Username: "no_address", Name: "No Address", Password: password, CustomerType: "individual"}, 400)
	var addresses struct {
		Items []auth.CustomerAddress `json:"items"`
	}
	decode(call("GET", "/customers/"+companyID+"/addresses", coworker.AccessToken, nil, 200), &addresses)
	if len(addresses.Items) != 1 || addresses.Items[0].Address != "1 Test Road, Bangkok" || addresses.Items[0].ContactName != "company_owner" {
		t.Fatal("sign-up address not recorded")
	}
	call("POST", "/customers/"+companyID+"/addresses", owner.AccessToken, map[string]string{"contact_name": "Warehouse", "address": "2 Warehouse Road"}, 201)
	call("POST", "/customers/"+companyID+"/addresses", coworker.AccessToken, map[string]string{"address": "3 Member Road"}, 403)
	call("GET", "/customers/"+companyID+"/addresses", outsider.AccessToken, nil, 403)
	decode(call("GET", "/customers/"+companyID+"/addresses", owner.AccessToken, nil, 200), &addresses)
	if len(addresses.Items) != 2 || addresses.Items[1].Address != "2 Warehouse Road" || addresses.Items[1].ContactName != "Warehouse" {
		t.Fatal("added address missing")
	}
	call("POST", "/customers/"+companyID+"/invitations", coworker.AccessToken, nil, 403)
	call("POST", "/requests", "", req.Input{CustomerID: companyID, Description: "job", Quantity: 1}, 401)
	call("POST", "/requests", outsider.AccessToken, req.Input{CustomerID: companyID, Description: "job", Quantity: 1}, 403)
	call("POST", "/requests", adminLogin.AccessToken, req.Input{CustomerID: companyID, Description: "job", Quantity: 1}, 403)
	var job req.Request
	decode(call("POST", "/requests", owner.AccessToken, req.Input{CustomerID: companyID, Description: "แม่พิมพ์อัดยาง", Quantity: 2, MaterialSpec: "S45C"}, 201), &job)
	if job.Status != "pending" || job.CreatedBy != owner.Account.ID {
		t.Fatal("wrong request state/ownership")
	}
	call("GET", "/requests/"+job.ID, coworker.AccessToken, nil, 200)
	call("GET", "/requests/"+job.ID, outsider.AccessToken, nil, 404)
	call("GET", "/requests/"+job.ID, adminLogin.AccessToken, nil, 200)
	worker := login("uc1_worker")
	call("GET", "/requests/"+job.ID, worker.AccessToken, nil, 200)
	call("GET", "/requests?limit=101", owner.AccessToken, nil, 400)
	var hidden struct {
		Items []req.Request `json:"items"`
	}
	decode(call("GET", "/requests", outsider.AccessToken, nil, 200), &hidden)
	if len(hidden.Items) != 0 {
		t.Fatal("cross-company list disclosure")
	}
	var outboxCount int
	if err = db.pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE request_id=$1`, job.ID).Scan(&outboxCount); err != nil || outboxCount != 2 {
		t.Fatal("request/outbox not committed atomically")
	}
	content := []byte("Private drawing notes / รายละเอียดงาน")
	var upload file.Link
	decode(call("POST", "/requests/"+job.ID+"/uploads", owner.AccessToken, file.Input{Name: "notes.txt", ContentType: "text/plain", Size: int64(len(content))}, 201), &upload)
	put := func(link file.Link, data []byte) {
		t.Helper()
		r, _ := http.NewRequest("PUT", link.URL, bytes.NewReader(data))
		for k, v := range link.Headers {
			r.Header.Set(k, v)
		}
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal("S3 upload failed")
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatalf("signed upload returned %d", response.StatusCode)
		}
	}
	put(upload, content)
	call("POST", "/uploads/"+upload.AttachmentID+"/complete", coworker.AccessToken, nil, 404)
	call("POST", "/uploads/"+upload.AttachmentID+"/complete", owner.AccessToken, nil, 200)
	call("POST", "/uploads/"+upload.AttachmentID+"/complete", owner.AccessToken, nil, 404)
	call("GET", "/attachments/"+upload.AttachmentID+"/download", outsider.AccessToken, nil, 404)
	var download file.Link
	decode(call("GET", "/attachments/"+upload.AttachmentID+"/download", coworker.AccessToken, nil, 200), &download)
	readDownload := func() []byte {
		t.Helper()
		response, err := http.Get(download.URL)
		if err != nil {
			t.Fatal("signed download failed")
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatalf("download status %d", response.StatusCode)
		}
		body, _ := io.ReadAll(response.Body)
		return body
	}
	if !bytes.Equal(readDownload(), content) {
		t.Fatal("download changed content")
	}
	replacement := bytes.Repeat([]byte("x"), len(content))
	put(upload, replacement)
	if !bytes.Equal(readDownload(), content) {
		t.Fatal("staging overwrite changed finalized evidence")
	}
	unsigned, _ := url.Parse(download.URL)
	unsigned.RawQuery = ""
	response, err := http.Get(unsigned.String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatalf("evidence public: HTTP %d", response.StatusCode)
	}
	call("POST", "/requests/"+job.ID+"/uploads", owner.AccessToken, file.Input{Name: "drawing.svg", ContentType: "image/svg+xml", Size: 100}, 400)
	var bad file.Link
	decode(call("POST", "/requests/"+job.ID+"/uploads", owner.AccessToken, file.Input{Name: "drawing.pdf", ContentType: "application/pdf", Size: 4}, 201), &bad)
	put(bad, []byte("FAKE"))
	call("POST", "/uploads/"+bad.AttachmentID+"/complete", owner.AccessToken, nil, 400)
	dispatcher := notification.NewDispatcher(db, notification.NewLINE(false, "", ""), slog.New(slog.NewTextHandler(io.Discard, nil)))
	for range 2 {
		if err = dispatcher.Once(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var inbox struct {
		Items []notification.Item `json:"items"`
	}
	decode(call("GET", "/notifications", adminLogin.AccessToken, nil, 200), &inbox)
	if len(inbox.Items) != 1 {
		t.Fatal("Admin inbox not delivered")
	}
	call("PATCH", "/notifications/"+inbox.Items[0].ID+"/read", outsider.AccessToken, nil, 404)
	call("PATCH", "/notifications/"+inbox.Items[0].ID+"/read", adminLogin.AccessToken, nil, 204)
	var outsideInbox struct {
		Items []notification.Item `json:"items"`
	}
	decode(call("GET", "/notifications", outsider.AccessToken, nil, 200), &outsideInbox)
	if len(outsideInbox.Items) != 0 {
		t.Fatal("another user's inbox disclosed")
	}
	var attempts int
	if err = db.pool.QueryRow(ctx, `SELECT attempts FROM outbox WHERE request_id=$1 AND channel='line'`, job.ID).Scan(&attempts); err != nil || attempts != 0 {
		t.Fatal("disabled LINE exhausted retries")
	}
	call("POST", "/auth/logout", owner.AccessToken, nil, 204)
	call("GET", "/requests/"+job.ID, owner.AccessToken, nil, 401)
}
