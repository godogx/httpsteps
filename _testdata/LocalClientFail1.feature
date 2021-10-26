Feature: HTTP Service

  Scenario: Fail after scenario with unexpected other responses
    When I request HTTP endpoint with method "DELETE" and URI "/delete-something"

    And I concurrently request idempotent HTTP endpoint

    Then I should have response with status "204"
