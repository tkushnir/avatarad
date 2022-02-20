package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/nfnt/resize"
)

// Main configuration options
type Config struct {
	CAcrtFile       string `envconfig:"LDAP_SSL_CACERT_FILE"`
	LdapServerFQDN  string `envconfig:"LDAP_SERVER_FQDN" required:"true"`
	LdapPort        int    `envconfig:"LDAP_PORT" default:"636"`
	LdapSSL         bool   `envconfig:"LDAP_SSL" default:"true"`
	LdapTLS         bool   `envconfig:"LDAP_TLS" default:"false"`
	LdapVerifyCert  bool   `envconfig:"LDAP_VERIFY_CERT" default:"true"`
	LdapBindUser    string `envconfig:"LDAP_BIND_USER" required:"true"`
	LdapBindPasswd  string `envconfig:"LDAP_BIND_PASSWORD" required:"true"`
	LdapUserBase    string `envconfig:"LDAP_USER_BASE" required:"true"`
	LdapUserFilter  string `envconfig:"LDAP_USER_FILTER" default:"(objectclass=inetOrgPerson)"`
	LdapAvatarAttr  string `envconfig:"LDAP_AVATAR_ATTRIBUTE" default:"jpegPhoto"`
	LdapEmailAttr   string `envconfig:"LDAP_EMAIL_ATTRIBUTE" default:"mail"`
	GravatarEnabled bool   `envconfig:"GRAVATAR_ENABLED" default:"false"`
	GravatarURL     string `envconfig:"GRAVATAR_URL" default:"https://secure.gravatar.com/avatar"`
}

// Main HTTP service
type Service struct {
	httpServer *http.Server
	Running    chan struct{}
}

// Avatar element
type Avatar struct {
	Image      []byte
	LastUpdate time.Time
}

const (
	contentType         = "Content-Type"
	frameOptionsHeader  = "X-Frame-Options"
	frameOptionsValue   = "DENY"
	xssProtectionHeader = "X-XSS-Protection"
	xssProtectionValue  = "1; mode=block"
	serverPort          = ":8080"
)

var (
	cfg Config
	//go:embed media/default.jpg
	defaultAvatarImg []byte
	defaultAvatar    = Avatar{Image: defaultAvatarImg}
	hs               map[string]Avatar
	lock             = sync.RWMutex{}
	maxTime          time.Duration
	pkgVersion       string = ""
	epoch                   = time.Unix(0, 0).Format(time.RFC1123)
)

var noCacheHeaders = map[string]string{
	"Expires":         epoch,
	"Cache-Control":   "no-cache, no-store, no-transform, must-revalidate, private, max-age=0",
	"Pragma":          "no-cache",
	"X-Accel-Expires": "0",
}

func panicIf(err error, what ...string) {
	if err != nil {
		if len(what) == 0 {
			panic(err)
		}

		panic(errors.New(err.Error() + (" " + what[0])))
	}
}

func writeNoCacheHeaders(w http.ResponseWriter) {
	for k, v := range noCacheHeaders {
		w.Header().Set(k, v)
	}
}

func writeSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set(frameOptionsHeader, frameOptionsValue)
	w.Header().Set(xssProtectionHeader, xssProtectionValue)
}

// Function to create main service
func NewService() *Service {
	mux := http.NewServeMux()

	mux.HandleFunc("/version", versionHandler)
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/avatar/", avatarHandler)

	return &Service{
		httpServer: &http.Server{
			Addr:    serverPort,
			Handler: mux,
		},
		Running: make(chan struct{}),
	}
}

// Run the service
func (s *Service) Run() error {
	close(s.Running)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Close the service gracefully
func (s *Service) Shutdown() error {
	return s.httpServer.Shutdown(context.TODO())
}

func main() {
	maxTime, _ = time.ParseDuration("30m")

	err := envconfig.Process("", &cfg)
	panicIf(err, "while reading configuration")

	hs = make(map[string]Avatar)
	FillHash()

	svc := NewService()

	if err := svc.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, r)
		}
	}()

	writeNoCacheHeaders(w)
	writeSecurityHeaders(w)

	w.Header().Set(contentType, "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.Encode(map[string]string{"version": pkgVersion})
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, r)
		}
	}()

	writeNoCacheHeaders(w)

	w.Header().Set(contentType, "text/plain")
	io.WriteString(w, "OK")
}

func hsGet(h string) Avatar {
	lock.RLock()
	defer lock.RUnlock()

	return hs[h]
}

func hsWrite(h string, av Avatar) {
	lock.Lock()
	defer lock.Unlock()

	hs[h] = av
}

func hsDelete(h string) {
	lock.Lock()
	defer lock.Unlock()

	delete(hs, h)
}

// Delete hash map elements if they are outdated
func PruneHash() {
	for h := range hs {
		av := hsGet(h)
		if len(av.Image) > 0 && time.Since(av.LastUpdate) > maxTime {
			fmt.Fprintln(os.Stderr, h+" × cache")
			hsDelete(h)
		}
	}
}

func getAvatar(h string) Avatar {
	var (
		body []byte
		av   Avatar
	)

	av = hsGet(h)
	if len(av.Image) == 0 || time.Since(av.LastUpdate) > maxTime {
		PruneHash()
		FillHash()
		av = hsGet(h)
		if len(av.Image) > 0 {
			fmt.Fprintln(os.Stderr, h+" → cached")
			return av
		}
		fmt.Fprintln(os.Stderr, h+" × LDAP")
		if cfg.GravatarEnabled {
			res, err := http.Get(cfg.GravatarURL + "/" + h + "?s=490&d=404")
			if err == nil && res.StatusCode == 200 {
				body, err = io.ReadAll(res.Body)
				res.Body.Close()
				if err == nil {
					hsWrite(h, Avatar{
						Image:      body,
						LastUpdate: time.Now(),
					})
					fmt.Fprintln(os.Stderr, h+" → Gravatar")
					return hsGet(h)
				}
			}
			fmt.Fprintln(os.Stderr, h+" × Gravatar")
		}
		fmt.Fprintln(os.Stderr, h+" → default")
		return defaultAvatar
	}
	fmt.Fprintln(os.Stderr, h+" → cached")
	return av
}

func encodeAvatar(img image.Image, format string) ([]byte, error) {
	var err error

	buf := new(bytes.Buffer)
	switch {
	case format == "jpeg":
		err = jpeg.Encode(buf, img, &jpeg.Options{Quality: 90})
	case format == "png":
		err = png.Encode(buf, img)
	}
	return buf.Bytes(), err
}

func avatarHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, r)
		}
	}()

	// read request body
	_, err := io.ReadAll(r.Body)
	panicIf(err, "while reading request body")

	q := r.URL.Query()
	size := 80
	qSize := ""
	if s, ok := q["s"]; ok {
		qSize = s[0]
	} else if s, ok := q["size"]; ok {
		qSize = s[0]
	}
	if s, err := strconv.Atoi(qSize); err == nil {
		size = s
	}

	var (
		resizedImg    image.Image
		resizedAvatar []byte
		avatar        Avatar
	)

	hash := strings.Split(strings.Split(r.URL.Path, "/")[2], ".")[0]
	avatar = getAvatar(hash)

	buf := bytes.NewBuffer(avatar.Image)
	img, imgFormat, err := image.Decode(buf)
	panicIf(err, "while decoding avatar")

	resizedImg = resize.Resize(uint(size), 0, img, resize.Lanczos3)

	resizedAvatar, err = encodeAvatar(resizedImg, imgFormat)
	panicIf(err, "while encoding image")

	w.Header().Set(contentType, "image/"+imgFormat)
	w.Header().Set("Content-Length", strconv.Itoa(len(resizedAvatar)))
	w.Write(resizedAvatar)
}
