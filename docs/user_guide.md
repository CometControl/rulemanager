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
  "templateName": "openshift",
  "parameters": {
    "target": {
      "environment": "production",
      "namespace": "payment-service",
      "workload": "payment-api"
    },
    "rules": [
      {"rule_type": "cpu", "severity": "warning", "operator": ">", "threshold": 0.7},
      {"rule_type": "cpu", "severity": "critical", "operator": ">", "threshold": 0.9},
      {"rule_type": "ram", "severity": "critical", "operator": ">", "threshold": 2000000000}
    ]
  }
}
```
This creates 3 separate rule entries, each independently manageable.

### 3. Managing Rules

-   **List Rules**: `GET /api/v1/rules`
    -   **Query Params**: `offset` (default 0), `limit` (default 10)
    -   **Response**: Array of rule objects.
    
-   **Get Rule**: `GET /api/v1/rules/{id}`
    -   **Response**: Single rule object.
    
-   **Update Rule**: `PUT /api/v1/rules/{id}`
    -   **Body**: `{ "templateName": "...", "parameters": { ... } }`
    -   **Features**: Supports **partial updates**. You only need to send the fields you want to change. New values are merged with existing ones.
    -   **Example (Partial Update)**:
        ```json
        {
          "parameters": {
            "rule": {
              "threshold": 0.8,
              "severity": "critical"
            }
          }
        }
        ```
        This will update `threshold` and `severity` while preserving other fields (like `target` or other `rule` properties).
        
-   **Delete Rule**: `DELETE /api/v1/rules/{id}`
    -   **Response**: 204 No Content.

### 4. Integration with Monitoring

The Rule Manager exposes an endpoint for the monitoring system (e.g., `vmalert`) to consume.

-   **vmalert Config**: `GET /api/v1/rules/vmalert`
    -   **Response**: A YAML file containing all active rules in standard Prometheus/vmalert format.
    -   **Usage**: This endpoint is typically polled by the monitoring agent, not used directly by the UI.
