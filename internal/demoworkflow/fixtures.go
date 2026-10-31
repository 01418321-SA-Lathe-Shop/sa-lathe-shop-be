package demoworkflow

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const admin = "pim.admin"
const worker = "anan.staff"
const demoLabel = "DEMO SET B"

type scenario struct {
	key, customer, title, price string
	stage                       int
}

// Stage 0=request; 1=draft; 2=revised/sent; 3=accepted awaiting evidence;
// 4=pending deposit; 5=queued shortage; 6=queued ready; 7=running;
// 8=completed; 9=invoiced/shipped pending bank review; 10=settled/accepted.
var scenarios = []scenario{
	{"r1", "somchai", "Rubber mold insert - awaiting estimate", "2400.00", 0},
	{"r2", "nida.company", "Compression mold base - draft quotation", "6400.00", 1},
	{"r3", "ploy.company", "Revised mold plate - customer decision", "2800.00", 2},
	{"r4", "nida.company", "Large rubber mold - awaiting PO and deposit", "18000.00", 3},
	{"r5", "somchai", "Custom shaft - deposit awaiting review", "2400.00", 4},
	{"r6", "nida.company", "SUS304 mold insert - material shortage", "8000.00", 5},
	{"r7", "ploy.company", "S45C bushings - fully reserved queue", "6400.00", 6},
	{"r8", "somchai", "Rubber mold pins - production in progress", "4800.00", 7},
	{"r9", "nida.company", "Finished mold plate - ready for invoicing", "9600.00", 8},
	{"r10", "somchai", "Delivered mold - bank payment pending review", "7200.00", 9},
	{"r11", "ploy.company", "Delivered bushings - paid and accepted", "6400.00", 10},
}

func (r *Runner) build(ctx context.Context) error {
	profiles := map[string]string{}
	for _, actor := range []string{"nida.company", "ploy.company", "somchai"} {
		v, e := r.get(ctx, actor, "/customers")
		if e != nil {
			return e
		}
		list := items(v)
		if len(list) != 1 {
			return ErrGuard
		}
		profiles[actor] = str(list[0], "id")
	}
	if profiles["nida.company"] != profiles["ploy.company"] || profiles["somchai"] == profiles["nida.company"] {
		return ErrGuard
	}
	unitIDs := []string{}
	for i, name := range []string{demoLabel + " piece", demoLabel + " kg", demoLabel + " sheet"} {
		u, e := r.post(ctx, fmt.Sprintf("unit-%d", i), admin, "/units", Object{"name": name})
		if e != nil {
			return e
		}
		unitIDs = append(unitIDs, str(u, "id"))
	}
	materialIDs := map[string]string{}
	for i, m := range []struct{ name, stock, cost string }{{"S45C", "100", "100"}, {"SUS304", "0", "250"}, {"AL6061", "60", "150"}, {"BRONZE", "40", "180"}, {"SS400", "50", "80"}, {"NBR", "20", "60"}} {
		v, e := r.post(ctx, "material-"+m.name, admin, "/materials", Object{"name": demoLabel + " " + m.name, "unit_id": unitIDs[i%len(unitIDs)]})
		if e != nil {
			return e
		}
		materialIDs[m.name] = str(v, "id")
		if m.stock != "0" {
			_, e = r.command(ctx, "stock-"+m.name, admin, "POST", "/materials/"+str(v, "id")+"/stock", v, Object{"mode": "addition", "quantity": m.stock, "unit_cost": m.cost, "reason": "TEST ONLY initial demo stock"})
			if e != nil {
				return e
			}
		}
	}
	jobs := map[string]string{}
	for i, s := range scenarios {
		request, e := r.post(ctx, s.key+"-request", s.customer, "/requests", Object{"customer_id": profiles[s.customer], "description": "[" + demoLabel + " " + s.key + "] " + s.title, "qty": int64(2 + i), "material_spec": "TEST ONLY; supplied demonstration drawing; no real manufacturing order", "deadline": r.started.Add(time.Duration(i+3) * 24 * time.Hour).UTC().Format(time.RFC3339Nano)})
		if e != nil {
			return e
		}
		requestID := str(request, "id")
		_, e = r.document(ctx, s.key+"-drawing", s.customer, "/requests/"+requestID+"/uploads", "/uploads/", "Demo specification for "+s.title+"; nominal dimensions 40 x 60 mm, tolerance +/- 0.1 mm. Not a production drawing.")
		if e != nil {
			return e
		}
		if s.stage == 0 {
			continue
		}
		estimator := worker
		if s.key == "r9" {
			estimator = "kittipong.staff"
		}
		quote, e := r.post(ctx, s.key+"-quote", estimator, "/requests/"+requestID+"/quotations", Object{"material_cost": "1000.00", "labor_cost": "500.00", "final_price": s.price})
		if e != nil {
			return e
		}
		if s.stage == 1 {
			continue
		}
		quote, e = r.post(ctx, s.key+"-send", estimator, "/quotations/"+str(quote, "id")+"/send", Object{"expected_lock_version": number(quote, "lock_version")})
		if e != nil {
			return e
		}
		if s.stage == 2 {
			_, e = r.post(ctx, s.key+"-reject", s.customer, "/quotations/"+str(quote, "id")+"/decision", Object{"expected_lock_version": number(quote, "lock_version"), "outcome": "reject", "reason": "TEST ONLY: revise delivery and price"})
			if e != nil {
				return e
			}
			revised, e := r.post(ctx, s.key+"-revision", estimator, "/requests/"+requestID+"/quotations", Object{"material_cost": "950.00", "labor_cost": "500.00", "final_price": "2600.00"})
			if e != nil {
				return e
			}
			_, e = r.post(ctx, s.key+"-revised-send", estimator, "/quotations/"+str(revised, "id")+"/send", Object{"expected_lock_version": number(revised, "lock_version")})
			if e != nil {
				return e
			}
			continue
		}
		decision, e := r.post(ctx, s.key+"-accept", s.customer, "/quotations/"+str(quote, "id")+"/decision", Object{"expected_lock_version": number(quote, "lock_version"), "outcome": "accept"})
		if e != nil {
			return e
		}
		terms := obj(decision["terms"])
		if s.stage == 3 {
			// A real uploaded company PO pending approval, with deposit still missing.
			_, e = r.submit(ctx, s, quote, requestID, "po", terms, false)
			if e != nil {
				return e
			}
			continue
		}
		if required, _ := terms["po_required"].(bool); required {
			if _, e = r.submit(ctx, s, quote, requestID, "po", terms, true); e != nil {
				return e
			}
		}
		if required, _ := terms["deposit_required"].(bool); required {
			if _, e = r.submit(ctx, s, quote, requestID, "deposit", terms, s.stage != 4); e != nil {
				return e
			}
		}
		if s.stage == 4 {
			continue
		}
		job, e := r.step(ctx, s.key+"-job", s.customer, "GET", "/quotations/"+str(quote, "id")+"/fulfillment", Object{}, true, func() (Object, error) {
			v, e := r.get(ctx, s.customer, "/quotations/"+str(quote, "id")+"/fulfillment")
			if e != nil {
				return nil, e
			}
			j := obj(v["job"])
			if str(j, "id") == "" {
				return nil, ErrGuard
			}
			return j, nil
		})
		if e != nil {
			return e
		}
		jobID := str(job, "id")
		jobs[s.key] = jobID
		matID := materialIDs["S45C"]
		quantity := "5"
		if s.stage == 5 {
			matID = materialIDs["SUS304"]
			quantity = "10"
		}
		plan, e := r.command(ctx, s.key+"-plan", worker, "PUT", "/jobs/"+jobID+"/materials", Object{"version": 0}, Object{"lines": []Object{{"material_id": matID, "quantity": quantity}}})
		if e != nil {
			return e
		}
		if s.stage <= 6 {
			continue
		}
		production, e := r.command(ctx, s.key+"-start", worker, "POST", "/production/jobs/"+jobID+"/start", Object{"version": 0}, Object{"expected_plan_version": number(plan, "version")})
		if e != nil {
			return e
		}
		used := map[int]string{7: "2", 8: "3", 9: "4", 10: "1"}[s.stage]
		production, e = r.command(ctx, s.key+"-usage", worker, "POST", "/production/jobs/"+jobID+"/usage", production, Object{"lines": []Object{{"material_id": matID, "quantity": used}}, "note": "TEST ONLY actual material issue"})
		if e != nil {
			return e
		}
		production, e = r.command(ctx, s.key+"-time", worker, "POST", "/production/jobs/"+jobID+"/time", production, Object{"minutes": 30 * (s.stage - 6), "note": "TEST ONLY machine time"})
		if e != nil {
			return e
		}
		if s.stage == 7 {
			continue
		}
		production, e = r.command(ctx, s.key+"-complete", worker, "POST", "/production/jobs/"+jobID+"/complete", production, nil)
		if e != nil {
			return e
		}
		_, e = r.command(ctx, s.key+"-labor", admin, "PUT", "/production/jobs/"+jobID+"/labor-cost", production, Object{"amount": "1200.00"})
		if e != nil {
			return e
		}
		if s.stage == 8 {
			continue
		}
		if e = r.finishCustomer(ctx, s, jobID); e != nil {
			return e
		}
	}
	return r.orders(ctx, materialIDs, jobs)
}
func (r *Runner) submit(ctx context.Context, s scenario, quote Object, requestID, kind string, terms Object, approve bool) (Object, error) {
	doc, e := r.document(ctx, s.key+"-"+kind+"-document", s.customer, "/requests/"+requestID+"/uploads", "/uploads/", "DEMO "+kind+" for quotation "+str(quote, "id")+"; company reference TEST-PO-"+s.key+". No real transaction.")
	if e != nil {
		return nil, e
	}
	body := Object{"idempotency_key": uuid.NewString(), "attachment_id": str(doc, "id"), "kind": kind}
	if kind == "deposit" {
		body["amount"] = terms["deposit_amount"]
		body["paid_at"] = terms["accepted_at"]
	}
	evidence, e := r.step(ctx, s.key+"-"+kind+"-submit", s.customer, "POST", "/quotations/"+str(quote, "id")+"/evidence", body, true, nil)
	if e != nil {
		return nil, e
	}
	if approve {
		return r.post(ctx, s.key+"-"+kind+"-approve", worker, "/evidence/"+str(evidence, "id")+"/review", Object{"outcome": "approved", "expected_version": number(evidence, "version")})
	}
	return evidence, nil
}
func (r *Runner) finishCustomer(ctx context.Context, s scenario, jobID string) error {
	base := "/fulfillment/jobs/" + jobID
	state, e := r.command(ctx, s.key+"-credit", admin, "PUT", base+"/credit-terms", Object{"version": 0}, Object{"credit_days": 14})
	if e != nil {
		return e
	}
	state, e = r.command(ctx, s.key+"-invoice", worker, "POST", base+"/invoice", state, nil)
	if e != nil {
		return e
	}
	method := "self"
	if s.stage == 9 {
		method = "third_party"
	}
	state, e = r.command(ctx, s.key+"-delivery", worker, "POST", base+"/deliveries", state, Object{"method": method, "receiver_name": "DEMO recipient - " + s.customer})
	if e != nil {
		return e
	}
	if s.stage == 9 {
		proof, e := r.document(ctx, s.key+"-balance-proof", s.customer, base+"/evidence/uploads", "/customer-evidence/", "TEST bank payment evidence only; amount "+str(obj(state["invoice"]), "balance")+" THB; not a valid bank receipt.")
		if e != nil {
			return e
		}
		_, e = r.command(ctx, s.key+"-bank", s.customer, "POST", base+"/payments/bank", state, Object{"amount": obj(state["invoice"])["balance"], "paid_at": obj(state["invoice"])["issued_at"], "evidence_id": str(proof, "id")})
		return e
	}
	state, e = r.command(ctx, s.key+"-cash", admin, "POST", base+"/payments/cash", state, Object{"amount": obj(state["invoice"])["balance"], "paid_at": obj(state["invoice"])["issued_at"]})
	if e != nil {
		return e
	}
	_, e = r.command(ctx, s.key+"-inspection", s.customer, "POST", base+"/inspection", state, Object{"outcome": "accept"})
	return e
}
func (r *Runner) orders(ctx context.Context, materials, jobs map[string]string) error {
	for i := 1; i <= 3; i++ {
		key := fmt.Sprintf("o%d", i)
		supplier, e := r.post(ctx, key+"-supplier", admin, "/suppliers", Object{"name": fmt.Sprintf("%s Supplier %d - TEST ONLY", demoLabel, i), "email": fmt.Sprintf("setb-supplier%d@example.com", i), "phone": "02100000" + fmt.Sprint(i)})
		if e != nil {
			return e
		}
		materialID := materials["S45C"]
		qty := "20"
		price := "100.00"
		refs := []string{}
		if i == 1 {
			materialID = materials["SUS304"]
			qty = "100"
			price = "250.00"
			refs = []string{jobs["r6"]}
		}
		if i == 3 {
			materialID = materials["AL6061"]
			qty = "10"
			price = "150.00"
		}
		order, e := r.step(ctx, key+"-order", admin, "POST", "/material-orders", Object{"idempotency_key": uuid.NewString(), "supplier_id": str(supplier, "id"), "job_ids": refs, "expected_at": r.started.Add(time.Duration(i) * 24 * time.Hour).UTC().Format(time.RFC3339Nano), "lines": []Object{{"material_id": materialID, "quantity": qty, "unit_cost": price}}}, true, nil)
		if e != nil {
			return e
		}
		if i == 1 {
			continue
		}
		order, e = r.command(ctx, key+"-accepted", admin, "POST", "/material-orders/"+str(order, "id")+"/actions", order, Object{"action": "accept", "reason": "TEST ONLY supplier confirmation"})
		if e != nil {
			return e
		}
		base := "/receiving/orders/" + str(order, "id")
		state, e := r.command(ctx, key+"-terms", admin, "POST", base+"/credit-terms", order, Object{"credit_days": 14})
		if e != nil {
			return e
		}
		good, damaged := "15", "5"
		if i == 3 {
			good = "10"
			damaged = "0"
		}
		state, e = r.command(ctx, key+"-receipt", worker, "POST", base+"/receipts", state, Object{"note": "TEST ONLY physical receipt", "lines": []Object{{"material_id": materialID, "good": good, "damaged": damaged}}})
		if e != nil {
			return e
		}
		if i == 2 {
			claims, _ := state["claims"].([]any)
			if len(claims) != 1 {
				return ErrGuard
			}
			_, e = r.command(ctx, key+"-claim", worker, "POST", base+"/claims/"+str(obj(claims[0]), "id")+"/submit", state, Object{"note": "TEST ONLY awaiting good replacements"})
			if e != nil {
				return e
			}
		}
	}
	return nil
}
