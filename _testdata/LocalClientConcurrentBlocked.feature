Feature: Local Client is safe to use under concurrency (blocked).

  Scenario: Requesting service-one
    When I request "service-one" HTTP endpoint with method "GET" and URI "/get-something?service=one"
    And I concurrently request idempotent "service-one" HTTP endpoint

    When I should have "service-one" response with body
    """json
    [
      {"service":"one"}
    ]
    """
    And I should have "service-one" response with status "OK"
    And I should have "service-one" response with header "Content-Type: application/json"
    And there is only one scenario running

  Scenario: Requesting service-two
    When I request "service-two" HTTP endpoint with method "GET" and URI "/get-something?service=two"
    And I concurrently request idempotent "service-two" HTTP endpoint

    Then I should have "service-two" response with body
    """json
    [
      {"service":"two"}
    ]
    """
    And I should have "service-two" response with status "OK"
    And I should have "service-two" response with header "Content-Type: application/json"
    And there is only one scenario running

  Scenario: Requesting both services
    When I request "service-one" HTTP endpoint with method "GET" and URI "/get-something?service=one"
    And I concurrently request idempotent "service-one" HTTP endpoint

    When I should have "service-one" response with body
    """json
    [
      {"service":"one"}
    ]
    """
    And I should have "service-one" response with status "OK"
    And I should have "service-one" response with header "Content-Type: application/json"

    When I request "service-two" HTTP endpoint with method "GET" and URI "/get-something?service=two"
    And I concurrently request idempotent "service-two" HTTP endpoint

    Then I should have "service-two" response with body
    """json
    [
      {"service":"two"}
    ]
    """
    And I should have "service-two" response with status "OK"
    And I should have "service-two" response with header "Content-Type: application/json"
    And there is only one scenario running
