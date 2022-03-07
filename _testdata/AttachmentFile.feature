Feature: Attachment file in steps

  Scenario: POST with body attachment from file
    When I request HTTP endpoint with method "POST" and URI "/file-attached"

    And I request HTTP endpoint with attachment as field "file" from file
    """
    _testdata/sample.txt
    """

    Then I should have response with status "OK"

    And I should have response with body
    """json
    {"content":"a b c"}
    """

  Scenario: POST with body attachment from file name
    When I request HTTP endpoint with method "POST" and URI "/file-attached"

    And I request HTTP endpoint with attachment as field "file" and file name "sample.txt"
    """
    a b c
    """

    Then I should have response with status "OK"

    And I should have response with body
    """json
    {"content":"a b c"}
    """
