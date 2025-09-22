package hatchet

import (
	"context"

	hatchetcli "github.com/hatchet-dev/hatchet/pkg/v1"
	hatchetworker "github.com/hatchet-dev/hatchet/pkg/v1/worker"
)

type Hatchet struct {
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

func (h *Hatchet) Start(ctx context.Context) error {
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
	h.workerCleanupFunction, err = h.hatchetWorker.Start()
	return err
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
