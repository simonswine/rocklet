package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var genCertValidity time.Duration
var genCertHosts []string

var genCertCmd = &cobra.Command{
	Use:   "gencert",
	Short: "Generate rockrobo certificate",
	Run: func(cmd *cobra.Command, args []string) {
		logger := zerolog.New(os.Stdout).With().Str("app", "gencert").Logger().Level(zerolog.DebugLevel)

		// read CA cert
		caCertBytes, err := ioutil.ReadFile(flags.Kubernetes.CACertPath)
		if err != nil {
			logger.Fatal().Err(err).Msg("can't read CA cert")
		}
		block, _ := pem.Decode(caCertBytes)
		if block == nil {
			logger.Fatal().Msg("failed to parse CA cert PEM")
		}
		caCert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to parse CA cert")
		}

		// read CA key
		caKeyBytes, err := ioutil.ReadFile(flags.Kubernetes.CAKeyPath)
		if err != nil {
			logger.Fatal().Err(err).Msg("can't read CA key")
		}
		block, _ = pem.Decode(caKeyBytes)
		if block == nil {
			logger.Fatal().Msg("failed to parse CA key PEM")
		}
		caKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to parse CA key")
		}

		// read key key if exists
		var key *rsa.PrivateKey
		keyBytes, err := ioutil.ReadFile(flags.Kubernetes.KeyPath)
		if os.IsNotExist(err) {
			// create new key
			key, err = rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				logger.Fatal().Err(err).Msg("can't generate key")
			}

			keyOut, err := os.OpenFile(flags.Kubernetes.KeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
			if err != nil {
				logger.Fatal().Err(err).Msg("can't write key")
			}
			pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
			keyOut.Close()
			logger.Info().Msgf("written private key to %s", flags.Kubernetes.KeyPath)
		} else if err != nil {
			logger.Fatal().Err(err).Msg("can't read key")
		} else {
			block, _ = pem.Decode(keyBytes)
			if block == nil {
				logger.Fatal().Msg("failed to parse key PEM")
			}
			key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				logger.Fatal().Err(err).Msg("failed to parse key")
			}
		}

		serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
		serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to generate serial number")
		}

		template := x509.Certificate{
			SerialNumber: serialNumber,
			Subject: pkix.Name{
				CommonName:   fmt.Sprintf("system:node:%s", flags.Kubernetes.NodeName),
				Organization: []string{"system:nodes"}, // "system:masters"}
			},
			NotBefore:             time.Now().Add(-24 * time.Hour),
			NotAfter:              time.Now().Add(genCertValidity),
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			DNSNames:              []string{flags.Kubernetes.NodeName},
		}

		// add addtional sans
		for _, h := range genCertHosts {
			if ip := net.ParseIP(h); ip != nil {
				template.IPAddresses = append(template.IPAddresses, ip)
			} else {
				template.DNSNames = append(template.DNSNames, h)
			}
		}

		// sign certificate
		derBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &key.PublicKey, caKey)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to create certificate")
		}
		certOut, err := os.Create(flags.Kubernetes.CertPath)
		if err != nil {
			logger.Fatal().Err(err).Msgf("failed to open %s for writing", flags.Kubernetes.CertPath)
		}
		pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
		certOut.Close()
		logger.Info().Msgf("written certificate to %s", flags.Kubernetes.CertPath)

	},
}

func init() {
	rootCmd.AddCommand(genCertCmd)
	genCertCmd.Flags().StringSliceVar(&genCertHosts, "certificate-sans", []string{}, "SAN DNS or IP addresses for cert")
	genCertCmd.Flags().DurationVar(&genCertValidity, "certificate-validity", time.Hour*24*365, "Validity of the certificate")
}
