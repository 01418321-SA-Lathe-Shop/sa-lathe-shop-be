package demoseed

import "testing"

func TestGuard(t *testing.T) {
	base := map[string]string{"SEED_ENABLED": "true", "SEED_ENV": "test", "SEED_CONFIRM_DATABASE": "sa_lathe_shop_seed_test", "SEED_DATABASE_URL": "postgres://user:placeholder@127.0.0.1:55432/sa_lathe_shop_seed_test?sslmode=disable"}
	if _, e := LoadConfig(func(k string) string { return base[k] }); e != nil {
		t.Fatal(e)
	}
	for _, change := range []map[string]string{
		{"SEED_ENABLED": ""}, {"SEED_ENV": "production"}, {"SEED_CONFIRM_DATABASE": "other"}, {"SEED_DATABASE_URL": ""},
		{"SEED_DATABASE_URL": "postgres://user@remote.example/sa_lathe_shop_seed_test"},
		{"SEED_DATABASE_URL": "postgres://user@localhost/sa_lathe_shop"},
		{"SEED_DATABASE_URL": "postgres://user@localhost/sa_lathe_shop_seed_test?host=remote"},
		{"SEED_DATABASE_URL": "postgres://user@localhost/sa_lathe_shop_seed_test?options=-csearch_path%3Dother"},
		{"SEED_DATABASE_URL": "postgres://user@localhost/sa_lathe_shop_seed_test?sslmode=disable&sslmode=require"},
	} {
		get := func(k string) string {
			if v, ok := change[k]; ok {
				return v
			}
			return base[k]
		}
		if _, e := LoadConfig(get); e == nil {
			t.Fatal("unsafe target accepted")
		}
	}
}

func TestManifestHasNoLegacyOrCredentials(t *testing.T) {
	if len(Accounts()) != 6 || len(ManifestDigest()) != 64 {
		t.Fatal("invalid manifest")
	}
	seen := map[string]bool{}
	for _, a := range Accounts() {
		if a.Username == "legacy.staff" || a.PasswordEnv == "" || seen[a.Username] {
			t.Fatal("invalid account manifest")
		}
		seen[a.Username] = true
	}
}

func TestVerificationOrigin(t *testing.T) {
	for _, base := range []string{"https://remote.example", "http://user:private@localhost:8080", "http://localhost:8080/path", "http://localhost:8080?redirect=other", "file:///tmp/api", "http://127.0.0.1:8080#fragment"} {
		if verifyOrigin(base) {
			t.Fatal("unsafe verification origin accepted")
		}
	}
	if !verifyOrigin("http://127.0.0.1:8080") {
		t.Fatal("loopback verification rejected")
	}
}
