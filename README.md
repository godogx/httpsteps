# Cucumber HTTP steps for Go

[![Build Status](https://github.com/godogx/httpsteps/workflows/test-unit/badge.svg)](https://github.com/godogx/httpsteps/actions?query=branch%3Amaster+workflow%3Atest-unit)
[![Coverage Status](https://codecov.io/gh/godogx/httpsteps/branch/master/graph/badge.svg)](https://codecov.io/gh/godogx/httpsteps)
[![GoDevDoc](https://img.shields.io/badge/dev-doc-00ADD8?logo=go)](https://pkg.go.dev/github.com/godogx/httpsteps)
[![Time Tracker](https://wakatime.com/badge/github/godogx/httpsteps.svg)](https://wakatime.com/badge/github/godogx/httpsteps)
![Code lines](https://sloc.xyz/github/godogx/httpsteps/?category=code)
![Comments](https://sloc.xyz/github/godogx/httpsteps/?category=comments)

This module implements HTTP-related step definitions
for [`github.com/cucumber/godog`](https://github.com/cucumber/godog).

## Steps

### Local Client 

Local and remote services can be tested with client request configuration and response expectations.

Please note, due to centralized nature of these mocks they can not be used concurrently by different scenarios.
If multiple scenarios configure a shared service, they will be locked in a sync sequence.
It is safe to use concurrent scenarios.

#### Request Setup

```gherkin
When I request HTTP endpoint with method "GET" and URI "/get-something?foo=bar"
```

In request configuration steps you can specify name of the service to apply configuration.
If service name is omitted, default service (with URL passed to `NewLocalClient`) is used:
* `request HTTP endpoint` - default service,
* `request "some-service" HTTP endpoint` - service named `some-service`.

Named services have to be explicitly added with their base URLs before running tests.

```gherkin
When I request "some-service" HTTP endpoint with method "GET" and URI "/get-something?foo=bar"
```

An additional header can be supplied. For multiple headers, call step multiple times.

```gherkin
And I request HTTP endpoint with header "X-Foo: bar"
```

```gherkin
And I request "some-service" HTTP endpoint with header "X-Foo: bar"
```

Or use table of values.

```gherkin
And I request "some-service" HTTP endpoint with headers
  | X-Foo | foo |
  | X-Bar | 123 |
```

An additional cookie can be supplied. For multiple cookies, call step multiple times.

```gherkin
And I request HTTP endpoint with cookie "name: value"
```
```gherkin
And I request "some-service" HTTP endpoint with cookie "name: value"
```

Or use table of values.

```gherkin
And I request "some-service" HTTP endpoint with cookies
  | cfoo | foo |
  | cbar | 123 |
```

Optionally request body can be configured. If body is a valid JSON5 payload, it will be converted to JSON before use.
Otherwise, body is used as is.

```gherkin
And I request HTTP endpoint with body
"""
[
  // JSON5 comments are allowed.
  {"some":"json"}
]
"""
```

Request body can be provided from file.

```gherkin
And I request HTTP endpoint with body from file
"""
path/to/file.json5
"""
```

Request body can be defined as form data.

```gherkin
And I request "some-service" HTTP endpoint with urlencoded form data
  | ffoo | abc |
  | fbar | 123 |
  | fbar | 456 |
```


By default, redirects are not followed. This behavior can be changed.

```gherkin
And I follow redirects from HTTP endpoint
```

```gherkin
And I follow redirects from "some-service" HTTP endpoint
```

If endpoint is capable of handling duplicated requests, you can check it for idempotency. This would send multiple
requests simultaneously and check

* if all responses are similar or (all successful like GET)
* if responses can be grouped into exactly ONE response of a kind and OTHER responses of another kind (one successful,
  other failed like with POST).

Number of requests can be configured with `Local.ConcurrencyLevel`, default value is 10.

```gherkin
And I concurrently request idempotent HTTP endpoint
```

Or for a named service.

```gherkin
And I concurrently request idempotent "some-service" HTTP endpoint
```


#### Response Expectations

Response expectation has to be configured with at least one step about status, response body or other responses body (
idempotency mode).

If response body is a valid JSON5 payload, it is converted to JSON before use.

JSON bodies are compared with [`assertjson`](https://github.com/swaggest/assertjson) which allows ignoring differences
when expected value is set to `"<ignore-diff>"`.

```gherkin
And I should have response with body
"""
[
  {"some":"json","time":"<ignore-diff>"}
]
"""
```

```gherkin
And I should have response with body from file
"""
path/to/file.json
"""
```

Status can be defined with either phrase or numeric code.

```gherkin
Then I should have response with status "OK"
```

```gherkin
Then I should have response with status "204"

And I should have other responses with status "Not Found"
```

In an idempotent mode you can check other responses.

```gherkin
And I should have other responses with body
"""
{"status":"failed"}
"""
```

```gherkin
And I should have other responses with body from file
"""
path/to/file.json
"""
```

Optionally response headers can be asserted.

```gherkin
Then I should have response with header "Content-Type: application/json"

And I should have other responses with header "Content-Type: text/plain"
And I should have other responses with header "X-Header: abc"
```

Header can be checked using a table.

```gherkin
And I should have "some-service" response with headers
  | Content-Type | application/json |
  | X-Baz        | abc              |
```

You can set expectations for named service by adding service name before `response` or `other responses`:
* `have response` - default,
* `have other responses` - default,
* `have "some-service" response` - service named `some-service`,
* `have "some-service" other responses` - service named `some-service`.

### External Server

External Server mock creates an HTTP server for each of registered services and allows control of expected 
requests and responses with gherkin steps. 

It is useful describe behavior of HTTP endpoints that are called by the app during test (e.g. 3rd party APIs).

Please note, due to centralized nature of these mocks they can not be used concurrently by different scenarios. 
If multiple scenarios configure a shared service, they will be locked in a sync sequence.
It is safe to use concurrent scenarios.

In simple case you can define expected URL and response.

```gherkin
Given "some-service" receives "GET" request "/get-something?foo=bar"

And "some-service" responds with status "OK" and body
"""
{"key":"value"}
"""
```

Or request with body.

```gherkin
And "another-service" receives "POST" request "/post-something" with body
"""
// Could be a JSON5 too.
{"foo":"bar"}
"""
```

Request with body from a file.

```gherkin
And "another-service" receives "POST" request "/post-something" with body from file
"""
_testdata/sample.json
"""
```

Request can expect to have a header.

```gherkin
And "some-service" request includes header "X-Foo: bar"
```

By default, each configured request is expected to be received 1 time. This can be changed to a different number.

```gherkin
And "some-service" request is received 1234 times
```

Or to be unlimited.

```gherkin
And "some-service" request is received several times
```

By default, requests are expected in same sequential order as they are defined. If there is no stable order you can have
an async expectation. Async requests are expected in any order.

```gherkin
And "some-service" request is async
```

Response may have a header.

```gherkin
And "some-service" response includes header "X-Bar: foo"
```

Response must have a status.

```gherkin
And "some-service" responds with status "OK"
```

Response may also have a body.

```gherkin
And "some-service" responds with status "OK" and body
"""
{"key":"value"}
"""
```

```gherkin
And "another-service" responds with status "200" and body from file
"""
_testdata/sample.json5
"""
```

### Dynamic Variables

When data is not known in advance, but can be inferred from previous steps, you can use 
[dynamic variables](https://github.com/bool64/shared).

Here is an example where value from response of one step is used in request of another step.

```gherkin
  Scenario: Creating user and making an order
    When I request HTTP endpoint with method "POST" and URI "/user"

    And I request HTTP endpoint with body
    """json
    {"name": "John Doe"}
    """

    # Undefined variable infers its value from the actual data on first encounter.
    Then I should have response with body
    """json5
    {
      // Capturing dynamic user id as $user_id variable.
     "id":"$user_id",
     "name": "John Doe",
     // Ignoring other dynamic values.
     "created_at":"<ignore-diff>","updated_at": "<ignore-diff>"
    }
    """

    # Creating an order for that user with $user_id.
    When I request HTTP endpoint with method "POST" and URI "/order"

    And I request HTTP endpoint with body
    """json5
    {
      // Replacing with the value of a variable captured previously.
      "user_id": "$user_id",
      "item_name": "Watermelon"
    }
    """
    # Variable interpolation works also with body from file.

    Then I should have response with body
    """json5
    {
     "id":"<ignore-diff>",
     "created_at":"<ignore-diff>","updated_at": "<ignore-diff>",
     "user_id":"$user_id"
    }
    """
```


## Example Feature

```gherkin
Feature: Example

  Scenario: Successful GET Request
    Given "template-service" receives "GET" request "/template/hello"

    And "template-service" responds with status "OK" and body
    """
    Hello, %s!
    """

    When I request HTTP endpoint with method "GET" and URI "/?name=Jane"

    Then I should have response with status "OK"

    And I should have response with body
    """
    Hello, Jane!
    """
```