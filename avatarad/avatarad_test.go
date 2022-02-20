package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Version struct {
	Source  string `json:"source,omitempty"`
	Version string `json:"version,omitempty"`
	Commit  string `json:"commit,omitempty"`
}

type Conf struct {
	LdapServerFQDN string `envconfig:"LDAP_HOSTNAME" required:"true"`
	LdapBindUser   string `envconfig:"LDAP_BIND_USER" required:"true"`
	LdapBindPasswd string `envconfig:"LDAP_ADMIN_PASSWORD" required:"true"`
	LdapUserBase   string `envconfig:"LDAP_USER_BASE" required:"true"`
}

const (
	ldapPort    = "389"
	gravatarURL = "http://secure.gravatar.com/avatar"
)

var conf Conf

func TestMain(m *testing.M) {
	maxTime, _ = time.ParseDuration("30m")

	if err := envconfig.Process("", &conf); err != nil {
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

	if got, want := w.Code, 200; want != got {
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

func TestHandleHealthzError(t *testing.T) {
	healthzHandler(nil, nil)
}

func TestHandleVersion(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/version", nil)
	v := Version{}

	versionHandler(w, r)

	if got, want := w.Code, 200; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	if _, err := io.ReadAll(w.Body); err != nil {
		t.Errorf("%v while reading response body", err)
	}

	json.NewDecoder(w.Body).Decode(&v)
	if v.Version != pkgVersion {
		t.Errorf("response body does not match expected result: "+
			"want '%s', got '%s'", pkgVersion, v.Version)
	}
}

func TestHandleVersionError(t *testing.T) {
	versionHandler(nil, nil)
}

func TestHandleAvatar(t *testing.T) {
	os.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	os.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	os.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	os.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	os.Setenv("LDAP_VERIFY_CERT", "false")
	os.Setenv("GRAVATAR_ENABLED", "true")
	os.Setenv("GRAVATAR_URL", gravatarURL)

	_ = envconfig.Process("", &cfg)

	hs = make(map[string]Avatar)
	FillHash()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/00000000000000000000000000000000", nil)

	avatarHandler(w, r)

	if got, want := w.Code, 200; want != got {
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

func TestHandleAvatarEve(t *testing.T) {
	os.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	os.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	os.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	os.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	os.Setenv("LDAP_VERIFY_CERT", "false")
	os.Setenv("GRAVATAR_ENABLED", "true")
	os.Setenv("GRAVATAR_URL", gravatarURL)

	_ = envconfig.Process("", &cfg)

	hs = make(map[string]Avatar)
	FillHash()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/38ff3520bdcc16a3bbe247f78a8e1610", nil)

	avatarHandler(w, r)

	if got, want := w.Code, 200; want != got {
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

func TestHandleAvatarEveUpdate(t *testing.T) {
	os.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	os.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	os.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	os.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	os.Setenv("LDAP_VERIFY_CERT", "false")
	os.Setenv("GRAVATAR_ENABLED", "true")
	os.Setenv("GRAVATAR_URL", gravatarURL)

	_ = envconfig.Process("", &cfg)

	hs = make(map[string]Avatar)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/38ff3520bdcc16a3bbe247f78a8e1610", nil)

	avatarHandler(w, r)

	if got, want := w.Code, 200; want != got {
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

func TestHandleAvatarEveInvalidate(t *testing.T) {
	const m string = "38ff3520bdcc16a3bbe247f78a8e1610"

	os.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	os.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	os.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	os.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	os.Setenv("LDAP_VERIFY_CERT", "false")
	os.Setenv("GRAVATAR_ENABLED", "true")
	os.Setenv("GRAVATAR_URL", gravatarURL)

	_ = envconfig.Process("", &cfg)

	hs = make(map[string]Avatar)
	FillHash()

	av := hsGet(m)
	av.LastUpdate = av.LastUpdate.Add(-time.Hour)
	hsWrite(m, av)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/"+m, nil)

	avatarHandler(w, r)

	if got, want := w.Code, 200; want != got {
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

func TestHandleAvatarGitea(t *testing.T) {
	os.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	os.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	os.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	os.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	os.Setenv("LDAP_VERIFY_CERT", "false")
	os.Setenv("GRAVATAR_ENABLED", "true")
	os.Setenv("GRAVATAR_URL", gravatarURL)

	_ = envconfig.Process("", &cfg)

	hs = make(map[string]Avatar)
	FillHash()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/b3ba9ac9a9461847e97fa0c39b4ba531", nil)

	avatarHandler(w, r)

	if got, want := w.Code, 200; want != got {
		t.Errorf("Want response code %d, got %d", want, got)
	}

	_, imgType, err := image.Decode(w.Body)
	if err != nil {
		t.Errorf("%v while decoding response body", err)
	}

	if got, want := imgType, "png"; got != want {
		t.Errorf("Want image type '%s', got '%s'", want, got)
	}
}

func TestHandleAvatarSz(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/00000000000000000000000000000000?s=290", nil)

	avatarHandler(w, r)

	if got, want := w.Code, 200; want != got {
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

func TestHandleAvatarSize(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/avatar/00000000000000000000000000000000?size=290", nil)

	avatarHandler(w, r)

	if got, want := w.Code, 200; want != got {
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

func TestHandleAvatarError(t *testing.T) {
	avatarHandler(nil, nil)
}

func TestPanicIf(t *testing.T) {
	err := errors.New("Test error message")

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	panicIf(err)
}

func TestPanicIfWhat(t *testing.T) {
	err := errors.New("Test error message")

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	panicIf(err, "while testing")
}

func TestRunService(t *testing.T) {
	svc := NewService()
	svcDone := make(chan struct{})

	go func() {
		svc.Run()
		defer close(svcDone)
	}()

	<-svc.Running

	// do checks here

	svc.Shutdown()

	<-svcDone
}

func TestMainBindError(t *testing.T) {
	l, err := net.Listen("tcp", serverPort)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	defer l.Close()

	os.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	os.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	os.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	os.Setenv("LDAP_VERIFY_CERT", "false")
	os.Setenv("LDAP_USER_BASE", conf.LdapUserBase)

	main()
}

func TestCACertNoFile(t *testing.T) {
	os.Setenv("LDAP_SSL_CACERT_FILE", "/path/does/not/exist")
	os.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	os.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	os.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	os.Setenv("LDAP_VERIFY_CERT", "false")
	os.Setenv("LDAP_USER_BASE", conf.LdapUserBase)

	_ = envconfig.Process("", &cfg)

	PrepareCerts()
}

func TestCACertDevNull(t *testing.T) {
	os.Setenv("LDAP_SSL_CACERT_FILE", "/dev/null")
	os.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	os.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	os.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	os.Setenv("LDAP_VERIFY_CERT", "false")
	os.Setenv("LDAP_USER_BASE", conf.LdapUserBase)

	_ = envconfig.Process("", &cfg)

	PrepareCerts()
}

func TestMainNoSSL(t *testing.T) {
	l, err := net.Listen("tcp", serverPort)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	defer l.Close()

	os.Setenv("LDAP_SSL_CACERT_FILE", "/dev/null")
	os.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	os.Setenv("LDAP_PORT", ldapPort)
	os.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	os.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	os.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	os.Setenv("LDAP_SSL", "false")

	main()
}

func TestMainUseTLS(t *testing.T) {
	l, err := net.Listen("tcp", serverPort)
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	defer l.Close()

	os.Setenv("LDAP_SSL_CACERT_FILE", "/dev/null")
	os.Setenv("LDAP_SERVER_FQDN", conf.LdapServerFQDN)
	os.Setenv("LDAP_PORT", ldapPort)
	os.Setenv("LDAP_BIND_USER", conf.LdapBindUser)
	os.Setenv("LDAP_BIND_PASSWORD", conf.LdapBindPasswd)
	os.Setenv("LDAP_USER_BASE", conf.LdapUserBase)
	os.Setenv("LDAP_SSL", "false")
	os.Setenv("LDAP_TLS", "true")

	main()
}
