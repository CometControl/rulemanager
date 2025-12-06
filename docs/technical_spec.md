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
1.  **JSON Schema**: Defines the input structure, validation rules, pipeline steps, and **uniqueness keys**.
2.  **Go Template**: Defines the output structure (Prometheus rule YAML).

## 3. API Specification

### 3.1 Rules

*   `POST /api/v1/rules`: Create a new rule.
    *   Body: `{ "templateName": "string", "parameters": { ... } }`
*   `POST /api/v1/rules/plan`: Plan rule creation.
    *   Body: Same as Create.
    *   Returns: Action (create/update) and diff/reason.
*   `GET /api/v1/rules`: List rules (pagination supported).
*   `GET /api/v1/rules/search`: Search rules by template and parameters.
*   `GET /api/v1/rules/{id}`: Get a specific rule.
*   `PUT /api/v1/rules/{id}`: Update a rule.
*   `POST /api/v1/rules/{id}/plan`: Plan rule update.
    *   Body: Same as Update.
    *   Returns: Action (update/conflict) and reason.
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

### 4.2 Uniqueness & Conflict Resolution
Uniqueness is enforced dynamically based on the `uniqueness_keys` defined in the Template Schema.
*   **Definition**: A list of dot-notation paths (e.g., `["target.namespace", "rules.rule_type"]`).
*   **Fallback**: If undefined, defaults to `["target", "rules.rule_type"]`.
*   **Creation Logic**:
    *   If a rule with matching keys exists: **Override** (Update) the existing rule.
*   **Update Logic**:
    *   If the updated parameters conflict with *another* rule (excluding self): **Reject** with `409 Conflict`.

### 4.3 Caching Strategy
*   **Templates**: Cached in-memory to reduce storage I/O. Refreshed on update.
*   **vmalert Output**: The generated YAML for `vmalert` is cached and invalidated only when a rule is created, updated, or deleted.

## 5. Integration

## 6. Infrastructure

### 6.1 Development Environment
The development environment is containerized using Docker Compose to ensure consistency across different setups (Linux, WSL2).
*   **Database**: MongoDB Community Edition running in a Docker container.
*   **Persistence**: Data is persisted locally via bind-mounts to `./data/mongo` to allow for easy inspection and persistence between restarts.
*   **Orchestration**: Managed via `Makefile` aliases (`make docker-up`, `make run`).

### 6.2 Deployment
The application is designed to be deployed as a stateless microservice, with the database being the only stateful component.

