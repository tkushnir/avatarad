package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/caarlos0/env/v10"
)

type Version struct {
	Source  string `json:"source,omitempty"`
	Version string `json:"version,omitempty"`
	Commit  string `json:"commit,omitempty"`
}

type Conf struct {
	LdapServerFQDN string `env:"LDAP_HOSTNAME,required"`
	LdapBindUser   string `env:"LDAP_BIND_USER,required"`
	LdapBindPasswd string `env:"LDAP_ADMIN_PASSWORD,required"`
	LdapUserBase   string `env:"LDAP_USER_BASE,required"`
}

const (
	ldapPort    = "389"
	gravatarURL = "http://secure.gravatar.com/avatar"
	strJpeg     = "jpeg"
)

var (
	errTestErrorMsg = errors.New("test error message")
)

var conf Conf

func TestMain(m *testing.M) {
	maxTime, _ = time.ParseDuration("30m")

	pkgVersion = "0.1.1.23"

	if err := env.Parse(&conf); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	exitVal := m.Run()

	os.Exit(exitVal)
}

func TestHandleHealthz(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/healthz", nil)

	healthzHandler(w, r)

	if got, want := w.Code, http.StatusOK; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	data, err := io.ReadAll(w.Body)
	if err != nil {
		t.Errorf("%v while reading response body", err)
	}

	s := string(data)
	if s != "OK" {
		t.Errorf("Want response 'OK', got '%s'", s)
	}
}

func TestHandleHealthzError(_ *testing.T) {
	healthzHandler(nil, nil)
}

func TestHandleVersion(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/version", nil)
	v := Version{}

	versionHandler(w, r)

	res := w.Result()

	defer func() {
		if err := res.Body.Close(); err != nil {
			t.Errorf("%v while closing stream", err)
		}
	}()

	if got, want := w.Code, http.StatusOK; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
		t.Errorf("%v while decoding response body", err)
	}

	if v.Version != pkgVersion {
		t.Errorf("response body does not match expected result: "+
			"want '%s', got '%s'", pkgVersion, v.Version)
	}
}

func TestHandleVersionError(_ *testing.T) {
	versionHandler(nil, nil)
}

func TestHandleAvatar(t *testing.T) {
	t.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	t.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	t.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	t.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	t.Setenv("LDAP_VERIFY_CERT", "false")

	_ = env.Parse(&cfg)

	hs = make(map[string]avatar)
	fillHash()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/00000000000000000000000000000000", nil)

	avatarHandler(w, r)

	if got, want := w.Code, http.StatusOK; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	_, imgType, err := image.Decode(w.Body)
	if err != nil {
		t.Errorf("%v while decoding response body", err)
	}

	if got, want := imgType, strJpeg; got != want {
		t.Errorf("Want image type '%s', got '%s'", want, got)
	}
}

func TestHandleAvatarEve(t *testing.T) {
	t.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	t.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	t.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	t.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	t.Setenv("LDAP_VERIFY_CERT", "false")

	_ = env.Parse(&cfg)

	hs = make(map[string]avatar)
	fillHash()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/38ff3520bdcc16a3bbe247f78a8e1610", nil)

	avatarHandler(w, r)

	if got, want := w.Code, http.StatusOK; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	_, imgType, err := image.Decode(w.Body)
	if err != nil {
		t.Errorf("%v while decoding response body", err)
	}

	if got, want := imgType, strJpeg; got != want {
		t.Errorf("Want image type '%s', got '%s'", want, got)
	}
}

func TestHandleAvatarEveUpdate(t *testing.T) {
	t.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	t.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	t.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	t.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	t.Setenv("LDAP_VERIFY_CERT", "false")

	_ = env.Parse(&cfg)

	hs = make(map[string]avatar)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/38ff3520bdcc16a3bbe247f78a8e1610", nil)

	avatarHandler(w, r)

	if got, want := w.Code, http.StatusOK; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	_, imgType, err := image.Decode(w.Body)
	if err != nil {
		t.Errorf("%v while decoding response body", err)
	}

	if got, want := imgType, strJpeg; got != want {
		t.Errorf("Want image type '%s', got '%s'", want, got)
	}
}

func TestHandleAvatarEveInvalidate(t *testing.T) {
	const m string = "38ff3520bdcc16a3bbe247f78a8e1610"

	t.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	t.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	t.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	t.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	t.Setenv("LDAP_VERIFY_CERT", "false")

	_ = env.Parse(&cfg)

	hs = make(map[string]avatar)
	fillHash()

	av := hsGet(m)
	av.LastUpdate = av.LastUpdate.Add(-time.Hour)
	hsWrite(m, av)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/"+m, nil)

	avatarHandler(w, r)

	if got, want := w.Code, http.StatusOK; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	_, imgType, err := image.Decode(w.Body)
	if err != nil {
		t.Errorf("%v while decoding response body", err)
	}

	if got, want := imgType, strJpeg; got != want {
		t.Errorf("Want image type '%s', got '%s'", want, got)
	}
}

func TestHandleAvatarGitea(t *testing.T) {
	t.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	t.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	t.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	t.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	t.Setenv("LDAP_VERIFY_CERT", "false")
	t.Setenv("GRAVATAR_ENABLED", "true")
	t.Setenv("GRAVATAR_URL", gravatarURL)

	_ = env.Parse(&cfg)

	hs = make(map[string]avatar)
	fillHash()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/b3ba9ac9a9461847e97fa0c39b4ba531", nil)

	avatarHandler(w, r)

	if got, want := w.Code, http.StatusOK; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	_, imgType, err := image.Decode(w.Body)
	if err != nil {
		t.Errorf("%v while decoding response body", err)
	}

	if got, want := imgType, "jpeg"; got != want {
		t.Errorf("Want image type '%s', got '%s'", want, got)
	}
}

func TestHandleAvatarSz(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/00000000000000000000000000000000?s=290", nil)

	avatarHandler(w, r)

	if got, want := w.Code, http.StatusOK; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	_, imgType, err := image.Decode(w.Body)
	if err != nil {
		t.Errorf("%v while decoding response body", err)
	}

	if got, want := imgType, strJpeg; got != want {
		t.Errorf("Want image type '%s', got '%s'", want, got)
	}
}

func TestHandleAvatarSize(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/00000000000000000000000000000000?size=290", nil)

	avatarHandler(w, r)

	if got, want := w.Code, http.StatusOK; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	_, imgType, err := image.Decode(w.Body)
	if err != nil {
		t.Errorf("%v while decoding response body", err)
	}

	if got, want := imgType, strJpeg; got != want {
		t.Errorf("Want image type '%s', got '%s'", want, got)
	}
}

func TestHandleAvatarError(_ *testing.T) {
	avatarHandler(nil, nil)
}

func TestPanicIf(t *testing.T) {
	err := errTestErrorMsg

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	panicIf(err)
}

func TestPanicIfWhat(t *testing.T) {
	err := errTestErrorMsg

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	panicIf(err, "while testing")
}

func TestRunService(t *testing.T) {
	svc := newService()
	svcDone := make(chan struct{})

	go func() {
		if err := svc.run(); err != nil {
			t.Errorf("Cannot run service")
		}
		defer close(svcDone)
	}()

	<-svc.Running

	// do checks here

	if err := svc.shutdown(); err != nil {
		t.Errorf("Cannot shutdown service gracefully")
	}

	<-svcDone
}

func TestMainBindError(t *testing.T) {
	l, err := net.Listen("tcp", serverPort)
	if err != nil {
		t.Errorf("%v", err)

		return
	}
	defer func() {
		if err = l.Close(); err != nil {
			t.Errorf("%v while closing connection", err)
		}
	}()

	t.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	t.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	t.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	t.Setenv("LDAP_VERIFY_CERT", "false")
	t.Setenv("LDAP_USER_BASE", conf.LdapUserBase)

	main()
}

func TestCACertNoFile(t *testing.T) {
	t.Setenv("LDAP_SSL_CACERT_FILE", "/path/does/not/exist")
	t.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	t.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	t.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	t.Setenv("LDAP_VERIFY_CERT", "false")
	t.Setenv("LDAP_USER_BASE", conf.LdapUserBase)

	_ = env.Parse(&cfg)

	prepareCerts()
}

func TestCACertDevNull(t *testing.T) {
	t.Setenv("LDAP_SSL_CACERT_FILE", "/dev/null")
	t.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	t.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	t.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	t.Setenv("LDAP_VERIFY_CERT", "false")
	t.Setenv("LDAP_USER_BASE", conf.LdapUserBase)

	_ = env.Parse(&cfg)

	prepareCerts()
}

func TestMainNoSSL(t *testing.T) {
	l, err := net.Listen("tcp", serverPort)
	if err != nil {
		t.Errorf("%v", err)

		return
	}
	defer func() {
		if err = l.Close(); err != nil {
			t.Errorf("%v while closing connection", err)
		}
	}()

	t.Setenv("LDAP_SSL_CACERT_FILE", "/dev/null")
	t.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	t.Setenv("LDAP_PORT", ldapPort)
	t.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	t.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	t.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	t.Setenv("LDAP_SSL", "false")

	main()
}

func TestMainUseTLS(t *testing.T) {
	l, err := net.Listen("tcp", serverPort)
	if err != nil {
		t.Errorf("%v", err)

		return
	}
	defer func() {
		if err = l.Close(); err != nil {
			t.Errorf("%v while closing connection", err)
		}
	}()

	t.Setenv("LDAP_SSL_CACERT_FILE", "/dev/null")
	t.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	t.Setenv("LDAP_PORT", ldapPort)
	t.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	t.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	t.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	t.Setenv("LDAP_SSL", "false")
	t.Setenv("LDAP_TLS", "true")

	main()
}
