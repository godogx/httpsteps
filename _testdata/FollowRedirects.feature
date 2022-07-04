Feature: FollowRedirects

  Scenario: Successful GET Request
    When I request HTTP endpoint with method "GET" and URI "/one"
    And I follow redirects from HTTP endpoint
    Then I should have response with status "OK"
    And I should have response with body
    """
    OK
    """
