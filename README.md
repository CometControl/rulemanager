# Rule Manager

Rule Manager is a robust, Go-based service designed to manage alerting rules in a centralized and standardized way. Inspired by Prometheus, it allows users to define, create, and validate alerting rules from predefined templates, ensuring consistency and reducing errors in monitoring configurations.

## Features

*   **Template-Based Rule Creation**: Generate complex Prometheus/VictoriaMetrics rules from simplified, user-friendly JSON templates.
*   **Dynamic Template Management**: Create, update, and manage rule templates and their schemas via API without redeploying the service.
*   **Advanced Validation**:
    *   **JSON Schema**: Validates user input against strict schemas.
    *   **Pipeline Validation**: Executes custom validation steps (e.g., checking if a metric exists in the TSDB) before creating a rule.
    *   **Dry-Run**: Test templates and data before saving them.
*   **Multi-Backend Support**:
    *   **Storage**: Supports MongoDB for production and a local file system mode for development.
    *   **Datasources**: Configurable integration with Prometheus, VictoriaMetrics, and Thanos.
*   **VictoriaMetrics Integration**: Exposes generated rules in a `vmalert`-compatible YAML format via a dedicated endpoint.

## Core Concepts: Schema & Templates

The power of Rule Manager lies in its separation of **Validation** (Schema) and **Generation** (Template). This approach ensures that users provide valid, structured data, which is then safely transformed into complex alerting rules.

### 1. The Schema (Validation)
The **Schema** defines the "contract" for the rule. It is a standard [JSON Schema](https://json-schema.org/) that dictates what parameters a user must provide to create a rule.

*   **Purpose**: To validate user input *before* any rule generation happens.
*   **Capabilities**:
    *   Define required fields (e.g., `service`, `threshold`).
    *   Enforce data types (e.g., `threshold` must be a number).
    *   Set constraints (e.g., `severity` must be one of `critical`, `warning`, `info`).
    *   **Pipelines**: Advanced validation logic (e.g., "check if metric X exists in Prometheus") can be embedded directly in the schema metadata.

**Example Schema Snippet (from `openshift.json`):**
```json
{
  "$schema": "http://json-schema.org/draft-07/schema",
  "title": "Openshift Monitoring Rule",
  "type": "object",
  "properties": {
    "target": {
      "type": "object",
      "properties": {
        "environment": {
          "type": "string",
          "description": "The deployment environment (e.g., production, staging)"
        },
        "namespace": {
          "type": "string",
          "description": "The Openshift namespace"
        },
        "workload": {
          "type": "string",
          "description": "The workload name (e.g., deployment name)"
        }
      },
      "required": ["environment", "namespace", "workload"],
      "description": "The target entity for the rules"
    },
    "rule": {
      "type": "object",
      "properties": {
        "rule_type": {
          "type": "string",
          "enum": ["cpu", "ram"],
          "description": "The type of resource to monitor"
        },
        "severity": {
          "type": "string",
          "enum": ["critical", "warning", "info"],
          "description": "The severity of the alert"
        },
        "operator": {
          "type": "string",
          "enum": [">", "<", ">=", "<=", "==", "!="],
          "description": "The comparison operator"
        },
        "threshold": {
          "type": "number",
          "description": "The threshold value for the alert"
        },
        "labels": {
          "type": "object",
          "additionalProperties": {
            "type": "string"
          },
          "description": "Additional labels to add to the alert"
        },
        "annotations": {
          "type": "object",
          "additionalProperties": {
            "type": "string"
          },
          "description": "Additional annotations to add to the alert"
        }
      },
      "required": ["rule_type", "severity", "operator", "threshold"],
      "description": "The rule to apply to the target"
    }
  },
  "required": ["target", "rule"],
  "datasource": {
    "type": "prometheus",
    "url": "http://localhost:9090"
  },
  "pipelines": [
    {
      "name": "validate_namespace_metrics",
      "type": "validate_metric_exists",
      "parameters": {
        "metric_name": "kube_namespace_status_phase",
        "labels": "{namespace=\"{{ .target.namespace }}\"}"
      }
    }
  ]
}
```

### 2. The Template (Generation)
The **Template** defines how the validated parameters are transformed into the final rule format (typically Prometheus/vmalert YAML). It uses Go's powerful `text/template` engine.

*   **Purpose**: To abstract the complexity of PromQL and YAML structure from the end-user.
*   **Capabilities**:
    *   Inject parameters into PromQL expressions (e.g., `up{job="{{.service}}"} == 0`).
    *   Logic and Control Flow: Use `if/else` and loops to generate dynamic rules based on input.
    *   Formatting: Ensure the output is always valid, properly indented YAML.

**Example Template Snippet (from `openshift.tmpl`):**
```yaml
{{- $target := .target -}}
{{- $rule := .rule -}}
{{ if eq $rule.rule_type "cpu" }}
- alert: HighCPUUsage_{{ $target.workload }}
  expr: sum(rate(container_cpu_usage_seconds_total{namespace="{{ $target.namespace }}", pod=~"{{ $target.workload }}-.*"}[5m])) by (pod) {{ $rule.operator }} {{ $rule.threshold }}
  for: 5m
  labels:
    severity: {{ $rule.severity }}
    environment: {{ $target.environment }}
    namespace: {{ $target.namespace }}
    {{- range $key, $value := $rule.labels }}
    {{ $key }}: {{ $value }}
    {{- end }}
  annotations:
    summary: "High CPU usage for {{ $target.workload }}"
    {{- range $key, $value := $rule.annotations }}
    {{ $key }}: {{ $value }}
    {{- end }}
{{ else if eq $rule.rule_type "ram" }}
- alert: HighMemoryUsage_{{ $target.workload }}
  expr: sum(container_memory_working_set_bytes{namespace="{{ $target.namespace }}", pod=~"{{ $target.workload }}-.*"}) by (pod) {{ $rule.operator }} {{ $rule.threshold }}
  for: 5m
  labels:
    severity: {{ $rule.severity }}
    environment: {{ $target.environment }}
    namespace: {{ $target.namespace }}
    {{- range $key, $value := $rule.labels }}
    {{ $key }}: {{ $value }}
    {{- end }}
  annotations:
    summary: "High Memory usage for {{ $target.workload }}"
    {{- range $key, $value := $rule.annotations }}
    {{ $key }}: {{ $value }}
    {{- end }}
{{ end }}
```

### 3. The Workflow
1.  **User Request**: The user sends a JSON payload with `templateName` and `parameters`.
2.  **Validation**: The service looks up the **Schema** for that template and validates the `parameters`. If invalid, the request is rejected immediately.
3.  **Generation**: If valid, the `parameters` are passed to the **Template** engine.
4.  **Result**: The template renders the final rule (e.g., a vmalert YAML block), which is then stored and served to the monitoring system.

## Getting Started

### Prerequisites

*   **Go**: Version 1.21 or higher.
*   **MongoDB**: (Optional) For production storage. The service can run in "Local Mode" using the filesystem.

### Installation

1.  Clone the repository:
    ```bash
    git clone https://github.com/your-org/rulemanager.git
    cd rulemanager
    ```

2.  Build the application:
    ```bash
    go build -o rulemanager ./cmd/rulemanager
    ```

### Configuration

The application is configured via a `config.yaml` file. An example configuration is provided in `config/config.yaml`.

Key configuration sections:
*   `server`: Port and host settings.
*   `database`: MongoDB connection details (if used).
*   `template_storage`: Choose between `local` (filesystem), `mongodb`, or `s3`.

### Running the Application

To run the application in Local Mode (using the `./data` directory for storage):

```bash
./rulemanager
```

Ensure your `config.yaml` is set up correctly or pass configuration via environment variables.

## Usage

### Creating a Rule

You can create a new rule by sending a POST request to the API.

```bash
curl -X POST http://localhost:8080/api/v1/rules \
  -H "Content-Type: application/json" \
  -d '{
    "templateName": "openshift",
    "parameters": {
      "target": {
        "environment": "production",
        "namespace": "payment-service",
        "workload": "payment-api"
      },
      "rules": [
        {
          "rule_type": "cpu",
          "severity": "critical",
          "operator": ">",
          "threshold": 0.9
        }
      ]
    }
  }'
```

### Retrieving Rules for vmalert

Configure your `vmalert` to poll this endpoint:

```bash
curl http://localhost:8080/api/v1/rules/vmalert
```

## Project Structure

```
rulemanager/
├── api/                        # HTTP API handlers
│   ├── router.go              # API routes and OpenAPI documentation
│   ├── rule_handlers.go       # CRUD endpoints for rules
│   ├── template_handlers.go   # CRUD endpoints for templates/schemas
│   └── vmalert_handler.go     # vmalert-compatible rules endpoint
├── cmd/
│   └── rulemanager/           # Application entry point
│       └── main.go
├── config/                     # Configuration loading and structures
├── internal/
│   ├── database/              # Storage layer
│   │   ├── store.go          # Interfaces (RuleStore, TemplateProvider)
│   │   ├── mongo_store.go    # MongoDB implementation
│   │   ├── file_store.go     # File-based storage for local mode
│   │   └── caching_store.go  # Template caching wrapper
│   ├── rules/                 # Business logic
│   │   ├── service.go        # Template rendering, validation
│   │   ├── seeder.go         # Default template loading at startup
│   │   └── pipelines.go      # Custom validation steps
│   └── validation/            # JSON Schema validation
├── templates/                  # Default templates (seeded on startup)
│   ├── _base/                 # JSON Schemas
│   │   ├── demo.json
│   │   └── openshift.json
│   └── go_templates/          # Go templates
│       ├── demo.tmpl
│       └── openshift.tmpl
├── docs/                       # Documentation
├── config.yaml                 # Configuration file
└── go.mod
```

### Key Directories

*   **`api/`**: All HTTP handlers using the Huma framework. Each handler validates input, calls services, and returns structured responses.
*   **`internal/database/`**: Abstract storage interfaces with implementations for MongoDB and local filesystem.
*   **`internal/rules/`**: Core business logic for template rendering, rule validation, and pipeline execution.
*   **`internal/validation/`**: JSON Schema validation using the `xeipuuv/gojsonschema` library.
*   **`templates/`**: Default templates that are seeded into the database on first startup.

## API Reference

The API uses standard REST conventions and returns JSON responses.

### Rules Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/rules` | Create one or more rules from a template |
| `GET` | `/api/v1/rules` | List all rules (supports pagination) |
| `GET` | `/api/v1/rules/{id}` | Get a specific rule by ID |
| `PUT` | `/api/v1/rules/{id}` | Update a rule (supports partial updates) |
| `DELETE` | `/api/v1/rules/{id}` | Delete a rule |
| `GET` | `/api/v1/rules/search` | Search rules by template and parameters |

### Templates Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/templates/schemas/{name}` | Create or update a schema |
| `GET` | `/api/v1/templates/schemas/{name}` | Get a schema by name |
| `DELETE` | `/api/v1/templates/schemas/{name}` | Delete a schema |
| `POST` | `/api/v1/templates/go-templates/{name}` | Create or update a Go template |
| `GET` | `/api/v1/templates/go-templates/{name}` | Get a Go template by name |
| `DELETE` | `/api/v1/templates/go-templates/{name}` | Delete a Go template |
| `POST` | `/api/v1/templates/test` | Test a template with parameters (dry-run) |

### vmalert Integration

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/rules/vmalert` | Get all rules in vmalert-compatible YAML format |

### Documentation

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/docs` | Interactive OpenAPI documentation (Swagger UI) |
| `GET` | `/openapi.json` | OpenAPI specification |

**Example: Creating Multiple Rules**

```bash
curl -X POST http://localhost:8080/api/v1/rules \
  -H "Content-Type: application/json" \
  -d '{
    "templateName": "openshift",
    "parameters": {
      "target": {
        "environment": "production",
        "namespace": "payment-services",
        "workload": "payment-api"
      },
      "rules": [
        {
          "rule_type": "cpu",
          "severity": "warning",
          "operator": ">",
          "threshold": 80
        },
        {
          "rule_type": "ram",
          "severity": "critical",
          "operator": ">",
          "threshold": 2147483648
        }
      ]
    }
  }'
```

This single request creates 2 separate rules (CPU and RAM alerts) for the same workload.

**Example: Searching Rules**

The search endpoint supports explicit filtering by template name and nested parameters.

```bash
# Search by template name
curl "http://localhost:8080/api/v1/rules/search?templateName=openshift"

# Search by nested parameter (using dot notation)
curl "http://localhost:8080/api/v1/rules/search?parameters.target.environment=production"

# Combine filters
curl "http://localhost:8080/api/v1/rules/search?templateName=openshift&parameters.target.service=payment-api"
```

## Architecture

The Rule Manager follows a clean architecture pattern:
*   **API Layer**: Built with [Huma](https://huma.rocks/), providing robust routing and validation.
*   **Service Layer**: Handles business logic, template rendering, and pipeline execution.
*   **Data Layer**: Abstracted storage interfaces allowing for swappable backends (MongoDB, File, S3).

For more detailed information, please refer to the documentation in the `docs/` directory:
*   [User Guide](docs/user_guide.md)
*   [Technical Specification](docs/technical_spec.md)
