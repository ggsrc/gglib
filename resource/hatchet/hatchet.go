package hatchet

import (
	"context"
	"errors"

	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
)

type Hatchet struct {
	initialized           bool
	clientOpts            []v0Client.ClientOpt
	workerName            string
	workerOpts            []hatchet.WorkerOption
	hatchetCli            *hatchet.Client
	hatchetWorker        *hatchet.Worker
	workerCleanupFunction func() error
}

func NewHatchet(clientOpt []v0Client.ClientOpt, workerName string, workerOpts ...hatchet.WorkerOption) *Hatchet {
	return &Hatchet{
		clientOpts: clientOpt,
		workerName: workerName,
		workerOpts: workerOpts,
	}
}

func (h *Hatchet) Init(ctx context.Context) error {
	client, err := hatchet.NewClient(h.clientOpts...)
	if err != nil {
		return err
	}
	h.hatchetCli = client

	worker, err := client.NewWorker(h.workerName, h.workerOpts...)
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

func (h *Hatchet) Name() string {
	return "hatchet"
}

func (h *Hatchet) GetHatchetWorker() *hatchet.Worker {
	return h.hatchetWorker
}

func (h *Hatchet) GetHatchetCli() *hatchet.Client {
	return h.hatchetCli
}

func (h *Hatchet) GetWorkerCleanupFunction() func() error {
	return h.workerCleanupFunction
}
