package config

import "testing"

func TestBuildUserRegistry_Valid(t *testing.T) {
	js := `[
		{"user_id":1,"api_key":"keyA"},
		{"user_id":2,"api_key":"keyB","role":"user"},
		{"user_id":0,"api_key":"adminKey","role":"ADMIN"}
	]`
	reg, err := buildUserRegistry(js)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reg) != 3 {
		t.Fatalf("got %d users, want 3", len(reg))
	}
	if reg["keyA"].UserID != 1 || reg["keyA"].Role != "user" {
		t.Errorf("keyA = %+v, want user_id 1 role user (default)", reg["keyA"])
	}
	if !reg["adminKey"].IsAdmin() {
		t.Errorf("adminKey should be admin (role is case-insensitive); got %+v", reg["adminKey"])
	}
	if reg["keyB"].IsAdmin() {
		t.Error("keyB must not be admin")
	}
}

func TestBuildUserRegistry_EmptyDisablesPerUserAuth(t *testing.T) {
	reg, err := buildUserRegistry("   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg != nil {
		t.Errorf("empty API_USERS must return nil (per-user auth off); got %v", reg)
	}
}

func TestBuildUserRegistry_Errors(t *testing.T) {
	cases := map[string]string{
		"invalid json":    `{nope`,
		"empty array":     `[]`,
		"missing apikey":  `[{"user_id":1}]`,
		"blank apikey":    `[{"user_id":1,"api_key":"  "}]`,
		"bad role":        `[{"user_id":1,"api_key":"k","role":"superuser"}]`,
		"user role id 0":  `[{"user_id":0,"api_key":"k"}]`,
		"user role no id": `[{"api_key":"k"}]`,
		"duplicate key":   `[{"user_id":1,"api_key":"k"},{"user_id":2,"api_key":"k"}]`,
	}
	for name, js := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := buildUserRegistry(js); err == nil {
				t.Errorf("expected error for %s", name)
			}
		})
	}
}

func TestLoadConfig_APIUsersAndOLTUserID(t *testing.T) {
	t.Setenv("SNMP_HOST", "")
	t.Setenv("SNMP_COMMUNITY", "")
	t.Setenv("OLTS", `[
		{"id":"c320","user_id":1,"host":"10.0.0.1","community":"public","boards":"1,2"},
		{"id":"c300a","user_id":2,"host":"1.2.3.4","port":1161,"community":"public","boards":"3:16"}
	]`)
	t.Setenv("DEFAULT_OLT", "c320")
	t.Setenv("API_USERS", `[{"user_id":1,"api_key":"keyA"},{"user_id":0,"api_key":"admin","role":"admin"}]`)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	// OLT ownership parsed from user_id.
	if cfg.OLTs[0].UserID != 1 || cfg.OLTs[1].UserID != 2 {
		t.Errorf("OLT user_ids = %d,%d; want 1,2", cfg.OLTs[0].UserID, cfg.OLTs[1].UserID)
	}
	// User registry built and keyed by api_key.
	if len(cfg.APIUsers) != 2 {
		t.Fatalf("got %d API users, want 2", len(cfg.APIUsers))
	}
	if cfg.APIUsers["keyA"].UserID != 1 || !cfg.APIUsers["admin"].IsAdmin() {
		t.Errorf("API users parsed wrong: %+v", cfg.APIUsers)
	}
}

func TestLoadConfig_BadAPIUsersFailsFast(t *testing.T) {
	t.Setenv("SNMP_HOST", "10.0.0.1")
	t.Setenv("SNMP_COMMUNITY", "public")
	t.Setenv("OLTS", "")
	t.Setenv("API_USERS", `[{"user_id":1}]`) // missing api_key
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected LoadConfig to fail fast on invalid API_USERS")
	}
}
