package main

import (
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/go-ldap/ldap/v3"
)

var (
	certsInit bool = false
	rootCA    *x509.CertPool
	tlsConfig tls.Config
)

// Prepare certificates for LDAP server access
func PrepareCerts() {
	var err error

	if cfg.LdapSSL || cfg.LdapTLS {
		rootCA, err = x509.SystemCertPool()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to load system CA pool: %v\n", err)
			rootCA = x509.NewCertPool()
		}

		if len(cfg.CAcrtFile) != 0 {
			caCert, err := ioutil.ReadFile(cfg.CAcrtFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unable to read CA certificate: %v\n", err)
			} else if ok := rootCA.AppendCertsFromPEM(caCert); !ok {
				fmt.Fprintf(os.Stderr, "Unable to add CA certificate\n")
			}
		}

		tlsConfig = tls.Config{
			InsecureSkipVerify: !cfg.LdapVerifyCert,
			ServerName:         cfg.LdapServerFQDN,
			RootCAs:            rootCA,
		}
	}
}

// Fill hash map with Avatar elements from LDAP
func FillHash() {
	var (
		l   *ldap.Conn
		err error
	)

	if !certsInit {
		PrepareCerts()
		certsInit = true
	}

	ldapServPort := fmt.Sprintf("%s:%d", cfg.LdapServerFQDN, cfg.LdapPort)

	if cfg.LdapSSL {
		l, err = ldap.DialTLS("tcp", ldapServPort, &tlsConfig)
	} else {
		l, err = ldap.Dial("tcp", ldapServPort)
	}
	panicIf(err, "while connecting to LDAP server "+ldapServPort)
	defer l.Close()

	if !cfg.LdapSSL && cfg.LdapTLS {
		err = l.StartTLS(&tlsConfig)
		panicIf(err, "while reconnecting to LDAP server "+ldapServPort+" using TLS")
	}

	err = l.Bind(cfg.LdapBindUser, cfg.LdapBindPasswd)
	panicIf(err, "while binding to LDAP server"+ldapServPort)

	searchRequest := ldap.NewSearchRequest(cfg.LdapUserBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		cfg.LdapUserFilter, []string{cfg.LdapEmailAttr, cfg.LdapAvatarAttr}, nil)

	sr, err := l.Search(searchRequest)
	panicIf(err, "while searching LDAP database")

	for _, entry := range sr.Entries {
		mail := entry.GetAttributeValue(cfg.LdapEmailAttr)
		if len(mail) == 0 {
			continue
		}

		avatar := entry.GetRawAttributeValue(cfg.LdapAvatarAttr)
		if len(avatar) == 0 {
			continue
		}

		h := md5.New()

		if _, err := io.WriteString(h, mail); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			continue
		}
		hash := fmt.Sprintf("%x", h.Sum(nil))
		av := hsGet(hash)
		if len(av.Image) > 0 && time.Since(av.LastUpdate) <= maxTime {
			continue
		}

		fmt.Fprintln(os.Stderr, hash+" → LDAP")
		hsWrite(hash, Avatar{
			Image:      avatar,
			LastUpdate: time.Now(),
		})
	}
}
