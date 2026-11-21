package store

import (
	"ams/internal/auth"
	"strings"
	"testing"
	"time"
)

func TestToolTokenUsesOrdinaryHashedDeviceSession(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	admin, err := st.CreateUser(ctx, "Owner", true, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	secret, hash := auth.NewToken()
	if err := st.CreateToolSession(ctx, hash, admin.ID, "Automation"); err != nil {
		t.Fatal(err)
	}
	var stored, fields string
	if err := st.db.QueryRowContext(ctx, `SELECT token_hash, token_hash || COALESCE(user_id,'') || via || user_agent || device FROM sessions WHERE user_id = ?`, admin.ID).Scan(&stored, &fields); err != nil {
		t.Fatal(err)
	}
	if stored != hash || strings.Contains(fields, secret) {
		t.Fatal("raw token persisted")
	}
	se, err := st.GetSession(ctx, hash)
	if err != nil || se == nil || se.Via != "password" || se.UserID != admin.ID || se.Device != "Automation" {
		t.Fatalf("not an ordinary device session: %+v %v", se, err)
	}
	devices, err := st.ListSessions(ctx)
	if err != nil || len(devices) != 1 || !devices[0].IsTool {
		t.Fatalf("tool not listed: %v %v", devices, err)
	}
	if err := st.DeleteSession(ctx, hash); err != nil {
		t.Fatal(err)
	}
	if se, err := st.GetSession(ctx, hash); err != nil || se != nil {
		t.Fatal("device revocation did not revoke tool")
	}
	// No separate authorization, expiry or password-change policy for tool tokens.
	if err := st.CreateToolSession(ctx, hash, admin.ID, "Second"); err != nil {
		t.Fatal(err)
	}
	if err := st.DeletePasswordSessions(ctx, "other-session"); err != nil {
		t.Fatal(err)
	}
	if se, err := st.GetSession(ctx, hash); err != nil || se != nil {
		t.Fatal("password change did not revoke tool")
	}
	if err := st.CreateToolSession(ctx, hash, admin.ID, "Third"); err != nil {
		t.Fatal(err)
	}
	if count, err := st.DeleteStaleSessions(ctx, time.Now().Add(time.Hour).Unix()); err != nil || count != 1 {
		t.Fatalf("tool bypassed stale-device cleanup: %d %v", count, err)
	}
	viewer, err := st.CreateUser(ctx, "Viewer", false, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateToolSession(ctx, "denied", viewer.ID, "Denied"); err == nil {
		t.Fatal("ordinary profile created admin tool token")
	}
}
