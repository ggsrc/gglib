package hatchet

import (
	"context"
	"errors"

	hatchetcli "github.com/hatchet-dev/hatchet/pkg/v1"
	hatchetworker "github.com/hatchet-dev/hatchet/pkg/v1/worker"
)

type Hatchet struct {
	initialized           bool
	clientOpts            []hatchetcli.Config
	workerOpt             hatchetworker.WorkerOpts
	hatchetCli            hatchetcli.HatchetClient
	hatchetWorker         hatchetworker.Worker
	workerCleanupFunction func() error
}

func NewHatchet(clientOpt []hatchetcli.Config, workerOpt hatchetworker.WorkerOpts) *Hatchet {
	return &Hatchet{
		clientOpts: clientOpt,
		workerOpt:  workerOpt,
	}
}

func (h *Hatchet) Init(ctx context.Context) error {
	client, err := hatchetcli.NewHatchetClient(h.clientOpts...)
	if err != nil {
		return err
	}
	h.hatchetCli = client

	worker, err := client.Worker(h.workerOpt)
	if err != nil {
		return err
	}
	h.hatchetWorker = worker
	h.initialized = true
	return nil
}

func (h *Hatchet) Start(ctx context.Context) error {
	if !h.initialized {
		return errors.New("hatchet not initialized")
	}
	var err error
	h.workerCleanupFunction, err = h.hatchetWorker.Start()
	if err != nil {
		return err
	}
	return nil
}

func (h *Hatchet) Stop(ctx context.Context) error {
	return h.workerCleanupFunction()
}

func (h *Hatchet) OK(ctx context.Context) error {
	return nil
}

func (h *Hatchet) GetHatchetWorker() hatchetworker.Worker {
	return h.hatchetWorker
}

func (h *Hatchet) GetHatchetCli() hatchetcli.HatchetClient {
	return h.hatchetCli
}

func (h *Hatchet) GetWorkerCleanupFunction() func() error {
	return h.workerCleanupFunction
}
