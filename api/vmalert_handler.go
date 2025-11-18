package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type VMAlertHandler struct {
	*RuleHandlers
}

func (h *RuleHandlers) RegisterVMAlertEndpoint(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-vmalert-config",
		Method:      http.MethodGet,
		Path:        "/api/v1/rules/vmalert",
		Summary:     "Get vmalert configuration",
		Description: "Generates the YAML configuration for vmalert.",
		Tags:        []string{"Integration"},
	}, h.GetVMAlertConfig)
}

type GetVMAlertConfigOutput struct {
	Body []byte `contentType:"application/x-yaml"`
}

func (h *RuleHandlers) GetVMAlertConfig(ctx context.Context, input *struct{}) (*GetVMAlertConfigOutput, error) {
	// 1. List all rules
	// In a real scenario, we might want to paginate or stream, but for now fetch all.
	// We need a ListAllRules method in store or use a large limit.
	// Let's assume 10000 is enough for now.
	rules, err := h.ruleStore.ListRules(ctx, 0, 10000)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	// 2. Generate Config
	configYAML, err := h.ruleService.GenerateVMAlertConfig(ctx, rules)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &GetVMAlertConfigOutput{Body: []byte(configYAML)}, nil
}
