package scaffold

import (
	"context"
	"errors"
	"fmt"
	"sync"

	scaffoldcontainer "github.com/hlfshell/scaffold/container"
)

/*
Create creates each service group in the order it was added. Services in
the same group are created in parallel. If a service fails, services
that were already created are cleaned up.
*/
func (s *Stack) Create(ctx context.Context) error {
	err := s.validate()
	if err != nil {
		return err
	}
	if s.created || s.creating {
		return fmt.Errorf("stack %s has already been created or is creating", s.name)
	}

	s.creating = true
	defer func() {
		s.creating = false
	}()

	s.ensureRunID()
	s.applyNamePrefix()
	s.applyLabels()

	if s.sharedNetwork {
		if s.namePrefix != "" {
			s.networkName = s.generatedNamePrefix()
		}
		created, err := scaffoldcontainer.CreateNetwork(ctx, s.networkName, s.stackLabels())
		if err != nil {
			return err
		}
		s.networkCreated = created

		for _, service := range s.services {
			attachable, ok := service.(NetworkAttachable)
			if ok {
				attachable.SetNetwork(s.networkName)
			}
		}
	}

	for _, group := range s.serviceGroups {
		created, err := s.createGroup(ctx, group)
		if len(created) > 0 {
			s.createdGroups = append(s.createdGroups, created)
		}
		if err != nil {
			cleanupErr := s.cleanupCreated(context.WithoutCancel(ctx))
			if cleanupErr != nil {
				return fmt.Errorf("failed to create service group: %w; cleanup failed: %v", err, cleanupErr)
			}
			return fmt.Errorf("failed to create service group: %w", err)
		}
	}

	s.created = true
	return nil
}

/*
Cleanup handles service groups in reverse creation order. Services that
were created in the same group are cleaned up in parallel.
*/
func (s *Stack) Cleanup(ctx context.Context) error {
	if s.cleaning {
		return fmt.Errorf("stack %s is already cleaning", s.name)
	}
	s.cleaning = true
	defer func() {
		s.cleaning = false
	}()

	var firstErr error

	for i := len(s.createdGroups) - 1; i >= 0; i-- {
		err := s.cleanupGroup(ctx, s.createdGroups[i])
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	s.createdGroups = [][]Service{}
	s.created = false

	if s.sharedNetwork && s.networkCreated {
		err := scaffoldcontainer.RemoveNetwork(ctx, s.networkName)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.networkCreated = false

	return firstErr
}

func (s *Stack) createGroup(ctx context.Context, group []Service) ([]Service, error) {
	var waitGroup sync.WaitGroup
	errs := make([]error, len(group))
	created := make([]bool, len(group))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, service := range group {
		waitGroup.Add(1)

		go func(i int, service Service) {
			defer waitGroup.Done()

			err := createService(ctx, service)
			if err != nil {
				errs[i] = fmt.Errorf("failed to create service %s: %w", service.Name(), err)
				cancel()
				return
			}

			created[i] = true
		}(i, service)
	}

	waitGroup.Wait()

	err := errors.Join(errs...)
	if err != nil {
		// If any service in the group failed, cleanup is attempted for every
		// service whose Create was invoked. Service cleanup should be
		// idempotent.
		return group, err
	}

	createdServices := []Service{}
	for i, service := range group {
		if created[i] {
			createdServices = append(createdServices, service)
		}
	}

	return createdServices, nil
}

func (s *Stack) cleanupGroup(ctx context.Context, group []Service) error {
	var waitGroup sync.WaitGroup
	errs := make([]error, len(group))

	for i, service := range group {
		waitGroup.Add(1)

		go func(i int, service Service) {
			defer waitGroup.Done()

			err := cleanupService(ctx, service)
			if err != nil {
				errs[i] = fmt.Errorf("failed to cleanup service %s: %w", service.Name(), err)
			}
		}(i, service)
	}

	waitGroup.Wait()

	return errors.Join(errs...)
}

func (s *Stack) cleanupCreated(ctx context.Context) error {
	cleanupErr := s.Cleanup(ctx)
	return cleanupErr
}

func createService(ctx context.Context, service Service) error {
	return service.Create(ctx)
}

func cleanupService(ctx context.Context, service Service) error {
	return service.Cleanup(ctx)
}

func (s *Stack) validate() error {
	if s.name == "" {
		return fmt.Errorf("stack name cannot be empty")
	}

	seen := map[string]struct{}{}
	for _, service := range s.services {
		if service == nil {
			return fmt.Errorf("stack %s contains nil service", s.name)
		}
		if service.Name() == "" {
			return fmt.Errorf("stack %s contains service with empty name", s.name)
		}
		if _, ok := seen[service.Name()]; ok {
			return fmt.Errorf("stack %s contains duplicate service name %s", s.name, service.Name())
		}
		seen[service.Name()] = struct{}{}
	}

	return nil
}
