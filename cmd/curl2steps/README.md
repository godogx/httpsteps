# curl2steps

This tool converts `curl` command to steps.

## Examples

```
curl2steps my_service curl -d '{"key1":"value1", "key2":"value2"}' -H "Content-Type: application/json" -X POST http://localhost:3000/data
```

```gherkin
    When I request "my_service" HTTP endpoint with method "POST" and URI "/data"
    And I request "my_service" HTTP endpoint with headers
        | Content-Type | application/json |
    And I request "my_service" HTTP endpoint with body
    ```json
    {"key1":"value1", "key2":"value2"}
    ```
```

```
curl2steps my_service curl -d "param1=value1&param2=value2" -X POST http://localhost:3000/data
```

```gherkin
    When I request "my_service" HTTP endpoint with method "POST" and URI "/data"
    And I request "my_service" HTTP endpoint with headers
        | Content-Type | application/x-www-form-urlencoded |
    And I request "my_service" HTTP endpoint with urlencoded form data
        | param1 | value1 |
        | param2 | value2 |
```