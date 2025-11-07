# Technical Development Details

## Introduction

This document outlines the technical details and implementation plan for the Rule Manager project. It is intended to be a living document that will be updated as the project evolves.

## Proposed Project Structure

A standard Go project structure will be used to organize the code:

```
/rulemanager
|-- /api
|   |-- router.go
|   |-- handlers.go
|-- /cmd
|   |-- /rulemanager
|       |-- main.go
|-- /config
|   |-- config.go
|-- /internal
|   |-- /database
|   |   |-- mongodb.go
|   |   |-- database.go // Interface for DB operations
|   |-- /rules
|   |   |-- service.go
|   |   |-- models.go
|   |-- /validator
|       |-- external_service.go
|-- /templates
|   |-- kafka_topic_rule.yaml
|-- go.mod
|-- go.sum
|-- README.md
|-- DEVELOPMENT.md
```

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

The templates will be stored as `.json` files in the `/templates` directory and loaded at startup.

### External Service Integration

A client for the external validation service will be implemented in the `validator` package. This client will be responsible for making requests to the external service and handling the responses.

## Development Setup

(To be added: Instructions on how to set up the development environment, including Go version, database setup, and any other dependencies.)

## Initial Tasks

*   [ ] Set up the basic project structure.
*   [ ] Implement the configuration loading.
*   [ ] Define the database interface and the MongoDB implementation.
*   [ ] Implement the service for managing rule templates.
*   [ ] Create the initial API endpoints for listing templates.
