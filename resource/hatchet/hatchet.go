// Package hatchet provides a Hatchet resource compatible with ggresource.Resource,
// with optional workflow-builder support so workflows can be built using the
// same client that backs the worker (no duplicate client).
//
// This mirrors github.com/ggsrc/gglib/resource/hatchet but adds
// NewHatchetWithWorkflowBuilder. The same API can be proposed upstream to gglib.
package hatchet

import (
	"context"
	"errors"

	hatchet_sdk "github.com/hatchet-dev/hatchet/sdks/go"
	v0Client "github.com/hatchet-dev/hatchet/pkg/client"
)

// WorkflowBuilderFunc is called in Init() after the Hatchet client is created.
// It returns a WorkerOption (e.g. hatchet_sdk.WithWorkflows(...)) so the worker
// is created with workflows built using that client. This avoids creating the
// client twice (once in the app, once inside Hatchet).
type WorkflowBuilderFunc func(client *hatchet_sdk.Client) hatchet_sdk.WorkerOption

type Hatchet struct {
	initialized           bool
	clientOpts            []v0Client.ClientOpt
	workerName            string
	workerOpts            []hatchet_sdk.WorkerOption
	workflowBuilder       WorkflowBuilderFunc
	hatchetCli            *hatchet_sdk.Client
	hatchetWorker         *hatchet_sdk.Worker
	workerCleanupFunction func() error
}

// NewHatchet creates a Hatchet resource with the same behavior as gglib's
// resource/hatchet (client and worker created in Init from opts only).
func NewHatchet(clientOpt []v0Client.ClientOpt, workerName string, workflowBuilder WorkflowBuilderFunc, workerOpts ...hatchet_sdk.WorkerOption) *Hatchet {
	return &Hatchet{
		clientOpts:  clientOpt,
		workerName:  workerName,
		workerOpts:  workerOpts,
		workflowBuilder: workflowBuilder,
	}
}

func (h *Hatchet) Init(ctx context.Context) error {
	client, err := hatchet_sdk.NewClient(h.clientOpts...)
	if err != nil {
		return err
	}
	h.hatchetCli = client

	opts := h.workerOpts
	if h.workflowBuilder != nil {
		opts = append(opts, h.workflowBuilder(client))
	}

	worker, err := client.NewWorker(h.workerName, opts...)
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

func (h *Hatchet) GetHatchetWorker() *hatchet_sdk.Worker {
	return h.hatchetWorker
}

func (h *Hatchet) GetHatchetCli() *hatchet_sdk.Client {
	return h.hatchetCli
}

func (h *Hatchet) GetWorkerCleanupFunction() func() error {
	return h.workerCleanupFunction
}
