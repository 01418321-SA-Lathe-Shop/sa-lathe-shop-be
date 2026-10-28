package demoseed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const DatasetName = "frontend-accounts"
const FixtureVersion = 1
const CompanyName = "Siam Precision Parts Co., Ltd."

type AccountSpec struct {
	Username    string `json:"username"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	PasswordEnv string `json:"password_env"`
	EmployeeNo  string `json:"employee_no,omitempty"`
	Position    string `json:"position,omitempty"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
}

// Fictional frontend identities only. Credentials are never part of fixtures.
func Accounts() []AccountSpec {
	return []AccountSpec{
		{Username: "pim.admin", Name: "Pim Rattana", Role: "admin", PasswordEnv: "SEED_PIM_PASSWORD"},
		{Username: "anan.staff", Name: "Anan Srisuk", Role: "worker", PasswordEnv: "SEED_ANAN_PASSWORD", EmployeeNo: "ST-001", Position: "Lathe operator", Email: "anan@example.com", Phone: "0812345678"},
		{Username: "kittipong.staff", Name: "Kittipong Meesuk", Role: "worker", PasswordEnv: "SEED_KITTIPONG_PASSWORD", EmployeeNo: "ST-002", Position: "CNC programmer", Email: "kittipong@example.com", Phone: "0898765432"},
		{Username: "nida.company", Name: "Nida Wongsuwan", Role: "customer", PasswordEnv: "SEED_NIDA_PASSWORD"},
		{Username: "ploy.company", Name: "Ploy Charoen", Role: "customer", PasswordEnv: "SEED_PLOY_PASSWORD"},
		{Username: "somchai", Name: "Somchai Jaidee", Role: "customer", PasswordEnv: "SEED_SOMCHAI_PASSWORD"},
	}
}

func ManifestDigest() string {
	data, _ := json.Marshal(struct {
		Accounts []AccountSpec
		Company  string
		Version  int
	}{Accounts(), CompanyName, FixtureVersion})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
