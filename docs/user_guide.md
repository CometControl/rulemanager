# Rule Manager User Guide

## Introduction

The **Rule Manager** is a service designed to simplify the creation and management of alerting rules for monitoring systems like Prometheus and VictoriaMetrics. Instead of writing complex PromQL queries manually, you can use pre-defined **Templates** to generate consistent and error-free rules.

This service provides a REST API that allows you to:
- Create, list, update, and delete alerting rules.
- Validate rule parameters against defined schemas.
- Generate rule files compatible with `vmalert`.

## Getting Started

### API Documentation

The Rule Manager comes with built-in interactive API documentation (Swagger UI).

To access it:
1.  Start the Rule Manager service.
2.  Open your browser and navigate to: `http://localhost:8080/docs` (or the appropriate host if deployed remotely).

This interface allows you to explore all available endpoints, see request/response schemas, and even try out API calls directly from your browser.

## API Guide for UI Developers

If you are building a user interface (UI) for the Rule Manager, here is a guide to the key workflows.

### 1. Fetching Templates

To allow users to create rules, you first need to know what templates are available and what fields they require.

-   **List Templates**: Currently, templates are managed via the backend configuration. You can fetch the schema for a specific template to build a form.
-   **Get Template Schema**: `GET /api/v1/templates/schemas/{templateName}`
    -   **Response**: A JSON Schema object.
    -   **Usage**: Use this schema to dynamically generate a form for the user. Libraries like `react-jsonschema-form` can automatically render forms based on this response.

### 2. Creating and Validating Rules

Once the user fills out the form, you can create one or more rules in a single request.

-   **Validate (Dry-Run)**: `POST /api/v1/templates/validate`
    -   **Body**: `{ "templateName": "...", "parameters": { ... } }`
    -   **Usage**: Call this endpoint before submitting the final rule to check for validation errors (e.g., missing fields, invalid values, or metrics that don't exist in the datasource).
    
-   **Create Rules**: `POST /api/v1/rules`
    -   **Body**: `{ "templateName": "...", "parameters": {"target": {...}, "rules": [{...}, {...}, ...]} }`
    -   **Usage**: Create one or more alert rules for the same target entity in one request. Specify the target once, and provide an array of rules. For a single rule, send an array with one element.
    -   **Response**: `{"ids": ["id1", "id2", ...], "count": N}` - Returns an array of created rule IDs.
    -   **Note**: Each rule in the array will be created as a separate entry, allowing individual management.

**Example:**
```json
{
  "templateName": "k8s",
  "parameters": {
    "target": {
      "environment": "production",
      "namespace": "payment-service",
      "workload": "payment-api"
    },
    "common": {
      "severity": "warning",
      "labels": {
        "team": "payments"
      }
    },
    "rules": [
      {"rule_type": "cpu", "operator": ">", "threshold": 0.7},
      {"rule_type": "cpu", "operator": ">", "threshold": 0.9, "common": {"severity": "critical"}},
      {"rule_type": "ram", "operator": ">", "threshold": 2000000000, "common": {"severity": "critical"}}
    ]
  }
}
```
This creates 3 separate rule entries, each independently manageable.

### 3. Managing Rules

-   **List Rules**: `GET /api/v1/rules`
    -   **Query Params**: `offset` (default 0), `limit` (default 10)
    -   **Response**: Array of rule objects.

-   **Search Rules**: `GET /api/v1/rules/search`
    -   **Description**: Search for rules using explicit filters.
    -   **Query Params**:
        -   `templateName`: Filter by template name (e.g., `?templateName=k8s`).
        -   `parameters.{path}`: Filter by any nested parameter using dot notation (e.g., `?parameters.target.environment=production`).
    -   **Examples**:
        -   `GET /api/v1/rules/search?templateName=k8s`
        -   `GET /api/v1/rules/search?parameters.target.service=payment-api`
        -   `GET /api/v1/rules/search?templateName=k8s&parameters.target.environment=production`
    -   **Response**: Array of matching rule objects.

    
-   **Get Rule**: `GET /api/v1/rules/{id}`
    -   **Response**: Single rule object.
    
-   **Update Rule**: `PUT /api/v1/rules/{id}`
    -   **Body**: `{ "templateName": "...", "parameters": { ... } }`
    -   **Features**: Supports **partial updates**. You only need to send the fields you want to change. New values are merged with existing ones.
    -   **Example (Partial Update)**:
        ```json
        {
          "parameters": {
            "common": {
              "severity": "critical"
            },
            "rules": [
              {
                "threshold": 0.85
              }
            ]
          }
        }
        ```
        This will update `severity` in common and `threshold` in the rule, while preserving other fields. Note that updating the `rules` array via partial update merges by index/key depending on the merge strategy, but for arrays it typically replaces or appends. For precise updates, it's safer to provide the full rule definition for the specific rule being updated.
        
-   **Delete Rule**: `DELETE /api/v1/rules/{id}`
    -   **Response**: 204 No Content.

### 4. Planning Changes (Uniqueness & Overrides)

The Rule Manager enforces **Uniqueness Constraints** to prevent duplicate rules. These constraints are defined in the template schema (e.g., a rule might be unique by `target.namespace` + `rule_type`).

To help you manage these constraints safely, the API provides "Plan" endpoints.

#### Planning Creation
Before creating a rule, you can check if it will conflict with or override an existing rule.

-   **Plan Creation**: `POST /api/v1/rules/plan`
    -   **Body**: Same as `POST /api/v1/rules`
    -   **Response**:
        -   `action`: `"create"` (safe to create) or `"update"` (will override existing rule).
        -   `existing_rule`: Details of the rule that will be overridden (if any).
        -   `reason`: Explanation of the action.

#### Planning Updates
When updating a rule, you might inadvertently change its parameters to values that conflict with *another* existing rule.

-   **Plan Update**: `POST /api/v1/rules/{id}/plan`
    -   **Body**: Same as `PUT /api/v1/rules/{id}`
    -   **Response**:
        -   `action`: `"update"` (safe to update) or `"conflict"` (violates uniqueness).
        -   `reason`: Explanation of the conflict.

**Note**: If you attempt a direct `PUT` that results in a conflict, the API will return a `409 Conflict` error.

### 5. Integration with Monitoring

The Rule Manager exposes an endpoint for the monitoring system (e.g., `vmalert`) to consume.

-   **vmalert Config**: `GET /api/v1/rules/vmalert`
    -   **Response**: A YAML file containing all active rules in standard Prometheus/vmalert format.
    -   **Usage**: This endpoint is typically polled by the monitoring agent, not used directly by the UI.
