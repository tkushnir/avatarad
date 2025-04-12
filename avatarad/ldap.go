package main

import (
	"crypto/md5" // #nosec G501
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/go-ldap/ldap/v3"
)

var (
	certsInit = false
	rootCA    *x509.CertPool
	tlsConfig tls.Config
)

func prepareCerts() {
	var err error

	if cfg.LdapSSL || cfg.LdapTLS {
		rootCA, err = x509.SystemCertPool()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to load system CA pool: %v\n", err)
			rootCA = x509.NewCertPool()
		}

		if len(cfg.CAcrtFile) != 0 {
			caCert, err := os.ReadFile(cfg.CAcrtFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unable to read CA certificate: %v\n", err)
			} else if ok := rootCA.AppendCertsFromPEM(caCert); !ok {
				fmt.Fprintf(os.Stderr, "Unable to add CA certificate\n")
			}
		}

		tlsConfig = tls.Config{
			InsecureSkipVerify: !cfg.LdapVerifyCert, // #nosec G402
			ServerName:         cfg.LdapServerFQDN,
			RootCAs:            rootCA,
		}
	}
}

func getEntries() []*ldap.Entry {
	var (
		l   *ldap.Conn
		err error
	)

	if !certsInit {
		prepareCerts()
		certsInit = true
	}

	ldapServPort := fmt.Sprintf("%s:%d", cfg.LdapServerFQDN, cfg.LdapPort)

	if cfg.LdapSSL {
		l, err = ldap.DialURL("ldaps://"+ldapServPort, ldap.DialWithTLSConfig(&tlsConfig))
	} else {
		l, err = ldap.DialURL("ldap://" + ldapServPort)
	}
	panicIf(err, "while connecting to LDAP server "+ldapServPort)
	defer func() {
		err = l.Close()
		panicIf(err, "while closing connection to LDAP server "+ldapServPort)
	}()

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

	return sr.Entries
}

func fillHash() {
	for _, entry := range getEntries() {
		mail := entry.GetAttributeValue(cfg.LdapEmailAttr)
		if len(mail) == 0 {
			continue
		}

		av := entry.GetRawAttributeValue(cfg.LdapAvatarAttr)
		if len(av) == 0 {
			continue
		}

		hash := fmt.Sprintf("%x", md5.Sum([]byte(mail))) // #nosec G401
		avtr := hsGet(hash)
		if len(avtr.Image) > 0 && time.Since(avtr.LastUpdate) <= maxTime {
			continue
		}

		fmt.Fprintln(os.Stderr, hash+" → LDAP")
		hsWrite(hash, avatar{
			Image:      av,
			LastUpdate: time.Now(),
		})

		hash = fmt.Sprintf("%x", sha256.Sum256([]byte(mail)))
		avtr = hsGet(hash)
		if len(avtr.Image) > 0 && time.Since(avtr.LastUpdate) <= maxTime {
			continue
		}

		fmt.Fprintln(os.Stderr, hash+" → LDAP")
		hsWrite(hash, avatar{
			Image:      av,
			LastUpdate: time.Now(),
		})
	}
}
