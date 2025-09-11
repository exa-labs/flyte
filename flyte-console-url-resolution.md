# Flyte Console URL Resolution: From Helm Chart to UI Links

## Overview

Flyte resolves templates defined in the helm chart to render links in the tasks/workflows UI through a multi-step process involving configuration loading, workflow execution setup, and runtime URL construction.

## The Flow: From Helm Chart to UI Links

### 1. Helm Chart Template Definition

The console URL configuration is defined in the Flyte helm chart at `charts/flyte-core/values.yaml`:

```yaml
configmap:
  # Configuration for Flyte console UI
  console:
    BASE_URL: /console
    CONFIG_DIR: /etc/flyte/config
```

This configuration is rendered into a Kubernetes ConfigMap via the template at `charts/flyte-core/templates/console/configmap.yaml`:

```yaml
{{- if .Values.flyteconsole.enabled }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: flyte-console-config
  namespace: {{ template "flyte.namespace" . }}
  labels: {{ include "flyteconsole.labels" . | nindent 4 }}
data: {{ toYaml .Values.configmap.console | nindent 2 }}
{{- end }}
```

### 2. Configuration Loading in FlyteAdmin

The console URL configuration is loaded into FlyteAdmin through the application configuration system:

- **Configuration Structure**: Defined in `flyteadmin/pkg/runtime/interfaces/application_configuration.go`
  ```go
  type ApplicationConfig struct {
      // ... other fields ...
      // A URL pointing to the flyteconsole instance used to hit this flyteadmin instance.
      ConsoleURL string `json:"consoleUrl,omitempty" pflag:",A URL pointing to the flyteconsole instance used to hit this flyteadmin instance."`
      // ... other fields ...
  }
  ```

- **Access Method**: `GetTopLevelConfig()` method provides access to this configuration throughout the application.

### 3. Workflow Execution Setup

When a workflow execution is created, the console URL is embedded into the workflow CRD:

**Location**: `flyteadmin/pkg/workflowengine/impl/k8s_executor.go`

```go
func (e K8sWorkflowExecutor) Execute(ctx context.Context, data interfaces.ExecutionData) (interfaces.ExecutionResponse, error) {
    // ... workflow building logic ...
    
    // Set console URL from configuration
    if consoleURL := e.config.ApplicationConfiguration().GetTopLevelConfig().ConsoleURL; len(consoleURL) > 0 {
        flyteWf.ConsoleURL = consoleURL
    }
    
    // ... create workflow CRD in Kubernetes ...
}
```

### 4. Console URL Storage in Workflow CRD

The console URL is stored in the `FlyteWorkflow` CRD object:

**Location**: `flytepropeller/pkg/apis/flyteworkflow/v1alpha1/workflow.go`

```go
type FlyteWorkflow struct {
    // ... other fields ...
    // Flyteconsole url
    ConsoleURL string `json:"consoleUrl,omitempty"`
    // ... other fields ...
}

func (in *FlyteWorkflow) GetConsoleURL() string { 
    return in.ConsoleURL 
}
```

### 5. Runtime URL Construction for Task Execution

During task execution, the console URL is constructed dynamically and passed to running tasks as environment variables:

**Location**: `flyteplugins/go/tasks/pluginmachinery/flytek8s/k8s_resource_adds.go`

```go
func GetExecutionEnvVars(id pluginsCore.TaskExecutionID, consoleURL string) []v1.EnvVar {
    // ... other environment variables ...
    
    if len(consoleURL) > 0 {
        consoleURL = strings.TrimRight(consoleURL, "/")
        envVars = append(envVars, v1.EnvVar{
            Name:  flyteExecutionURL,
            Value: fmt.Sprintf("%s/projects/%s/domains/%s/executions/%s/nodeId/%s/nodes", 
                   consoleURL, 
                   nodeExecutionID.GetProject(), 
                   nodeExecutionID.GetDomain(), 
                   nodeExecutionID.GetName(), 
                   id.GetUniqueNodeID()),
        })
    }
    
    return envVars
}
```

### 6. URL Template Format

The console URL follows this template pattern:
```
{consoleURL}/projects/{project}/domains/{domain}/executions/{executionId}/nodeId/{nodeId}/nodes
```

## Key Components and Their Roles

### 1. **Helm Chart Configuration**
- **File**: `charts/flyte-core/values.yaml`
- **Role**: Defines the base console URL template
- **Default**: `BASE_URL: /console`

### 2. **FlyteAdmin Configuration**
- **File**: `flyteadmin/pkg/runtime/interfaces/application_configuration.go`
- **Role**: Defines the configuration structure for console URL
- **Field**: `ConsoleURL string`

### 3. **Workflow Executor**
- **File**: `flyteadmin/pkg/workflowengine/impl/k8s_executor.go`
- **Role**: Embeds console URL into workflow CRD during execution creation
- **Method**: `Execute()` method sets `flyteWf.ConsoleURL`

### 4. **Workflow CRD**
- **File**: `flytepropeller/pkg/apis/flyteworkflow/v1alpha1/workflow.go`
- **Role**: Stores console URL in the workflow object
- **Field**: `ConsoleURL string`

### 5. **Task Execution Environment**
- **File**: `flyteplugins/go/tasks/pluginmachinery/flytek8s/k8s_resource_adds.go`
- **Role**: Constructs specific console URLs for tasks at runtime
- **Method**: `GetExecutionEnvVars()` creates `FLYTE_EXECUTION_URL` environment variable

## Configuration Flow

1. **Helm Deployment**: Helm chart values define console configuration
2. **ConfigMap Creation**: Kubernetes ConfigMap stores console configuration
3. **FlyteAdmin Startup**: Loads configuration from config files/environment
4. **Workflow Creation**: Console URL embedded in workflow CRD
5. **Task Execution**: Specific console URLs generated for each task
6. **Environment Variables**: Tasks receive `FLYTE_EXECUTION_URL` pointing to their console page

## Configuration Sources

The console URL can be configured through several methods:

1. **Helm Values**: `charts/flyte-core/values.yaml` → `configmap.console.BASE_URL`
2. **Environment Variables**: Via FlyteAdmin configuration
3. **Configuration Files**: YAML configuration files read by FlyteAdmin
4. **Command Line Flags**: Via pflag system in FlyteAdmin

## Summary

Flyte resolves console URL templates through a configuration pipeline that starts with helm chart values, flows through FlyteAdmin configuration, gets embedded in workflow CRDs, and finally generates specific console URLs for individual tasks at runtime. The system provides flexibility to configure the console URL at deployment time while dynamically constructing task-specific URLs during execution.