// Main package for avatarad service
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

	"github.com/caarlos0/env/v10"
	"github.com/nfnt/resize"
)

type config struct {
	CAcrtFile       string `env:"LDAP_SSL_CACERT_FILE"`
	LdapServerFQDN  string `env:"LDAP_SERVER_FQDN,required"`
	LdapPort        int    `env:"LDAP_PORT"                   envDefault:"636"`
	LdapSSL         bool   `env:"LDAP_SSL"                    envDefault:"true"`
	LdapTLS         bool   `env:"LDAP_TLS"                    envDefault:"false"`
	LdapVerifyCert  bool   `env:"LDAP_VERIFY_CERT"            envDefault:"true"`
	LdapBindUser    string `env:"LDAP_BIND_USER,required"`
	LdapBindPasswd  string `env:"LDAP_BIND_PASSWORD,required"`
	LdapUserBase    string `env:"LDAP_USER_BASE,required"`
	LdapUserFilter  string `env:"LDAP_USER_FILTER"            envDefault:"(objectclass=inetOrgPerson)"`
	LdapAvatarAttr  string `env:"LDAP_AVATAR_ATTRIBUTE"       envDefault:"jpegPhoto"`
	LdapEmailAttr   string `env:"LDAP_EMAIL_ATTRIBUTE"        envDefault:"mail"`
	GravatarEnabled bool   `env:"GRAVATAR_ENABLED"            envDefault:"false"`
	GravatarURL     string `env:"GRAVATAR_URL"                envDefault:"https://secure.gravatar.com/avatar"`
}

type service struct {
	httpServer *http.Server
	Running    chan struct{}
}

type avatar struct {
	Image      []byte
	LastUpdate time.Time
}

const (
	contentType         = "Content-Type"
	defaultTimeout      = 3
	defaultJpegQuality  = 90
	frameOptionsHeader  = "X-Frame-Options"
	frameOptionsValue   = "DENY"
	serverPort          = ":8080"
	xssProtectionHeader = "X-XSS-Protection"
	xssProtectionValue  = "1; mode=block"
)

var (
	cfg config
	//go:embed media/default.jpg
	defaultAvatar []byte
	hs            map[string]avatar
	lock          = sync.RWMutex{}
	maxTime       time.Duration
	pkgVersion    string
	epoch         = time.Unix(0, 0).Format(time.RFC1123)
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

func newService() *service {
	mux := http.NewServeMux()

	mux.HandleFunc("/version", versionHandler)
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/avatar/", avatarHandler)

	return &service{
		httpServer: &http.Server{
			Addr:              serverPort,
			Handler:           mux,
			ReadHeaderTimeout: defaultTimeout * time.Second,
		},
		Running: make(chan struct{}),
	}
}

func (s *service) run() error {
	close(s.Running)
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *service) shutdown() error {
	return s.httpServer.Shutdown(context.TODO())
}

func main() {
	maxTime, _ = time.ParseDuration("30m")

	err := env.Parse(&cfg)
	panicIf(err, "while reading configuration")

	hs = make(map[string]avatar)
	fillHash()

	svc := newService()

	if err := svc.run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func versionHandler(w http.ResponseWriter, _ *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, r)
		}
	}()

	writeNoCacheHeaders(w)
	writeSecurityHeaders(w)

	w.Header().Set(contentType, "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	if err := enc.Encode(map[string]string{"version": pkgVersion}); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, r)
		}
	}()

	writeNoCacheHeaders(w)

	w.Header().Set(contentType, "text/plain")
	if _, err := io.WriteString(w, "OK"); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func hsGet(h string) avatar {
	lock.RLock()
	defer lock.RUnlock()

	return hs[h]
}

func hsWrite(h string, av avatar) {
	lock.Lock()
	defer lock.Unlock()

	hs[h] = av
}

func hsDelete(h string) {
	lock.Lock()
	defer lock.Unlock()

	delete(hs, h)
}

func pruneHash() {
	for h := range hs {
		av := hsGet(h)
		if len(av.Image) > 0 && time.Since(av.LastUpdate) > maxTime {
			fmt.Fprintln(os.Stderr, h+" × cache")
			hsDelete(h)
		}
	}
}

func getAvatar(h string) avatar {
	var (
		body []byte
		av   avatar
	)

	av = hsGet(h)
	if len(av.Image) == 0 || time.Since(av.LastUpdate) > maxTime {
		pruneHash()
		fillHash()
		av = hsGet(h)
		if len(av.Image) > 0 {
			fmt.Fprintln(os.Stderr, h+" → cached")

			return av
		}
		fmt.Fprintln(os.Stderr, h+" × LDAP")
		if cfg.GravatarEnabled {
			res, err := http.Get(cfg.GravatarURL + "/" + h + "?s=490&d=" + strconv.Itoa(http.StatusNotFound))
			if err == nil && res.StatusCode == http.StatusOK {
				body, err = io.ReadAll(res.Body)
				cerr := res.Body.Close()
				if err == nil && cerr == nil {
					hsWrite(h, avatar{
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

		hsWrite(h, avatar{
			Image:      defaultAvatar,
			LastUpdate: time.Now(),
		})

		return hsGet(h)
	}
	fmt.Fprintln(os.Stderr, h+" → cached")

	return av
}

func encodeAvatar(img image.Image, format string) ([]byte, error) {
	var err error

	buf := new(bytes.Buffer)
	switch format {
	case "jpeg":
		err = jpeg.Encode(buf, img, &jpeg.Options{Quality: defaultJpegQuality})
	case "png":
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

	var size uint64

	q := r.URL.Query()
	size = 80
	qSize := ""
	if s, ok := q["s"]; ok {
		qSize = s[0]
	} else if s, ok := q["size"]; ok {
		qSize = s[0]
	}
	if s, err := strconv.ParseUint(qSize, 10, 64); err == nil {
		size = s
	}

	var (
		resizedImg    image.Image
		resizedAvatar []byte
		avatar        avatar
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
	if _, err := w.Write(resizedAvatar); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
