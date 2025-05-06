package main

import(
		"github.com/sony/gobreaker"
		"golang.org/x/crypto/ssh"
		"time"
		"fmt"
		"context"
		"github.com/cenkalti/backoff/v4"
)


type ResilientSSHClient struct {
    sshclient  *ssh.Client
    cb         *gobreaker.CircuitBreaker
    backoff    backoff.BackOff
}

func NewResilientClient(ctx context.Context,remote, login, password string) (*ResilientSSHClient, error) {
	config := &ssh.ClientConfig{
		User: login,
		//Auth: []ssh.AuthMethod{ssh.Password(password)},
		Auth:            []ssh.AuthMethod{publicKeyAuth("/home/andrey/.ssh/myadminvps.ru")}, // TODO: move to main
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
		BannerCallback:  func(message string) error { return nil }, //ignore banner
	}

	client, err := ssh.Dial("tcp", remote, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial  %w", err)
	}

    defaultBackoff := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		MaxInterval:         5 * time.Second,
		Multiplier:          1.5,
		RandomizationFactor: 0.5,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}

	cbs := gobreaker.Settings{
		Name:        "ssh-connection",
		MaxRequests: 5,         
		Interval:    1 * time.Minute,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	}

    return &ResilientSSHClient{
        sshclient: client,
        cb: gobreaker.NewCircuitBreaker(cbs),
        backoff: backoff.WithContext(defaultBackoff, ctx),
    }, nil
}

func newSSHSession(client *ssh.Client,cb *gobreaker.CircuitBreaker) (*ssh.Session, error) {
	res, err := cb.Execute(func() (any, error) {
        return client.NewSession()
    })
    if err != nil {
        return nil, err
    }
    return res.(*ssh.Session), nil
}