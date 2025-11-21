package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type VMAlertHandler struct {
	*RuleHandlers
}

// RegisterVMAlertEndpoint registers the vmalert configuration endpoint.
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

// GetVMAlertConfig generates and returns the vmalert configuration.
func (h *RuleHandlers) GetVMAlertConfig(ctx context.Context, input *struct{}) (*GetVMAlertConfigOutput, error) {
	rules, err := h.ruleStore.ListRules(ctx, 0, 10000)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	configYAML, err := h.ruleService.GenerateVMAlertConfig(ctx, rules)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &GetVMAlertConfigOutput{Body: []byte(configYAML)}, nil
}
