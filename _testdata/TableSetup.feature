Feature: Table Setup

  Scenario: Request and response can be configured with table parameters
    When I request "some-service" HTTP endpoint with method "POST" and URI "/hello"
    And I request "some-service" HTTP endpoint with headers
      | X-Foo | foo |
      | X-Bar | 123 |
    And I request "some-service" HTTP endpoint with query parameters
      | qfoo | foo |
      | qbar | 123 |
      | qbar | 456 |
    And I request "some-service" HTTP endpoint with cookies
      | cfoo | foo |
      | cbar | 123 |
    And I request "some-service" HTTP endpoint with urlencoded form data
      | ffoo | abc |
      | fbar | 123 |
      | fbar | 456 |

    Then I should have "some-service" response with status "OK"
    And I should have "some-service" response with body
    """json
    [
      {"some":"json"}
    ]
    """
    And I should have "some-service" response with headers
      | Content-Type | application/json |
      | X-Baz        | abc              |
