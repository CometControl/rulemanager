# Business Requirements Document (BRD)

## 1. Executive Summary

The Rule Manager service is a centralized platform designed to streamline the creation, management, and validation of alerting rules for monitoring systems like Prometheus and VictoriaMetrics. By abstracting the complexity of PromQL and rule configuration behind user-friendly templates, it empowers engineering teams to define consistent and error-free alerts without deep domain expertise in monitoring query languages.

## 2. Problem Statement

Managing alerting rules in a large-scale, microservices environment presents several challenges:
*   **Complexity**: Writing correct PromQL queries is difficult and error-prone.
*   **Inconsistency**: Lack of standardization leads to fragmented alerting strategies across teams.
*   **Validation**: Rules are often only validated at deployment time, leading to broken alerts in production.
*   **Scalability**: Manual management of rule files becomes unmanageable as the number of services grows.

## 3. Scope

### In Scope
*   **Rule Management**: CRUD operations for alerting rules.
*   **Template Management**: System for defining and storing rule templates.
*   **Validation**: Pre-flight checks for rule parameters and metric existence.
*   **Integration**: Generation of `vmalert`-compatible rule files.
*   **Storage**: Support for MongoDB and local filesystem storage.

### Out of Scope
*   **Alert Execution**: The service generates rules but does not execute them. This is handled by `vmalert` or Prometheus.
*   **Alert Routing**: Notification routing (PagerDuty, Slack) is handled by Alertmanager, not this service.

## 4. Functional Requirements

### 4.1 Rule Management
*   **FR-01**: Users must be able to create new alerting rules based on existing templates.
*   **FR-02**: Users must be able to list, view, update, and delete existing rules.
*   **FR-03**: The system must validate user input against the template's schema before creating a rule.

### 4.2 Template Management
*   **FR-04**: Administrators must be able to define rule templates using JSON Schema (for input) and Go Templates (for output).
*   **FR-05**: Templates must support dynamic validation logic (Pipelines).
*   **FR-06**: Templates must be versioned or updateable without service redeployment.

### 4.3 Validation
*   **FR-07**: The system must support "Dry-Run" validation to test templates.
*   **FR-08**: The system must be able to query the target TSDB to verify if metrics exist before creating a rule (Pipeline Validation).

### 4.4 Integration
*   **FR-09**: The system must expose a `GET` endpoint that returns all active rules in a format consumable by `vmalert`.

## 5. Non-Functional Requirements

### 5.1 Usability
*   **NFR-01**: The API must be RESTful and documented (OpenAPI).
*   **NFR-02**: Error messages must be descriptive and actionable.

### 5.2 Performance
*   **NFR-03**: The `vmalert` endpoint must respond within 200ms, utilizing caching to avoid database load on every poll.

### 5.3 Extensibility
*   **NFR-04**: The system must be designed to support multiple storage backends (e.g., S3) in the future with minimal code changes.
*   **NFR-05**: The system must support multiple datasource types (Prometheus, VictoriaMetrics).
