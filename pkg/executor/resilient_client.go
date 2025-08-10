package executor

import(
		"github.com/sony/gobreaker"
		"golang.org/x/crypto/ssh"
		"time"
		"fmt"
		"github.com/cenkalti/backoff/v4"
)

type SSHClient interface {
	NewSession() (*ssh.Session, error)
	Close() error
	RemoteAddr() string
}

type ResilienceConfig struct {
	BackoffSettings  		*backoff.ExponentialBackOff
	CircuitBreakerSettings 	 gobreaker.Settings
	CircuitBreaker			*gobreaker.CircuitBreaker
}

type ResilientSSHClient struct {
    SSHClient  	*ssh.Client
	ResConf 	*ResilienceConfig
}

func NewResilienceConfig(defaultBackOff *backoff.ExponentialBackOff, cbs gobreaker.Settings, cb *gobreaker.CircuitBreaker)(*ResilienceConfig){
	return &ResilienceConfig{
			BackoffSettings: 		defaultBackOff,
			CircuitBreakerSettings: cbs,
			CircuitBreaker: 		cb,
	}
}

func (r *ResilienceConfig) Configure(backoffSettings *backoff.ExponentialBackOff, cbSettings gobreaker.Settings) {
    r.BackoffSettings = backoffSettings
    r.CircuitBreakerSettings = cbSettings
}

func (c *ResilientSSHClient) Close() error {
    return c.SSHClient.Close()
}

func NewResilientClient(remote string,  config *ssh.ClientConfig) (*ResilientSSHClient, error) {
	client, err := ssh.Dial("tcp", remote, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial  %w", err)
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
	resConfig := NewResilienceConfig(
		&backoff.ExponentialBackOff{
			InitialInterval:     500 * time.Millisecond,
			MaxInterval:         5 * time.Second,
			Multiplier:          1.5,
			RandomizationFactor: 0.5,
			Stop:                backoff.Stop,
			Clock:               backoff.SystemClock,
		},
		cbs,
		gobreaker.NewCircuitBreaker(cbs),
	)

    return &ResilientSSHClient{
        SSHClient: client,
 		ResConf: resConfig,
    }, nil
}

