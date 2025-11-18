# Technical Development Details

## Introduction

This document outlines the technical details and implementation plan for the Rule Manager project. It is intended to be a living document that will be updated as the project evolves.

## Proposed Project Structure

This revised structure is more idiomatic for a growing Go project and more accurately reflects the detailed architecture designed in this document.

```
/rulemanager
|-- /api
|   |-- router.go            // Huma router setup and registration of all handlers
|   |-- rule_handlers.go     // HTTP handlers for Rule CRUD
|   |-- template_handlers.go // HTTP handlers for Template CRUD & validation
|-- /cmd
|   |-- /rulemanager
|       |-- main.go          // Main application entry point
|-- /config
|   |-- config.go            // Configuration struct and loading (Viper)
|   |-- config.yaml          // Example configuration file
|-- /internal
|   |-- /database
|   |   |-- store.go            // Defines the RuleStore and TemplateProvider interfaces
|   |   |-- mongo_store.go      // MongoDB implementation
|   |   |-- file_store.go       // Local file system implementation
|   |   |-- caching_store.go    // In-memory caching wrapper for TemplateProvider
|   |-- /rules
|   |   |-- service.go          // Core business logic for managing rules
|   |   |-- pipelines.go        // Rule creation pipeline logic (StepRunners)
|   |-- /validation
|   |   |-- schema.go           // JSON Schema validation logic
|-- /templates               // Default templates if using 'local' provider
|   |-- /_base
|   |   |-- openshift.json
|   |-- /go_templates
|   |   |-- openshift.tmpl
|   |-- openshift.json
|-- go.mod
|-- go.sum
|-- README.md
|-- DEVELOPMENT.md
```

**Justification for Changes:**

*   **/api:** Handlers are split by resource (`rule_handlers.go`, `template_handlers.go`) for better organization.
*   **/config:** An example `config.yaml` is included to make the structure more concrete.
*   **/internal/database:** Files are renamed to be more descriptive (`store.go` for interfaces, `mongo_store.go` for the implementation) and show where other implementations would live.
*   **/internal/validation:** The directory is renamed from `validator` and split into multiple files to cleanly separate the different validation responsibilities (JSON Schema, PromQL, external calls).

## Implementation Details

### Configuration

Application configuration (e.g., database connection string, external service URL) will be managed through a configuration file (e.g., `config.yaml`) and loaded at startup. A `config` package will provide access to the configuration values.

### API Layer

The API will be built using the [Huma](https://huma.rocks/) framework, which is built on top of the `chi` router. This choice provides the following advantages:

*   **Automatic OpenAPI Generation:** Huma can generate an OpenAPI specification from the code, which is crucial for the goal of sharing the available rules and their parameters.
*   **Built-in Validation:** Huma provides request and response validation, which can be used to validate rule creation requests.
*   **Performance:** `chi` is a lightweight and high-performance router.

### Service Layer

The service layer will contain the core business logic of the application. This includes:

*   Managing rule templates.
*   Creating and validating new rules.
*   Interacting with the database layer and the external validator.

### Data Access Layer (DAL)

To ensure flexibility in database choices, a data access layer will be implemented with an interface that defines the required database operations. The initial implementation will be for MongoDB, but other databases can be supported by implementing the same interface.

### Rule Templates

Rule templates will be defined using [JSON Schema](https://json-schema.org/). This provides a standard and powerful way to define the structure of the data required for each rule. The benefits of this approach are:

*   **Standardization:** JSON Schema is a widely adopted standard for describing the structure of JSON data.
*   **Validation:** The schemas can be used to validate the parameters of incoming rule creation requests.
*   **Discoverability:** The schemas can be exposed through the API, allowing clients to dynamically understand the requirements for each rule template.
*   **Database Support:**
    *   **MongoDB:** Uses MongoDB to store rule configurations for production environments.
    *   **Local File Store:** Supports a local file-based storage mode for development and testing without external dependencies.
*   **External Validation:** Connects to external services to validate rule-specific details (e.g., verifying the existence of a Kafka topic and its monitorability).
The templates will be stored as `.json` files in the `/templates` directory and loaded at startup.

### Common Rule Sections and Schema Composition

To promote reusability and maintain consistency, our JSON Schemas will define the *parameters* required to generate a complete Prometheus-compatible alert rule. The Rule Manager service will be responsible for taking these parameters and templating them into a final rule configuration.

This approach allows us to:
*   Abstract away the complexity of PromQL from the end-user.
*   Enforce consistency in labeling and annotations.
*   Create a simplified and user-friendly API for rule creation.

For each "monitoring world" (e.g., OpenShift, Kafka), we can choose to have a single consolidated file for all its related rules, or separate files for each rule. For OpenShift, we will use a consolidated approach.

**1. Base Schemas for Monitoring Worlds:**

We will create base schemas that define the common parameters required for each monitoring world. These will be stored in a `_base` subdirectory within `/templates`.

**Example: OpenShift Base Schema (`/templates/_base/openshift.json`)**

This schema defines the core identifiers for any OpenShift-related alert.

```json
{
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "title": "Openshift Monitoring Rule",
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
        },
        "rule_type": {
            "type": "string",
            "enum": [
                "cpu",
                "ram"
            ],
            "description": "The type of resource to monitor"
        }
    },
    "required": [
        "environment",
        "namespace",
        "workload",
        "rule_type"
    ]
}
```

**2. Consolidated Rule Schema with Conditional Logic:**

We can define all rules for a specific world in a single schema and use conditional logic to change validation rules based on the input. For example, if a user wants to monitor a specific type of asset, the available rule types can be dynamically restricted.

**Example: Conditional Asset Monitoring Schema (`/templates/asset_monitoring.json`)**

This schema demonstrates how to change the available `rule_type` options based on the value of `asset_type`.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Asset Monitoring Rule",
  "description": "A rule for monitoring a specific asset.",
  "type": "object",
  "properties": {
    "asset_type": {
      "description": "The type of the asset being monitored.",
      "type": "string",
      "enum": ["server", "network_switch"]
    },
    "rule_type": {
      "description": "The type of rule to create.",
      "type": "string"
    }
  },
  "required": ["asset_type", "rule_type"],
  "allOf": [
    {
      "if": {
        "properties": { "asset_type": { "const": "server" } }
      },
      "then": {
        "properties": {
          "rule_type": { "enum": ["cpu_usage", "memory_usage"] }
        }
      }
    },
    {
      "if": {
        "properties": { "asset_type": { "const": "network_switch" } }
      },
      "then": {
        "properties": {
          "rule_type": { "enum": ["packet_loss", "latency"] }
        }
      }
    }
  ],
  "oneOf": [
    {
      "if": { "properties": { "rule_type": { "const": "cpu_usage" } } },
      "then": { /* Schema for CPU usage parameters... */ }
    },
    {
      "if": { "properties": { "rule_type": { "const": "memory_usage" } } },
      "then": { /* Schema for Memory usage parameters... */ }
    },
    {
      "if": { "properties": { "rule_type": { "const": "packet_loss" } } },
      "then": { /* Schema for Packet Loss parameters... */ }
    },
    {
      "if": { "properties": { "rule_type": { "const": "latency" } } },
      "then": { /* Schema for Latency parameters... */ }
    }
  ]
}
```

In this example, the `allOf` block is used to conditionally apply `enum` restrictions to the `rule_type` field based on the `asset_type`. The `oneOf` block then defines the specific parameters required for each of those rule types.

**3. Generated Prometheus Rule:**

The Rule Manager will take the user's input, validated against the `openshift.json` schema, and generate a Prometheus rule. The specific rule generated will depend on the `rule_type` provided.

This approach provides a clear separation between the user-friendly API and the underlying Prometheus configuration, while ensuring that the generated rules are valid and consistent.

### Rule Template Engine and Storage

The core of the rule generation logic lies in the templating engine. While the JSON Schemas define the *inputs* for a rule, the Go templates define the *structure* and *logic* of the final Prometheus alert rule.

**1. Storage:**

The base templates for each rule will be stored as Go template files (with a `.tmpl` extension). For consolidated rules like OpenShift, there will be a single template file.

**Example Directory Structure:**
'''
/templates
|-- /_base
|   |-- openshift.json
|-- /go_templates
|   |-- openshift.tmpl      <-- The consolidated Go template
|-- openshift.json          <-- The consolidated JSON schema
'''

**2. Template Content:**

The `.tmpl` files contain the skeleton of the Prometheus rule in YAML format. For consolidated templates, we use `if/else` blocks to generate the correct rule based on the `rule_type`.

**Example: `/templates/go_templates/openshift.tmpl`**

```yaml
{{ if eq .rule_type "cpu" }}
- alert: HighCPUUsage_{{ .workload }}
  expr: sum(rate(container_cpu_usage_seconds_total{namespace="{{ .namespace }}", pod=~"{{ .workload }}-.*"}[5m])) by (pod) > 0.8
  for: 5m
  labels:
    severity: warning
    environment: {{ .environment }}
    namespace: {{ .namespace }}
  annotations:
    summary: "High CPU usage for {{ .workload }}"
{{ else if eq .rule_type "ram" }}
- alert: HighMemoryUsage_{{ .workload }}
  expr: sum(container_memory_working_set_bytes{namespace="{{ .namespace }}", pod=~"{{ .workload }}-.*"}) by (pod) > 1000000000
  for: 5m
  labels:
    severity: warning
    environment: {{ .environment }}
    namespace: {{ .namespace }}
  annotations:
    summary: "High Memory usage for {{ .workload }}"
{{ end }}
```
*Note the escaped `{{...}}` for fields that should be templated by Prometheus itself.*

**3. Templating Process:**

When a user submits a request to create a rule:
1.  The service identifies the requested rule "world" (e.g., `openshift`).
2.  It validates the user's JSON input against the corresponding schema (`/templates/openshift.json`).
3.  Upon successful validation, it loads the corresponding Go template (`/templates/go_templates/openshift.tmpl`).
4.  It executes the template, passing the user's validated JSON data (which includes the `rule_type`) as the input.
5.  The output is the final, rendered Prometheus rule in YAML format, ready to be used.

This approach provides a powerful and flexible way to manage our rule templates, cleanly separating the data definition (JSON Schema) from the presentation and logic (Go Templates).

### Template Storage Strategy

While storing templates on the local filesystem is simple and effective for a static set of rules, a more dynamic system can be achieved by storing templates in a remote database. This allows for updating rule templates without redeploying the Rule Manager service.

**1. Storage Backends:**

We will support multiple backends for template storage, configurable at startup. The proposed backends are:
*   `local`: The default, reading from the `/templates` directory.
*   `mongodb`: Fetches templates from a specified MongoDB collection.
*   `s3`: Fetches templates from a specified S3 bucket.

**2. Configuration:**

The application's configuration will be updated to include a `template_storage` section.

**Example `config.yaml`:**
```yaml
template_storage:
  type: mongodb # "local", "mongodb", or "s3"
  mongodb:
    connection_string: "mongodb://user:pass@host:27017/templates"
    database: "rule_templates"
    collection: "templates"
  # s3:
  #   bucket: "my-rule-templates"
  #   region: "us-east-1"
```

**3. Data Model (for MongoDB):**

When using MongoDB, templates would be stored in a collection with a structure like this:

```json
{
  "_id": "openshift",
  "type": "schema", // "schema" or "template"
  "content": "{ ... JSON schema content ... }"
}
```
```json
{
  "_id": "openshift_template",
  "type": "template",
  "content": "{{ if eq .rule_type ... Go template content ... }}"
}
```

**4. Implementation via `TemplateProvider` Interface:**

To abstract the storage mechanism, we will introduce a `TemplateProvider` interface in the `rules` service.

```go
package rules

// TemplateProvider defines the interface for retrieving rule templates.
type TemplateProvider interface {
    GetSchema(name string) (string, error)
    GetTemplate(name string) (string, error)
}
```

We will create concrete implementations for each storage backend (`LocalTemplateProvider`, `MongoTemplateProvider`, `S3TemplateProvider`). The application will instantiate the correct provider at startup based on the configuration.

**5. Caching:**

To avoid performance bottlenecks from fetching templates on every request, a caching layer will be implemented. The `TemplateProvider` will be wrapped in a `CachingTemplateProvider` that stores templates in memory. The cache will be populated at startup and can be refreshed via a dedicated API endpoint (e.g., `POST /-/reload-templates`).

This design provides the flexibility to manage templates dynamically while maintaining a clean separation of concerns and good performance.

### Dynamic Template Management via API

To transform the Rule Manager into a fully extensible platform, we will provide an API for users to create, manage, and validate their own "monitoring worlds" and rule templates. This makes the remote storage strategy essential.

**1. API Endpoints:**

A new set of RESTful endpoints will be created for managing templates. These endpoints will read from and write to the configured template storage backend (e.g., MongoDB).

*   `POST /api/v1/templates/schemas`: Create or update a JSON schema.
*   `GET /api/v1/templates/schemas/{name}`: Retrieve a JSON schema.
*   `DELETE /api/v1/templates/schemas/{name}`: Delete a JSON schema.

*   `POST /api/v1/templates/go-templates`: Create or update a Go template.
*   `GET /api/v1/templates/go-templates/{name}`: Retrieve a Go template.
*   `DELETE /api/v1/templates/go-templates/{name}`: Delete a Go template.

**2. Template Validation and Security:**

Allowing user-submitted Go templates introduces security risks, as a malicious or poorly-formed template could cause panics or excessive resource consumption. We will implement a multi-layered validation strategy to mitigate these risks.

**a. Parsing Validation:**
On any `POST` request to create or update a template, the service will first attempt to parse it. The request will be rejected if the template is not syntactically valid.

**b. Dry-Run Validation Endpoint:**
To help users build and test their templates safely, a dedicated validation endpoint will be provided.

*   `POST /api/v1/templates/validate`

This endpoint will accept a payload containing a Go template and example JSON data. The service will then attempt to render the template using the provided data in a "dry-run" mode. The response will indicate whether the rendering was successful and return the rendered output or any errors encountered. The template is **not** saved during this process.

**Example `validate` payload:**
```json
{
  "template_content": "{{ if .user }}Hello, {{ .user.name }}{{ end }}",
  "example_data": {
    "user": {
      "name": "Alex"
    }
  }
}
```

This provides an essential feedback loop for users to develop and debug their templates without affecting the system.

**3. Updated User Workflow:**

1.  A user defines a JSON schema for their new rule and a corresponding Go template.
2.  The user iteratively tests their template and schema using the `POST /api/v1/templates/validate` endpoint until it functions as expected.
3.  Once validated, the user uploads the final schema and Go template using the `POST /api/v1/templates/schemas` and `POST /api/v1/templates/go-templates` endpoints.
4.  The service saves the templates to the remote store (e.g., MongoDB) and invalidates its cache for the updated templates.
5.  The new rule template is now immediately available for use via the standard rule creation API.

### Huma Implementation Strategy

While Huma excels at generating OpenAPI specifications and validating requests based on static Go structs, our design requires validating against dynamic JSON schemas loaded from a database. The Huma documentation does not explicitly support this use case. Therefore, we will adopt a hybrid approach that leverages Huma's strengths while handling dynamic validation manually within the API handlers.

**1. Generic Request Body:**

For the rule creation endpoint (`POST /api/v1/rules`), we will not define a specific Go struct for every possible rule. Instead, we will define a generic struct that includes the rule's name and a field to hold the arbitrary user-provided parameters.

```go
// CreateRuleInput defines the request body for creating a new rule.
type CreateRuleInput struct {
    Body struct {
        TemplateName string          `json:"templateName"`
        Parameters   json.RawMessage `json:"parameters"` // Raw JSON for dynamic validation
    }
}
```

**2. Manual Validation in Handler:**

Within the Huma handler for this endpoint, we will perform the following steps:
1.  Huma will bind and validate the `CreateRuleInput` struct, ensuring `templateName` and `parameters` are present.
2.  We will use the `templateName` to fetch the corresponding JSON schema from our `TemplateProvider` (which may be cached).
3.  We will use a dedicated Go JSON Schema validation library (e.g., `go-jsonschema`) to validate the `parameters` (`json.RawMessage`) against the schema retrieved in the previous step.
4.  If the manual validation fails, we return a `400 Bad Request` response with the validation errors.
5.  If it succeeds, we proceed with the rule generation logic, passing the validated `parameters` to the Go template engine.

**3. OpenAPI Specification:**

A limitation of this approach is that the dynamic `parameters` field cannot be automatically documented in detail in the OpenAPI specification. Huma will document it as a generic JSON object.

To compensate for this, the API will provide a separate endpoint to retrieve the JSON schema for a given template (e.g., `GET /api/v1/templates/schemas/{name}`). This allows API clients and UIs to dynamically fetch the schema and build appropriate forms or requests.

**4. Leveraging Huma's Strengths:**

This hybrid model allows us to still use Huma for:
*   **Routing:** Managing all our API endpoints.
*   **Static Validation:** Validating the structure of all API requests and responses that *are* static (e.g., the template management endpoints).
*   **Response Handling:** Marshaling Go structs into JSON responses with correct status codes.
*   **Core OpenAPI Documentation:** Generating the overall API documentation, even if some parts are less specific.

This strategy provides a clear and robust path for integrating Huma's powerful features with the project's core requirement for dynamic, user-defined rule templates.

### Recommended Go Packages and Dependencies

This section outlines recommended third-party and standard library packages that align with the architectural design of the project.

**Core Packages:**

*   **Configuration (`github.com/spf13/viper`):** For loading `config.yaml` and managing configuration from multiple sources like environment variables.
*   **MongoDB Driver (`go.mongodb.org/mongo-driver`):** The official driver for all interactions with a MongoDB backend.
*   **JSON Schema Validation (`github.com/xeipuuv/gojsonschema`):** Essential for the manual validation of dynamic rule parameters within the Huma handler.
*   **Structured Logging (`log/slog`):** The official structured logging library (Go 1.21+) for creating machine-readable logs.
*   **AWS SDK (`github.com/aws/aws-sdk-go-v2`):** The official SDK for implementing the S3 template storage backend.
*   **Testing (`github.com/stretchr/testify`):** Provides powerful assertion (`assert`, `require`) and mocking (`mock`) capabilities to make testing more effective and readable.

**Prometheus & VictoriaMetrics Ecosystem Integration:**

To ensure the generated rules are valid and compatible with Prometheus and VictoriaMetrics, we should directly leverage packages from the ecosystem.

*   **PromQL/MetricQL Expression Validation (`github.com/VictoriaMetrics/metricsql`):**
    *   **Consideration:** The `expr` field in a rule template is critical and prone to syntax errors. Instead of discovering these errors only when Prometheus/VictoriaMetrics loads the rule, we should validate them proactively.
    *   **Implementation:** When a Go template for a rule is created or updated (via the API), the service should perform a validation step. It can render the template with dummy data and then use the `metricsql.Parse()` function from this package to check if the resulting query expression is syntactically valid. This provides immediate feedback to the user and prevents invalid rules from being stored. Using `metricsql` allows us to support both standard PromQL and VictoriaMetrics-specific MetricQL extensions.

*   **Official Data Structures:**
    *   **Consideration:** While `github.com/prometheus/common/model` is useful, for this project we are generating YAML directly via Go templates to ensure maximum flexibility and compatibility with `vmalert`'s expected format, which might differ slightly or require specific ordering not guaranteed by struct marshaling. We rely on `metricsql` for the critical expression validation.

### Integration with VictoriaMetrics (vmalert)

To allow seamless integration with the VictoriaMetrics ecosystem, the service must expose all generated rules in a YAML format compatible with `vmalert`'s `-rule` flag when pointed at an HTTP endpoint.

**1. `vmalert` Rule Format:**

`vmalert` expects a YAML file containing a list of rule groups. The service will generate a single YAML document dynamically that adheres to this format.

**Example YAML Format:**
```yaml
groups:
  - name: openshift_rules
    rules:
      - alert: HighCpuUsage_my-app
        expr: "sum(rate(container_cpu_usage_seconds_total{...}[5m])) / ... > 80"
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High CPU usage detected"
      # ... more rules in this group
  - name: kafka_rules
    rules:
      - alert: KafkaTooManyOfflinePartitions
        expr: "kafka_topic_partitions{state="offline"} > 0"
        for: 15m
        labels:
          severity: critical
        annotations:
          summary: "Kafka has offline partitions"
      # ... more rules in this group
```

**2. API Endpoint:**

A new GET endpoint will be created to serve this YAML file. The response `Content-Type` will be `application/x-yaml`.

*   `GET /api/v1/rules/vmalert`

This endpoint will be designed for frequent polling by `vmalert` instances.

**3. Implementation Details:**

The handler for this endpoint will:
1.  Fetch all saved rules from the database.
2.  Group these rules. A default grouping strategy could be by "monitoring world" (e.g., `openshift`, `kafka`), which can be derived from the template name used by the rule.
3.  For each rule, render its corresponding Go template with the saved parameters to generate the final rule object (as a Go struct compatible with a YAML library).
4.  Construct the final data structure containing the list of groups.
5.  Use a standard Go YAML library (e.g., `gopkg.in/yaml.v3`) to marshal the data structure into a single YAML string.
6.  Serve the resulting YAML string.

**4. Performance and Caching:**

Generating this file on every request is not feasible. A caching layer is essential.
*   **In-Memory Cache:** The generated YAML output will be cached in memory as a string.
*   **Cache Invalidation:** The cache for this endpoint will be invalidated and the YAML string regenerated whenever a rule is created, updated, or deleted via the API. This ensures that `vmalert` always receives an up-to-date set of rules on its next poll.

This approach ensures efficient and reliable integration with VictoriaMetrics.

### Datasource Definition in Templates

To ensure that rules are queried against the correct Time-Series Database (TSDB), the definition of the datasource is embedded directly within the rule template's JSON Schema. This makes the template self-contained and removes the need for the user to select a datasource when creating a rule.

The datasource information is defined in a `datasource` object within the schema. This object is not part of the user-provided `properties` but is a top-level key in the schema file itself, serving as metadata.

**Example: Part of a Template Schema (`/templates/vm_single_node.json`)**

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.com/schemas/vm_single_node.json",
  "title": "VictoriaMetrics Single Node Alert",
  "description": "Alerts for a single node VictoriaMetrics instance.",
  "type": "object",
  "datasource": {
    "type": "victoriametrics",
    "url": "http://vm-single-node.example.com:8428",
    "credentials": {
      "type": "basic_auth",
      "secret_ref": "vm-read-creds"
    }
  },
  "properties": {
    "instance_name": {
      "type": "string",
      "description": "The name of the VictoriaMetrics instance."
    },
    "disk_usage_threshold": {
      "type": "number",
      "description": "Disk usage threshold percentage (e.g., 85)."
    }
  },
  "required": ["instance_name", "disk_usage_threshold"]
}
```

**Explanation:**

*   **`datasource`:** A top-level object containing the TSDB connection details.
*   **`type`:** The type of TSDB (e.g., `thanos`, `prometheus`, `victoriametrics`).
*   **`url`:** The query endpoint for the TSDB.
*   **`credentials`:** An object specifying how to authenticate.
    *   **`type`:** The authentication method (e.g., `bearer_token`, `basic_auth`).
    *   **`secret_ref`:** A reference to a secret managed by a separate secrets management system (like Kubernetes Secrets or HashiCorp Vault). The Rule Manager would be responsible for resolving this reference to the actual secret value. **We will not store raw credentials in the template.**

When a rule is created from this template, the Rule Manager service will:
1.  Validate the user's input against the `properties` in the schema.
2.  Store the rule with its parameters.
3.  Internally associate the created rule with the datasource information from the template. This information is used by the query/execution engine but is not part of the user-facing rule object.

This approach tightly couples a rule template to its required datasource, ensuring correctness and simplifying the user experience.

### Rule Lifecycle Management (CRUD)

To manage the lifecycle of rule instances, a full suite of CRUD (Create, Read, Update, Delete) API endpoints will be implemented. This design is centered around a database abstraction layer to ensure flexibility and adherence to best practices.

**1. Database Abstraction Layer (`RuleStore`):**

To decouple our business logic from a specific database technology, we will define a `RuleStore` interface within the `database` package. All API handlers will interact with this interface, not with a concrete database driver.

**Rule Data Model:**
```go
package database

import (
	"encoding/json"
	"time"
)

// Rule represents a user-defined alert rule instance.
type Rule struct {
	ID           string          `json:"id" bson:"_id,omitempty"`
	TemplateName string          `json:"templateName" bson:"templateName"`
	Parameters   json.RawMessage `json:"parameters" bson:"parameters"`
	CreatedAt    time.Time       `json:"createdAt" bson:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt" bson:"updatedAt"`
}
```

**RuleStore Interface:**
```go
package database

import "context"

// RuleStore defines the interface for database operations on rules.
type RuleStore interface {
    CreateRule(ctx context.Context, rule *Rule) error
    GetRule(ctx context.Context, id string) (*Rule, error)
    ListRules(ctx context.Context, offset, limit int) ([]*Rule, error)
    UpdateRule(ctx context.Context, id string, rule *Rule) error
    DeleteRule(ctx context.Context, id string) error
}
```

At startup, we will instantiate a concrete implementation of this interface (e.g., `MongoRuleStore`) based on the application's configuration and inject it into the API handlers.

**2. API Endpoints (Huma Operations):**

The following endpoints will be defined to manage the rules.

*   **Create Rule:** `POST /api/v1/rules`
    *   **Request Body:** `{ "templateName": "...", "parameters": { ... } }`
    *   **Logic:** The handler validates the input against the dynamic JSON schema (which includes the datasource definition), then calls `RuleStore.CreateRule()`. **Must trigger `vmalert` cache invalidation.**

*   **List Rules:** `GET /api/v1/rules`
    *   **Query Parameters:** `offset` (int), `limit` (int) for pagination.
    *   **Logic:** Calls `RuleStore.ListRules()` and returns the list of rules.

*   **Get Rule:** `GET /api/v1/rules/{ruleId}`
    *   **Path Parameter:** `ruleId` (string).
    *   **Logic:** Calls `RuleStore.GetRule()` and returns the specific rule or a 404 error.

*   **Update Rule:** `PUT /api/v1/rules/{ruleId}`
    *   **Path Parameter:** `ruleId` (string).
    *   **Request Body:** `{ "templateName": "...", "parameters": { ... } }`
    *   **Logic:** The handler validates the new input against the schema, then calls `RuleStore.UpdateRule()`. **Must trigger `vmalert` cache invalidation.**

*   **Delete Rule:** `DELETE /api/v1/rules/{ruleId}`
    *   **Path Parameter:** `ruleId` (string).
    *   **Logic:** Calls `RuleStore.DeleteRule()`. **Must trigger `vmalert` cache invalidation.**

This approach provides a clean, RESTful API for managing rules while maintaining a flexible and maintainable data layer that can evolve with the project's needs.

### Local Mode (File Store)

For development and testing purposes, the Rule Manager can run in "Local Mode" using the filesystem instead of MongoDB. This is controlled by the `template_storage.type` configuration.

**Configuration (`config.yaml`):**

```yaml
template_storage:
  type: "file"
  file:
    path: "./data" # Directory to store rules and templates
```

When enabled, rules and templates are stored as JSON files in the specified directory. This allows developers to run the application with zero external dependencies.

### Rule Creation Pipelines

To provide advanced validation, enrichment, and automation when a rule is created, we will introduce a "pipeline" concept. A pipeline is a series of declarative steps defined within the rule template's JSON schema that are executed sequentially by the Rule Manager during the rule creation process.

This approach allows us to add complex behaviors (like checking if a metric exists before creating the rule) without modifying the core service code, making the system highly extensible and keeping the logic tied to the template it belongs to.

**1. Declaring Pipelines in the JSON Schema:**

A new top-level `pipelines` array can be added to any rule template's JSON schema. Each object in the array represents a step to be executed.

**2. Conditional Execution for Consolidated Templates:**

For consolidated templates that handle multiple rule types (e.g., a single `openshift.json` for both CPU and memory alerts), pipeline steps can be executed conditionally. This is achieved by adding a `condition` object to the pipeline step definition. The step will only run if the user's input matches the condition.

**Example: Schema with a Conditional Metric Validation Pipeline**

This example for a consolidated OpenShift template shows how to run a specific validation check only when the corresponding `rule_type` is selected by the user.

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "OpenShift Consolidated Rules",
  "datasource": {
    "type": "prometheus",
    "url": "https://prometheus.example.com:9090"
  },
  "pipelines": [
    {
      "name": "Validate OpenShift CPU Metric",
      "condition": { "property": "rule_type", "equals": "cpu_usage" },
      "type": "validate_metric_exists",
      "parameters": {
        "metric_name": "container_cpu_usage_seconds_total",
        "labels": "{{ .workload_labels }}"
      }
    },
    {
      "name": "Validate OpenShift Memory Metric",
      "condition": { "property": "rule_type", "equals": "memory_usage" },
      "type": "validate_metric_exists",
      "parameters": {
        "metric_name": "container_memory_working_set_bytes",
        "labels": "{{ .workload_labels }}"
      }
    }
  ],
  "properties": {
    "rule_type": {
      "type": "string",
      "enum": ["cpu_usage", "memory_usage"]
    },
    "workload_labels": {
      "type": "object",
      "description": "Labels to identify the specific workload (e.g., deployment, statefulset)."
    }
    // ... other properties
  }
}
```

**3. Pipeline Processor Implementation:**

The Rule Manager service will include a `PipelineProcessor`. When a rule is created:

1.  The processor retrieves the `pipelines` array from the template schema.
2.  It iterates through each step.
3.  For each step, it first checks if a `condition` is present. If so, it evaluates the condition against the user's input parameters. If the condition is not met, the step is skipped.
4.  If the step is to be executed, the processor looks up a registered "Step Runner" that matches the `type` (e.g., `validate_metric_exists`).
5.  It executes the Step Runner, passing it the rule's parameters and the parameters defined for that pipeline step. The runner can also access the `datasource` information from the schema.
6.  If any step fails (e.g., a validation step returns `false`), the entire pipeline is halted, and a descriptive error is returned to the user, preventing the rule from being created.

**4. Core Pipeline Step Runner: `validate_metric_exists`**

We will implement a set of built-in Step Runners. The first one is for validation.

*   **`validate_metric_exists`**:
    *   **Purpose:** Checks if a given metric, optionally with specific labels, exists in the target TSDB. This prevents the creation of rules that will never fire due to a typo or misconfiguration.
    *   **Logic:**
        1.  The runner receives the `metric_name` and `labels` from the pipeline step's parameters, templating them with the user's input where necessary.
        2.  It connects to the `datasource` specified in the schema (e.g., a Prometheus endpoint).
        3.  It constructs a query to check for the existence of the metric. For Prometheus, a simple way to do this is to query the `/api/v1/series` endpoint with a `match[]` parameter (e.g., `GET /api/v1/series?match[]={__name__="up",job="prometheus"}`).
        4.  It executes the query. If the query returns an empty result, it means no time series match, and the validation step fails.
        5.  If the step fails, the pipeline is halted, and an error is returned to the user.

This pipeline system provides a powerful mechanism for adding custom, declarative logic to the rule creation process in a maintainable and extensible way.



### External Service Integration

A client for the external validation service will be implemented in the `validator` package. This client will be responsible for making requests to the external service and handling the responses.

## Development Setup

(To be added: Instructions on how to set up the development environment, including Go version, database setup, and any other dependencies.)


