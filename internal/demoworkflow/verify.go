package demoworkflow

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	if e != nil || res.StatusCode != 200 || len(data) < 5 || string(data[:5]) != "%PDF-" {
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
				if s.stage == 12 {
					expected = "in_progress"
				}
				if str(state, "status") != expected {
					return fmt.Errorf("initial job state mismatch for %s", s.key)
				}
			}
		}
	}
	if initial {
		cancelled, e := r.get(ctx, "somchai", "/requests/"+str(results["cancelled-request"], "id"))
		if e != nil || str(cancelled, "status") != "canceled" {
			return fmt.Errorf("cancelled request fixture mismatch")
		}
	}
	// UC6 owner change 2026-11-06: Workers place and manage orders; customers stay out.
	for actor, want := range map[string]int{worker: 200, "nida.company": 403, "ploy.company": 403, "somchai": 403} {
		status, e := r.api.status(ctx, actor, "/material-orders")
		if e != nil {
			return e
		}
		if status != want {
			return fmt.Errorf("procurement permission mismatch")
		}
	}
	if initial {
		stock, e := r.get(ctx, admin, "/materials?search=S45C&limit=100")
		if e != nil {
			return e
		}
		found := false
		for _, m := range items(stock) {
			if str(m, "id") == str(results["material-S45C"], "id") {
				found = true
			}
			if str(m, "id") == str(results["material-S45C"], "id") && (str(m, "qty_on_hand") != "112.000000" || str(m, "qty_reserved") != "5.000000" || str(m, "avg_cost") != "48.000000") {
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
			// o2 receives 15 good + 5 damaged of 20: everything arrived, so the order is received while the claim stays open (docs/02-frontend.md UC7).
			if key == "o2" && str(financial, "status") != "received" {
				return fmt.Errorf("partial receipt mismatch")
			}
			if key == "o3" && (str(financial, "status") != "received" || str(obj(financial["finance"]), "payment_status") != "pending") {
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
		state, e = r.get(ctx, admin, "/fulfillment/jobs/"+str(results["r12-job"], "id"))
		if e != nil {
			return e
		}
		payments, _ := state["payments"].([]any)
		if len(payments) == 0 || str(obj(payments[len(payments)-1]), "status") != "rejected" {
			return fmt.Errorf("rejected bank payment fixture mismatch")
		}
		state, e = r.get(ctx, admin, "/fulfillment/jobs/"+str(results["r13-job"], "id"))
		if e != nil {
			return e
		}
		deliveries, _ := state["deliveries"].([]any)
		if len(deliveries) == 0 || str(obj(deliveries[len(deliveries)-1]), "status") != "returned" || str(state, "job_status") != "in_progress" {
			return fmt.Errorf("returned delivery fixture mismatch")
		}
		cancelledOrder, e := r.get(ctx, admin, "/material-orders/"+str(results["o4-order"], "id"))
		if e != nil || str(cancelledOrder, "status") != "cancelled" || str(cancelledOrder, "supplier_response") != "rejected" {
			return fmt.Errorf("cancelled supplier order fixture mismatch")
		}
		for key, expected := range map[string]string{"o5": "partially_received", "o6": "damaged"} {
			orderState, e := r.get(ctx, admin, "/receiving/orders/"+str(results[key+"-order"], "id"))
			if e != nil || str(orderState, "status") != expected {
				return fmt.Errorf("%s supplier order fixture mismatch", expected)
			}
		}
	}
	return nil
}
