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

Once the user fills out the form, you can create a rule.

-   **Validate (Dry-Run)**: `POST /api/v1/templates/validate`
    -   **Body**: `{ "templateName": "...", "parameters": { ... } }`
    -   **Usage**: Call this endpoint before submitting the final rule to check for validation errors (e.g., missing fields, invalid values, or metrics that don't exist in the datasource).
-   **Create Rule**: `POST /api/v1/rules`
    -   **Body**: `{ "templateName": "...", "parameters": { ... } }`
    -   **Response**: The created rule object, including its unique `id`.

### 3. Managing Rules

-   **List Rules**: `GET /api/v1/rules`
    -   **Usage**: Display a table of existing rules. Supports pagination.
-   **Get Rule**: `GET /api/v1/rules/{id}`
    -   **Usage**: Fetch details for editing a specific rule.
-   **Update Rule**: `PUT /api/v1/rules/{id}`
    -   **Body**: `{ "templateName": "...", "parameters": { ... } }`
-   **Delete Rule**: `DELETE /api/v1/rules/{id}`

### 4. Integration with Monitoring

The Rule Manager exposes an endpoint for the monitoring system (e.g., `vmalert`) to consume.

-   **vmalert Config**: `GET /api/v1/rules/vmalert`
    -   **Response**: A YAML file containing all active rules in standard Prometheus/vmalert format.
    -   **Usage**: This endpoint is typically polled by the monitoring agent, not used directly by the UI.
