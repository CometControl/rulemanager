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
      "environment": "production",
      "namespace": "payment-service",
      "workload": "payment-api",
      "rule_type": "cpu"
    }
  }'
```

### Retrieving Rules for vmalert

Configure your `vmalert` to poll this endpoint:

```bash
curl http://localhost:8080/api/v1/rules/vmalert
```

## Architecture

The Rule Manager follows a clean architecture pattern:
*   **API Layer**: Built with [Huma](https://huma.rocks/), providing robust routing and validation.
*   **Service Layer**: Handles business logic, template rendering, and pipeline execution.
*   **Data Layer**: Abstracted storage interfaces allowing for swappable backends (MongoDB, File, S3).

For more detailed information, please refer to the documentation in the `docs/` directory:
*   [Business Requirements](docs/business_requirements.md)
*   [Technical Specification](docs/technical_spec.md)
