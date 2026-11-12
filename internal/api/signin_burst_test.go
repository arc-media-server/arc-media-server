package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Drive the real handlers with simultaneous wrong guesses, not just Limiter's unit API.
func TestSignInConcurrentBurstIsBounded(t *testing.T) {
	for _, name := range []string{"password", "pin", "admin-profile", "change-password", "friend", "invite", "link"} {
		t.Run(name, func(t *testing.T) {
			s, _ := securityServer(t)
			hash, err := auth.Hash("correct-secret")
			if err != nil {
				t.Fatal(err)
			}
			owner, err := s.store.CreateUser(t.Context(), "Owner", true, "", hash, "")
			if err != nil {
				t.Fatal(err)
			}
			pinHash, err := auth.Hash("1234")
			if err != nil {
				t.Fatal(err)
			}
			viewer, err := s.store.CreateUser(t.Context(), "PIN viewer", false, "", "", pinHash)
			if err != nil {
				t.Fatal(err)
			}
			var handler http.HandlerFunc
			body := `{"password":"wrong"}`
			switch name {
			case "password":
				handler = s.login
			case "pin":
				handler = s.chooseProfile
				body = `{"id":"` + viewer.ID + `","pin":"0000"}`
			case "admin-profile":
				handler = s.chooseProfile
				body = `{"id":"` + owner.ID + `","password":"wrong"}`
			case "change-password":
				handler = s.changePassword
				body = `{"current":"wrong","new":"new-password"}`
			case "friend":
				handler = s.friendLogin
				body = `{"name":"Unknown","password":"wrong"}`
			case "invite":
				handler = s.acceptInvite
			case "link":
				handler = s.approveLink
				body = `{"code":"WRNG"}`
			}
			const burst = 20
			start := make(chan struct{})
			codes := make(chan int, burst)
			var wg sync.WaitGroup
			for range burst {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					r := httptest.NewRequest("POST", "http://arc/api/auth/login", strings.NewReader(body))
					r.RemoteAddr = "192.168.1.20:1234"
					r = r.WithContext(context.WithValue(r.Context(), userKey, (*store.User)(owner)))
					r.SetPathValue("token", "wrong-invite")
					w := httptest.NewRecorder()
					handler(w, r)
					codes <- w.Code
				}()
			}
			close(start)
			wg.Wait()
			close(codes)
			admitted := 0
			for code := range codes {
				if code != http.StatusTooManyRequests {
					admitted++
				}
			}
			if admitted > 5 {
				t.Fatalf("%d simultaneous wrong guesses reached verification; want at most 5", admitted)
			}
		})
	}
}
