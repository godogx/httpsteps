Feature: HTTP Service

  Scenario: Successful GET Request
    When I request HTTP endpoint with method "GET" and URI "/get-something?foo=bar"
    Then I should have response with status "OK"
    And I should have response with body
    """json
    [
      {"some":"json"}
    ]
    """
    And I should have response with header "Content-Type: application/json"

  Scenario: Bad request
    When I request HTTP endpoint with method "DELETE" and URI "/bad-request"
    And I request HTTP endpoint with header "X-Foo: bar"
    And I request HTTP endpoint with cookie "c1: v1"
    And I request HTTP endpoint with cookie "c2: v2"
    Then I should have response with status "Bad Request"
    And I should have response with body from file
    """
    _testdata/sample.json
    """

  Scenario: POST with body
    When I request HTTP endpoint with method "POST" and URI "/with-body"
    And I request HTTP endpoint with body
    """
    [
      {"some":"json"}
    ]
    """
    Then I should have response with body
    """json
    {"status":"ok"}
    """
    And I should have response with status "OK"


  Scenario: Successful DELETE Request with no content
    When I request HTTP endpoint with method "DELETE" and URI "/delete-something"
    Then I should have response with status "No Content"

  Scenario: Successful DELETE Request with code 204
    When I request HTTP endpoint with method "DELETE" and URI "/delete-something"
    And I concurrently request idempotent HTTP endpoint
    Then I should have response with status "204"
    And I should have other responses with status "Not Found"
    And I should have other responses with body
    """json
    {"status":"failed"}
    """
    And I should have other responses with header "Content-Type: application/json"

  Scenario: POST with body with json5 comments
    When I request HTTP endpoint with method "POST" and URI "/with-json5-body"
    And I request HTTP endpoint with body
    """json5
    [
      // some test data
      {"some":"json5"}
    ]
    """
    Then I should have response with status "OK"
    And I should have response with body from file
    """
    _testdata/sample.json5
    """

  Scenario: GET with csv body
    When I request HTTP endpoint with method "GET" and URI "/with-csv-body"
    And I request HTTP endpoint with body from file
    """
    _testdata/sample.csv
    """
    Then I should have response with status "OK"
    And I should have response with body from file
    """
    _testdata/sample.csv
    """

  Scenario: Successful call against named service
    When I request "some-service" HTTP endpoint with method "GET" and URI "/get-something?foo=bar"
    Then I should have "some-service" response with status "OK"
    And I should have "some-service" response with body
    """json
    [
      {"some":"json"}
    ]
    """
    And I should have "some-service" response with header "Content-Type: application/json"


