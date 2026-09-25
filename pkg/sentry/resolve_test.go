package sentry

import "testing"

func TestResolve(t *testing.T) {
	const svc = "snmp-olt-zte"
	const fullSHA = "0123456789abcdef0123456789abcdef01234567"

	tests := []struct {
		name        string
		env         map[string]string
		fallbackEnv string
		version     string
		wantEnv     string
		wantRelease string
	}{
		{
			name:        "tag build without overrides",
			fallbackEnv: "production",
			version:     "v3.3.4",
			wantEnv:     "production",
			wantRelease: "snmp-olt-zte@v3.3.4",
		},
		{
			name:        "main build with short sha",
			fallbackEnv: "production",
			version:     "abc1234",
			wantEnv:     "production",
			wantRelease: "snmp-olt-zte@abc1234",
		},
		{
			name:        "full sha is shortened to 7 chars",
			fallbackEnv: "production",
			version:     fullSHA,
			wantEnv:     "production",
			wantRelease: "snmp-olt-zte@0123456",
		},
		{
			name:        "uppercase 40-char value is not treated as a sha",
			fallbackEnv: "production",
			version:     "0123456789ABCDEF0123456789ABCDEF01234567",
			wantEnv:     "production",
			wantRelease: "snmp-olt-zte@0123456789ABCDEF0123456789ABCDEF01234567",
		},
		{
			name:        "local build defaults to dev",
			fallbackEnv: "development",
			version:     "dev",
			wantEnv:     "development",
			wantRelease: "snmp-olt-zte@dev",
		},
		{
			name:        "empty version becomes dev",
			fallbackEnv: "production",
			version:     "  ",
			wantEnv:     "production",
			wantRelease: "snmp-olt-zte@dev",
		},
		{
			name:        "SENTRY_ENVIRONMENT overrides fallback",
			env:         map[string]string{"SENTRY_ENVIRONMENT": "prod-jkt"},
			fallbackEnv: "production",
			version:     "v3.3.4",
			wantEnv:     "prod-jkt",
			wantRelease: "snmp-olt-zte@v3.3.4",
		},
		{
			name:        "SENTRY_RELEASE overrides built-in release",
			env:         map[string]string{"SENTRY_RELEASE": "snmp-olt-zte@custom"},
			fallbackEnv: "production",
			version:     "v3.3.4",
			wantEnv:     "production",
			wantRelease: "snmp-olt-zte@custom",
		},
		{
			name: "both overrides set",
			env: map[string]string{
				"SENTRY_ENVIRONMENT": "prod-jkt",
				"SENTRY_RELEASE":     "snmp-olt-zte@custom",
			},
			fallbackEnv: "production",
			version:     "abc1234",
			wantEnv:     "prod-jkt",
			wantRelease: "snmp-olt-zte@custom",
		},
		{
			name: "whitespace-only overrides are ignored",
			env: map[string]string{
				"SENTRY_ENVIRONMENT": "   ",
				"SENTRY_RELEASE":     "\t",
			},
			fallbackEnv: "staging",
			version:     "abc1234",
			wantEnv:     "staging",
			wantRelease: "snmp-olt-zte@abc1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(k string) string { return tt.env[k] }
			gotEnv, gotRelease := Resolve(getenv, svc, tt.fallbackEnv, tt.version)
			if gotEnv != tt.wantEnv {
				t.Errorf("environment = %q, want %q", gotEnv, tt.wantEnv)
			}
			if gotRelease != tt.wantRelease {
				t.Errorf("release = %q, want %q", gotRelease, tt.wantRelease)
			}
		})
	}
}
