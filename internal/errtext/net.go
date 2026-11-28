package errtext

import (
	"errors"
	"net"
	"net/url"
	"syscall"
)

const (
	unreachable = "Cannot reach the server. Check your connection."
	noServer    = "Cannot find the server. Check your connection."
	timedOut    = "The connection timed out. Check your connection and try again."
)

func IsNetwork(err error) bool {
	var ue *url.Error
	var op *net.OpError
	var dns *net.DNSError
	return errors.As(err, &ue) || errors.As(err, &op) || errors.As(err, &dns) ||
		errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.EHOSTUNREACH)
}

func network(err error) (string, bool) {
	if !IsNetwork(err) {
		return "", false
	}
	var dns *net.DNSError
	var ne net.Error
	switch {
	case errors.As(err, &dns):
		return noServer, true
	case errors.As(err, &ne) && ne.Timeout():
		return timedOut, true
	}
	return unreachable, true
}
