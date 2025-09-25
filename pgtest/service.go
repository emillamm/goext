package pgtest

import (
	"context"
	"fmt"
	"sync"

	"github.com/emillamm/envx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	sm        *SessionManager
	session   *EphemeralSession
	err       error
	env       envx.EnvX
	readyChan chan struct{}
	doneChan  chan struct{}
	doneWG    sync.WaitGroup
}

func NewService(env envx.EnvX) *Service {
	return &Service{
		env:       env,
		readyChan: make(chan struct{}),
		doneChan:  make(chan struct{}),
	}
}

func (s *Service) Start(ctx context.Context) {
	if s.sm != nil {
		panic("Service was already started once and should not be reused")
	}

	s.doneWG.Add(1)

	// create new session manager and ephemeral database
	go func() {
		defer close(s.readyChan)
		sm, err := NewSessionManager(s.env)
		if err != nil {
			s.err = err
			close(s.doneChan)
			return
		}
		s.sm = sm

		session, err := s.sm.NewEphemeralSession()
		if err != nil {
			s.err = err
			close(s.doneChan)
			return
		}
		s.session = session
	}()

	// handle shutdown
	go func() {
		select {
		case <-ctx.Done():
			s.err = fmt.Errorf("context cancelled prematurely: %w", context.Cause(ctx))
		case <-s.doneChan:
			break
		}
		if s.sm != nil {
			s.sm.Close()
		}
		s.session = nil // set to nil as we don't want Env() to return values related to this session anymore
		s.doneWG.Done() // mark service as done
	}()
}

func (s *Service) Stop() {
	select {
	case <-s.doneChan:
		return
	default:
		close(s.doneChan)
	}
}

// Return environment with overwritten values of the new session
func (s *Service) Env() envx.EnvX {
	if s.session != nil {
		return s.session.Params.EnvOverwrite(s.env)
	}
	return s.env
}

func (s *Service) Connect() (*pgxpool.Pool, error) {
	return s.session.Connect()
}

func (s *Service) WaitForReady() {
	<-s.readyChan
}

func (s *Service) WaitForDone() {
	s.doneWG.Wait()
}

func (s *Service) Err() error {
	return s.err
}
