package dns

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
)

type DoHClient struct {
	upstream string
	c        *http.Client
}

var client *DoHClient
var clientOnce sync.Once

func GetDoHClient(upstream string) *DoHClient {
	clientOnce.Do(func() {
		if client == nil {
			if !strings.HasPrefix(upstream, "https://") {
				upstream = "https://" + upstream
			}

			if !strings.HasSuffix(upstream, "/dns-query") {
				upstream = upstream + "/dns-query"
			}

			c := &http.Client{
				Timeout: 5 * time.Second,
				Transport: &http.Transport{
					DialContext: (&net.Dialer{
						Timeout:   3 * time.Second,
						KeepAlive: 30 * time.Second,
					}).DialContext,
					TLSHandshakeTimeout: 5 * time.Second,
					MaxIdleConnsPerHost: 100,
					MaxIdleConns:        100,
				},
			}

			client = &DoHClient{
				upstream: upstream,
				c:        c,
			}
		}
	})

	return client
}

func (d *DoHClient) doGetRequest(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	pack, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s?dns=%s", d.upstream, base64.RawStdEncoding.EncodeToString(pack))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.Header.Set("Accept", "application/dns-message")

	resp, err := d.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Debug("[DoH] Error while resolving ", url, " : ", resp.Status)
	}

	buf := bytes.Buffer{}
	buf.ReadFrom(resp.Body)

	ret_msg := new(dns.Msg)
	err = ret_msg.Unpack(buf.Bytes())
	if err != nil {
		return nil, err
	}

	return ret_msg, nil
}

func (d *DoHClient) Resolve(ctx context.Context, domain string, dnsTypes []uint16) ([]string, error) {
	var ret []string

	for _, dnsType := range dnsTypes {
		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn(domain), dnsType)

		resp, err := d.doGetRequest(ctx, msg)
		if err != nil {
			return nil, err
		}

		if resp.Rcode != dns.RcodeSuccess {
			continue
		}

		for _, answer := range resp.Answer {
			if t, ok := answer.(*dns.A); ok {
				ret = append(ret, t.A.String())
			}
			if t, ok := answer.(*dns.AAAA); ok {
				ret = append(ret, t.AAAA.String())
			}
		}
	}

	return ret, nil
}
