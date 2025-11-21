# Technical Specification

## 1. System Architecture

The Rule Manager is designed as a modular microservice with a clear separation of concerns.

### 1.1 Components

*   **API Layer (Huma/Chi)**: Handles HTTP requests, routing, and initial validation.
*   **Service Layer**:
    *   **Rule Service**: Orchestrates rule creation, retrieval, and deletion.
    *   **Template Service**: Manages template storage and retrieval.
    *   **Pipeline Processor**: Executes validation steps defined in schemas.
*   **Data Access Layer (DAL)**:
    *   **RuleStore**: Interface for rule persistence (MongoDB, File).
    *   **TemplateProvider**: Interface for template retrieval (Local, MongoDB, S3).
*   **Integration Layer**:
    *   **External Validator**: Client for communicating with external validation services.
    *   **TSDB Client**: Client for querying Prometheus/VictoriaMetrics for validation.

## 2. Data Model

### 2.1 Rule
Stored in the database (e.g., MongoDB collection `rules`).

```go
type Rule struct {
    ID           string          `json:"id" bson:"_id,omitempty"`
    TemplateName string          `json:"templateName" bson:"templateName"`
    Parameters   json.RawMessage `json:"parameters" bson:"parameters"` // User inputs
    CreatedAt    time.Time       `json:"createdAt" bson:"createdAt"`
    UpdatedAt    time.Time       `json:"updatedAt" bson:"updatedAt"`
}
```

### 2.2 Template
Templates consist of two parts:
1.  **JSON Schema**: Defines the input structure, validation rules, and pipeline steps.
2.  **Go Template**: Defines the output structure (Prometheus rule YAML).

## 3. API Specification

### 3.1 Rules

*   `POST /api/v1/rules`: Create a new rule.
    *   Body: `{ "templateName": "string", "parameters": { ... } }`
*   `GET /api/v1/rules`: List rules (pagination supported).
*   `GET /api/v1/rules/{id}`: Get a specific rule.
*   `PUT /api/v1/rules/{id}`: Update a rule.
*   `DELETE /api/v1/rules/{id}`: Delete a rule.
*   `GET /api/v1/rules/vmalert`: Get all rules in `vmalert` YAML format.

### 3.2 Templates

*   `GET /api/v1/templates/schemas/{name}`: Get a template's JSON schema.
*   `POST /api/v1/templates/validate`: Dry-run validation of a template.

## 4. Component Details

### 4.1 Pipeline Processor
The Pipeline Processor allows for dynamic, declarative validation logic.
*   **Trigger**: Runs before rule creation/update.
*   **Definition**: Defined in the `pipelines` array of the JSON Schema.
*   **Step Runners**:
    *   `validate_metric_exists`: Queries the configured datasource to ensure the metric exists.

### 4.2 Caching Strategy
*   **Templates**: Cached in-memory to reduce storage I/O. Refreshed on update.
*   **vmalert Output**: The generated YAML for `vmalert` is cached and invalidated only when a rule is created, updated, or deleted.

## 5. Integration

### 5.1 VictoriaMetrics
The service generates a YAML file compatible with `vmalert`.
*   **Format**: Standard Prometheus rule groups.
*   **Grouping**: Rules are grouped by "monitoring world" (derived from template name).
