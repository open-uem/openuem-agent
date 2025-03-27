package sftp

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ocsp"
	gossh "golang.org/x/crypto/ssh"
)

type SFTP struct {
	Server ssh.Server
}

func sftpHandler(sess ssh.Session) {
	debugStream := io.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
	}
	server, err := sftp.NewServer(
		sess,
		serverOptions...,
	)
	if err != nil {
		log.Printf("[ERROR]: sftp server init error: %s\n", err)
		return
	}
	if err := server.Serve(); err == io.EOF {
		server.Close()
		log.Println("[INFO]: sftp client exited session.")
	} else if err != nil {
		log.Printf("[ERROR]: sftp server completed with error: %v", err)
	}
}

func New() *SFTP {
	s := SFTP{}
	return &s
}

func (s *SFTP) Serve(address string, sftpCert, caCert *x509.Certificate, db *badger.DB) error {
	s.Server = ssh.Server{
		Addr: address,
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			log.Printf("[INFO]: SSH session opened by %s", ctx.User())

			// Validate certificate against OCSP
			isValid, err := isCertValidFromCache(sftpCert, caCert, db)
			if err != nil {
				log.Println("[ERROR]: found and error trying to validate cert against cache")
				return false
			}

			if !isValid {
				log.Println("[ERROR]: cert it not valid according to cache")
				return false
			}

			// Check that the public key used is authorized
			rsaPublicKey := sftpCert.PublicKey.(*rsa.PublicKey)
			authorizedKey, err := gossh.NewPublicKey(rsaPublicKey)
			if err != nil {
				log.Println("[ERROR]: could not parse SSH public key from RSA public key")
				return false
			}

			return ssh.KeysEqual(key, authorizedKey)
		},
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": sftpHandler,
		},
	}

	return s.Server.ListenAndServe()
}

func isCertValidFromCache(sftpCert, caCert *x509.Certificate, db *badger.DB) (bool, error) {
	var ocspStatus bool

	certSerial := sftpCert.SerialNumber

	if err := db.View(
		func(tx *badger.Txn) error {
			item, err := tx.Get(certSerial.Bytes())
			if err != nil {
				if errors.Is(err, badger.ErrKeyNotFound) {
					ocspStatus, err = isCertValid(sftpCert, caCert)
					if err != nil {
						return err
					}

					if err := db.Update(func(txn *badger.Txn) error {
						var e *badger.Entry
						if ocspStatus {
							e = badger.NewEntry(certSerial.Bytes(), []byte("true")).WithTTL(time.Hour)
						} else {
							e = badger.NewEntry(certSerial.Bytes(), []byte("false")).WithTTL(time.Hour)
						}
						if err := txn.SetEntry(e); err != nil {
							return err
						}
						return nil
					}); err != nil {
						log.Println("[ERROR]: could not add cert OCSP status in cache")
						return err
					}
					return nil
				}
				return err
			}

			// Check value stored in cache
			valCopy, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			ocspStatus, err = strconv.ParseBool(string(valCopy))
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
		log.Println("[ERROR]: could not check OCSP status in cache")
		return false, err
	}
	return ocspStatus, nil
}

func isCertValid(sftpCert, caCert *x509.Certificate) (bool, error) {
	ocspRequest, err := ocsp.CreateRequest(sftpCert, caCert, &ocsp.RequestOptions{Hash: crypto.SHA256})
	if err != nil {
		log.Println("[ERROR]: could not create OCSP Request")
		return false, err
	}

	if len(sftpCert.OCSPServer) == 0 {
		log.Println("[ERROR]: no OCSP server found in certificate")
		return false, err
	}

	ocspServer := sftpCert.OCSPServer[0]
	ocspURL, err := url.Parse(ocspServer)
	if err != nil {
		log.Println("[ERROR]: could not parse OCSP Responder URL")
		return false, err
	}

	httpRequest, err := http.NewRequest(http.MethodPost, ocspServer, bytes.NewBuffer(ocspRequest))
	if err != nil {
		log.Println("[ERROR]: could not create HTTP request to OCSP Responder")
		return false, err
	}

	httpRequest.Header.Add("Content-Type", "application/ocsp-request")
	httpRequest.Header.Add("Accept", "application/ocsp-response")
	httpRequest.Header.Add("host", ocspURL.Host)

	httpClient := &http.Client{}
	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		log.Println("[ERROR]: could not send request to OCSP Responder")
		return false, err
	}
	defer httpResponse.Body.Close()
	output, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		log.Println("[ERROR]: could not read response from OCSP Responder")
		return false, err
	}

	ocspResponse, err := ocsp.ParseResponse(output, caCert)
	if err != nil {
		log.Println("[ERROR]: could not parse OCSP Response")
		return false, err
	}

	if ocspResponse.Status == 2 {
		log.Println("[ERROR]: could not check OCSP status, try again later")
		return false, err
	}

	if ocspResponse.Status == 1 {
		log.Println("[ERROR]: unauthorized. Your certificate has been revoked")
		return false, err
	}
	return true, nil
}
