package material

import "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"

// Explicit allowlists prevent newly-added financial fields leaking to Workers.
func MaterialViews(a auth.Account, values []Material) any {
	if a.Role == auth.RoleAdmin {
		return values
	}
	result := make([]map[string]any, 0, len(values))
	for _, v := range values {
		result = append(result, map[string]any{"id": v.ID, "name": v.Name, "unit_id": v.UnitID, "unit_name": v.UnitName, "is_active": v.Active, "version": v.Version, "qty_on_hand": v.OnHand, "qty_reserved": v.Reserved, "qty_available": v.Available})
	}
	return result
}
func MovementViews(a auth.Account, values []Movement) any {
	if a.Role == auth.RoleAdmin {
		return values
	}
	result := make([]map[string]any, 0, len(values))
	for _, v := range values {
		result = append(result, map[string]any{"id": v.ID, "material_id": v.MaterialID, "actor_id": v.ActorID, "mode": v.Mode, "quantity": v.Quantity, "before_quantity": v.BeforeQuantity, "after_quantity": v.AfterQuantity, "version": v.Version, "created_at": v.CreatedAt})
	}
	return result
}
