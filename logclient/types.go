package logclient

import (
	"crypto/x509"

	ctgo "github.com/google/certificate-transparency-go"
)

type Entry struct {
	Id          int64
	Log         *Log
	Certificate *x509.Certificate
	Chain       []*x509.Certificate
	CertData    []byte
}

func ParseRawLogEntry(id int64, log *Log, rawEntry *ctgo.RawLogEntry) Entry {
	entry := Entry{Id: id, Log: log, Chain: make([]*x509.Certificate, 0), CertData: rawEntry.Cert.Data}

	if cert, err := x509.ParseCertificate(rawEntry.Cert.Data); err == nil {
		entry.Certificate = cert
	}

	for _, rawCert := range rawEntry.Chain {
		if cert, err := x509.ParseCertificate(rawCert.Data); err == nil {
			entry.Chain = append(entry.Chain, cert)
		}
	}

	return entry
}

type LogMap map[string]*Log

type Page struct {
	start int64
	end   int64
}

func NewPage(start int64, pageSize int) (page Page) {
	page = Page{start: start, end: start + int64(pageSize) - 1}

	if page.start%int64(pageSize) != 0 {
		// align end to page size
		page.end = (page.start/int64(pageSize))*int64(pageSize) + int64(pageSize) - 1
	}

	return
}

type ById []Entry

func (a ById) Len() int           { return len(a) }
func (a ById) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ById) Less(i, j int) bool { return a[i].Id < a[j].Id }
