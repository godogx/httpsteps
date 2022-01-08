Feature: Local Client is safe to use under concurrency.

  Scenario: Requesting service-one
    Given I should not be blocked for "service-one"

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

  Scenario: Requesting service-two
    Given I should not be blocked for "service-two"

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
