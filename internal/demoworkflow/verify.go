package demoworkflow

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *client) status(ctx context.Context, actor, path string) (int, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", c.cfg.APIOrigin+path, nil)
	if e != nil {
		return 0, ErrGuard
	}
	req.Header.Set("Authorization", "Bearer "+c.tokens[actor])
	res, e := c.http.Do(req)
	if e != nil {
		return 0, ErrGuard
	}
	defer res.Body.Close()
	return res.StatusCode, nil
}
func (c *client) checkDocument(ctx context.Context, actor, path string) error {
	link, e := c.call(ctx, actor, "GET", path, nil)
	if e != nil {
		return e
	}
	u, e := url.Parse(str(link, "url"))
	if e != nil || u.Scheme+"://"+u.Host != c.cfg.StorageOrigin || u.User != nil {
		return ErrGuard
	}
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return ErrGuard
	}
	res, e := c.http.Do(req)
	if e != nil {
		return fmt.Errorf("signed demo download failed")
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, 4096))
	res.Body.Close()
	if e != nil || res.StatusCode != 200 || !strings.Contains(string(data), "DEMO / TEST ONLY") {
		return fmt.Errorf("demo document content verification failed")
	}
	u.RawQuery = ""
	req, e = http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return ErrGuard
	}
	res, e = c.http.Do(req)
	if e != nil {
		return ErrGuard
	}
	res.Body.Close()
	if res.StatusCode != 401 && res.StatusCode != 403 {
		return fmt.Errorf("demo evidence not confirmed private")
	}
	return nil
}
func (r *Runner) verify(ctx context.Context, initial bool) error {
	results, e := r.store.Results(ctx)
	if e != nil {
		return e
	}
	for _, s := range scenarios {
		request := results[s.key+"-request"]
		requestID := str(request, "id")
		if requestID == "" {
			return ErrGuard
		}
		for _, actor := range []string{"nida.company", "ploy.company", "somchai"} {
			expected := 200
			if (s.customer == "somchai") != (actor == "somchai") {
				expected = 404
			}
			status, e := r.api.status(ctx, actor, "/requests/"+requestID)
			if e != nil {
				return e
			}
			if status != expected {
				return fmt.Errorf("customer isolation mismatch for %s", s.key)
			}
		}
		for _, suffix := range []string{"-drawing", "-po-document", "-deposit-document"} {
			if doc := results[s.key+suffix]; doc != nil {
				if e = r.api.checkDocument(ctx, s.customer, "/attachments/"+str(doc, "id")+"/download"); e != nil {
					return e
				}
			}
		}
		if doc := results[s.key+"-balance-proof"]; doc != nil {
			if e = r.api.checkDocument(ctx, s.customer, "/customer-evidence/"+str(doc, "id")+"/download"); e != nil {
				return e
			}
		}
		if s.stage >= 5 {
			jobID := str(results[s.key+"-job"], "id")
			state, e := r.get(ctx, worker, "/production/jobs/"+jobID)
			if e != nil {
				return e
			}
			if state["finance"] != nil {
				return fmt.Errorf("Worker received internal production costs")
			}
			if initial {
				expected := "completed"
				if s.stage <= 6 {
					expected = "queued"
				}
				if s.stage == 7 {
					expected = "in_progress"
				}
				if str(state, "status") != expected {
					return fmt.Errorf("initial job state mismatch for %s", s.key)
				}
			}
		}
	}
	for _, actor := range []string{worker, "nida.company", "ploy.company", "somchai"} {
		status, e := r.api.status(ctx, actor, "/material-orders")
		if e != nil {
			return e
		}
		if status != 403 {
			return fmt.Errorf("procurement permission mismatch")
		}
	}
	if initial {
		stock, e := r.get(ctx, admin, "/materials?search=DEMO&limit=100")
		if e != nil {
			return e
		}
		found := false
		for _, m := range items(stock) {
			if str(m, "id") == str(results["material-S45C"], "id") {
				found = true
			}
			if str(m, "id") == str(results["material-S45C"], "id") && (str(m, "qty_on_hand") != "105.000000" || str(m, "qty_reserved") != "8.000000" || str(m, "avg_cost") != "100.000000") {
				return fmt.Errorf("initial stock reconciliation mismatch")
			}
		}
		if !found {
			return ErrGuard
		}
		for _, key := range []string{"r6", "r7"} {
			plan, e := r.get(ctx, worker, "/jobs/"+str(results[key+"-job"], "id")+"/materials")
			if e != nil {
				return e
			}
			ready, _ := plan["ready"].(bool)
			if ready != (key == "r7") {
				return fmt.Errorf("queued reservation readiness mismatch")
			}
		}
		for _, key := range []string{"o2", "o3"} {
			path := "/receiving/orders/" + str(results[key+"-order"], "id")
			operational, e := r.get(ctx, worker, path)
			if e != nil {
				return e
			}
			if operational["finance"] != nil {
				return fmt.Errorf("Worker received supplier finance")
			}
			financial, e := r.get(ctx, admin, path)
			if e != nil {
				return e
			}
			if number(obj(financial["finance"]), "credit_days") != 14 {
				return fmt.Errorf("supplier credit term mismatch")
			}
			if key == "o2" && str(financial, "status") != "partially_received" {
				return fmt.Errorf("partial receipt mismatch")
			}
			if key == "o3" && (str(financial, "status") != "received" || str(obj(financial["finance"]), "payment_status") != "unpaid") {
				return fmt.Errorf("full receipt credit fixture mismatch")
			}
		}
		state, e := r.get(ctx, admin, "/fulfillment/jobs/"+str(results["r10-job"], "id"))
		if e != nil {
			return e
		}
		if str(obj(state["invoice"]), "balance") != "3600.00" {
			return fmt.Errorf("pending bank proof changed invoice balance")
		}
		state, e = r.get(ctx, admin, "/fulfillment/jobs/"+str(results["r11-job"], "id"))
		if e != nil {
			return e
		}
		if str(state, "request_status") != "completed" || str(obj(state["invoice"]), "balance") != "0.00" {
			return fmt.Errorf("paid/accepted closure mismatch")
		}
	}
	return nil
}
